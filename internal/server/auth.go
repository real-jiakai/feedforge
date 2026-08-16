package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/real-jiakai/feedforge/internal/store"
)

// Session-cookie authentication. The cookie carries a random token whose
// SHA-256 is stored server-side, HttpOnly + SameSite=Lax; combined with the
// JSON content-type requirement on every mutating call this covers CSRF.

const (
	sessionCookie = "ff_session"
	sessionTTL    = 30 * 24 * time.Hour
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

// dummyHash is compared against when the username does not exist, so both
// login failure paths cost one bcrypt verification.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("feedforge-no-such-user"), bcrypt.DefaultCost)

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"isAdmin"`
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

// currentUser resolves the session cookie, or nil when not signed in.
func (s *Server) currentUser(r *http.Request) *store.User {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil
	}
	u, ok := s.cfg.Store.SessionUser(hashToken(c.Value))
	if !ok {
		return nil
	}
	return u
}

type userHandler func(http.ResponseWriter, *http.Request, *store.User)

func (s *Server) requireUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.currentUser(r)
		if u == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login required"})
			return
		}
		next(w, r, u)
	}
}

func (s *Server) requireAdmin(next userHandler) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request, u *store.User) {
		if !u.IsAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin only"})
			return
		}
		next(w, r, u)
	})
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID string) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := hex.EncodeToString(raw)
	if err := s.cfg.Store.CreateSession(hashToken(token), userID, time.Now().Add(sessionTTL)); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	return nil
}

// handleRegister creates an account. The very first account becomes the
// admin (and adopts any feeds from a pre-account data directory); after
// that, accounts can only be created while the admin has registration
// enabled. On a fresh instance the admin is seeded with the built-in
// recipes, so "Your feeds" starts as exactly the feeds this instance is
// meant to provide.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !readJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if !usernameRe.MatchString(req.Username) {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "username must be 3-32 letters, digits, dots, dashes or underscores"})
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "password must be 8-72 characters"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hashing password: " + err.Error()})
		return
	}
	u, err := s.cfg.Store.CreateUser(req.Username, string(hash))
	switch {
	case errors.Is(err, store.ErrUsernameTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	case errors.Is(err, store.ErrRegistrationClosed):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if u.IsAdmin && !s.cfg.Store.HasFeeds() {
		s.seedRecipes(u.ID)
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.log.Info("user registered", "username", u.Username, "admin", u.IsAdmin)
	writeJSON(w, http.StatusCreated, userInfo{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin})
}

// seedRecipes creates the built-in recipe feeds for a brand-new instance.
func (s *Server) seedRecipes(ownerID string) {
	for _, rc := range Recipes {
		f := rc.Feed
		f.OwnerID = ownerID
		if err := s.cfg.Store.Create(&f); err != nil {
			s.log.Error("seeding recipe feed", "recipe", rc.ID, "err", err)
		}
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if !readJSON(w, r, &req) {
		return
	}
	u, err := s.cfg.Store.UserByName(strings.TrimSpace(req.Username))
	if err != nil {
		// Burn a bcrypt verification anyway so an attacker cannot tell a
		// wrong username from a wrong password by timing.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid username or password"})
		return
	}
	if err := s.startSession(w, r, u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, userInfo{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !requireJSONContentType(w, r) {
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		s.cfg.Store.DeleteSession(hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not signed in"})
		return
	}
	writeJSON(w, http.StatusOK, userInfo{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin})
}

func (s *Server) handleGetAdminSettings(w http.ResponseWriter, r *http.Request, _ *store.User) {
	writeJSON(w, http.StatusOK, map[string]bool{"registrationEnabled": s.cfg.Store.RegistrationEnabled()})
}

func (s *Server) handlePutAdminSettings(w http.ResponseWriter, r *http.Request, _ *store.User) {
	var req struct {
		RegistrationEnabled bool `json:"registrationEnabled"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := s.cfg.Store.SetRegistrationEnabled(req.RegistrationEnabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.log.Info("registration toggled", "enabled", req.RegistrationEnabled)
	writeJSON(w, http.StatusOK, map[string]bool{"registrationEnabled": req.RegistrationEnabled})
}
