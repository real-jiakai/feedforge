package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Accounts. The first user ever created becomes the administrator; everyone
// after that is a regular user and may only be created while the admin has
// registration enabled (it starts disabled). Each feed belongs to the user
// who created it.

var (
	ErrRegistrationClosed = errors.New("registration is disabled")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrUserNotFound       = errors.New("user not found")
)

// User is one account. PasswordHash is a bcrypt hash.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	IsAdmin      bool      `json:"isAdmin"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Session maps a (hashed) browser token to a user until it expires.
type Session struct {
	UserID  string    `json:"userId"`
	Expires time.Time `json:"expires"`
}

// Settings holds instance-wide options the admin can change at runtime.
type Settings struct {
	RegistrationEnabled bool `json:"registrationEnabled"`
}

// CreateUser adds an account, enforcing the registration policy atomically:
// the first user becomes the admin regardless of the policy (there is nobody
// to enable registration yet); later users require registration to be
// enabled. When the first admin is created, feeds from a pre-account data
// directory (no owner) are adopted so they stay visible and manageable.
func (s *Store) CreateUser(username, passwordHash string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToLower(username)
	if _, taken := s.userIDByName[key]; taken {
		return nil, ErrUsernameTaken
	}
	first := len(s.users) == 0
	if !first && !s.settings.RegistrationEnabled {
		return nil, ErrRegistrationClosed
	}
	id, err := newID(func(id string) bool { _, ok := s.users[id]; return ok })
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		IsAdmin:      first,
		CreatedAt:    time.Now().UTC(),
	}
	s.users[u.ID] = u
	s.userIDByName[key] = u.ID
	if err := s.writeUsersLocked(); err != nil {
		delete(s.users, u.ID)
		delete(s.userIDByName, key)
		return nil, err
	}
	if first {
		for _, f := range s.feeds {
			if f.OwnerID == "" {
				f.OwnerID = u.ID
				_ = s.writeFeedLocked(f)
			}
		}
	}
	cp := *u
	return &cp, nil
}

// UserByName looks an account up case-insensitively.
func (s *Store) UserByName(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.userIDByName[strings.ToLower(username)]
	if !ok {
		return nil, ErrUserNotFound
	}
	cp := *s.users[id]
	return &cp, nil
}

// UserByID returns a copy of one account.
func (s *Store) UserByID(id string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}

// CountUsers reports how many accounts exist (0 = fresh instance).
func (s *Store) CountUsers() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.users)
}

// RegistrationEnabled reports whether new accounts may currently be created.
func (s *Store) RegistrationEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.RegistrationEnabled
}

// SetRegistrationEnabled flips the registration policy.
func (s *Store) SetRegistrationEnabled(v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings.RegistrationEnabled = v
	return s.writeSettingsLocked()
}

// CreateSession stores a session under the SHA-256 hex of its token, so a
// leaked data directory does not hand out usable session cookies.
func (s *Store) CreateSession(tokenHash, userID string, expires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneSessionsLocked(time.Now())
	s.sessions[tokenHash] = Session{UserID: userID, Expires: expires}
	return s.writeSessionsLocked()
}

// SessionUser resolves a token hash to its account, if the session is live.
func (s *Store) SessionUser(tokenHash string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[tokenHash]
	if !ok || time.Now().After(sess.Expires) {
		return nil, false
	}
	u, ok := s.users[sess.UserID]
	if !ok {
		return nil, false
	}
	cp := *u
	return &cp, true
}

// DeleteSession logs one session out.
func (s *Store) DeleteSession(tokenHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[tokenHash]; !ok {
		return
	}
	delete(s.sessions, tokenHash)
	_ = s.writeSessionsLocked()
}

func (s *Store) pruneSessionsLocked(now time.Time) {
	for k, sess := range s.sessions {
		if now.After(sess.Expires) {
			delete(s.sessions, k)
		}
	}
}

// ---------- persistence ----------

func (s *Store) usersPath() string    { return filepath.Join(s.dir, "users.json") }
func (s *Store) sessionsPath() string { return filepath.Join(s.dir, "sessions.json") }
func (s *Store) settingsPath() string { return filepath.Join(s.dir, "settings.json") }

func (s *Store) loadAccounts() error {
	raw, err := os.ReadFile(s.usersPath())
	switch {
	case err == nil:
		var users []*User
		if err := json.Unmarshal(raw, &users); err != nil {
			return err
		}
		for _, u := range users {
			s.users[u.ID] = u
			s.userIDByName[strings.ToLower(u.Username)] = u.ID
		}
	case !os.IsNotExist(err):
		return err
	}

	if raw, err := os.ReadFile(s.settingsPath()); err == nil {
		_ = json.Unmarshal(raw, &s.settings)
	}
	if raw, err := os.ReadFile(s.sessionsPath()); err == nil {
		_ = json.Unmarshal(raw, &s.sessions)
	}
	s.pruneSessionsLocked(time.Now())
	return nil
}

func (s *Store) writeUsersLocked() error {
	users := make([]*User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	raw, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.usersPath(), raw)
}

func (s *Store) writeSettingsLocked() error {
	raw, err := json.MarshalIndent(s.settings, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.settingsPath(), raw)
}

func (s *Store) writeSessionsLocked() error {
	raw, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	return atomicWrite(s.sessionsPath(), raw)
}
