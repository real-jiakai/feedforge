// Package server wires the HTTP API, feed generation pipeline, and web UI.
package server

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/real-jiakai/feedforge/internal/feed"
	"github.com/real-jiakai/feedforge/internal/fetch"
	"github.com/real-jiakai/feedforge/internal/pattern"
	"github.com/real-jiakai/feedforge/internal/store"
)

// Config configures a Server.
type Config struct {
	Store   *store.Store
	Fetcher *fetch.Fetcher
	BaseURL string // public origin, e.g. https://feeds.example.com; empty = derive from request
	WebFS   fs.FS  // embedded UI files
	Logger  *slog.Logger
}

// Server handles all HTTP traffic.
type Server struct {
	cfg Config
	log *slog.Logger

	cacheMu sync.Mutex
	cache   map[string]*cacheEntry

	pvMu    sync.Mutex
	pvPages map[string]*previewPage
}

type cacheEntry struct {
	mu         sync.Mutex
	lastGood   *builtFeed
	lastGoodAt time.Time
	lastErr    error
	lastTryAt  time.Time
}

type builtFeed struct {
	meta  feed.Meta
	items []feed.Item
}

type previewPage struct {
	content  string
	finalURL string
	at       time.Time
}

const (
	previewPageTTL    = 90 * time.Second
	previewCacheMax   = 32
	errRetryInterval  = time.Minute
	maxAPIBody        = 1 << 20 // 1 MB request bodies
	pageExcerptMax    = 250_000
	previewItemsShown = 50
	buildTimeout      = 60 * time.Second
)

// New builds the Server and its routes.
func New(cfg Config) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Server{
		cfg:     cfg,
		log:     cfg.Logger,
		cache:   make(map[string]*cacheEntry),
		pvPages: make(map[string]*previewPage),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/recipes", s.handleRecipes)

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth/me", s.handleMe)
	mux.HandleFunc("GET /api/admin/settings", s.requireAdmin(s.handleGetAdminSettings))
	mux.HandleFunc("PUT /api/admin/settings", s.requireAdmin(s.handlePutAdminSettings))

	mux.HandleFunc("GET /api/feeds", s.requireUser(s.handleListFeeds))
	mux.HandleFunc("GET /api/feeds/{id}", s.requireUser(s.handleGetFeed))
	mux.HandleFunc("POST /api/feeds", s.requireUser(s.handleCreateFeed))
	mux.HandleFunc("PUT /api/feeds/{id}", s.requireUser(s.handleUpdateFeed))
	mux.HandleFunc("DELETE /api/feeds/{id}", s.requireUser(s.handleDeleteFeed))
	mux.HandleFunc("POST /api/feeds/{id}/refresh", s.requireUser(s.handleRefreshFeed))
	mux.HandleFunc("POST /api/preview", s.requireUser(s.handlePreview))

	mux.HandleFunc("GET /feeds/{file}", s.handleFeedOutput)

	mux.HandleFunc("GET /demo", s.handleDemo)
	mux.HandleFunc("GET /demo/article/{n}", s.handleDemoArticle)

	mux.Handle("GET /", http.FileServerFS(cfg.WebFS))

	return s.withRecovery(s.withLogging(mux))
}

// ---------- middleware ----------

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("http", "method", r.Method, "path", r.URL.Path, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "path", r.URL.Path, "err", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---------- small helpers ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// requireJSONContentType enforces a JSON content type on mutating calls.
// The check is what keeps a cross-origin HTML form (which can only send
// text/plain, multipart or urlencoded, and needs no preflight) from driving
// the mutating endpoints of a token-less deployment.
func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ctype := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]))
	if ctype != "application/json" {
		writeJSON(w, http.StatusUnsupportedMediaType,
			map[string]string{"error": `Content-Type must be application/json`})
		return false
	}
	return true
}

// readJSON decodes a request body, requiring a JSON content type.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	if !requireJSONContentType(w, r) {
		return false
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxAPIBody))
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return false
	}
	return true
}

// smartWhitespaceDefault re-reads the raw body to tell "field omitted" from
// "explicitly false", so API clients that never heard of the option get the
// same forgiving matching the wizard uses.
func applySmartWhitespaceDefault(raw []byte, f *store.Feed) {
	var probe struct {
		SmartWhitespace *bool `json:"smartWhitespace"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.SmartWhitespace == nil {
		f.SmartWhitespace = true
	}
}

// readJSONRaw decodes like readJSON but also returns the raw body bytes.
func readJSONRaw(w http.ResponseWriter, r *http.Request, v any) ([]byte, bool) {
	defer r.Body.Close()
	if !requireJSONContentType(w, r) {
		return nil, false
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxAPIBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reading body: " + err.Error()})
		return nil, false
	}
	if err := json.Unmarshal(raw, v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return nil, false
	}
	return raw, true
}

func cutString(s string, max int) (string, bool) {
	if len(s) <= max {
		return s, false
	}
	cut := s[:max]
	// If the cut point split a multi-byte rune, drop the partial rune so
	// the excerpt stays valid UTF-8. DecodeLastRuneInString reports
	// (RuneError, 1) for each trailing byte of an incomplete sequence; a
	// complete final rune (of any width) is left alone.
	for len(cut) > 0 {
		r, size := utf8.DecodeLastRuneInString(cut)
		if r != utf8.RuneError || size > 1 {
			break
		}
		cut = cut[:len(cut)-1]
	}
	return cut, true
}

// baseURL returns the public origin to use in generated feed URLs. A
// configured BaseURL always wins: deriving it from the request means the
// Host header — which any client sets freely — would otherwise end up baked
// into cached feed documents served to everyone else.
func (s *Server) baseURL(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return strings.TrimRight(s.cfg.BaseURL, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

// ---------- config / feeds CRUD ----------

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"needsSetup":          s.cfg.Store.CountUsers() == 0,
		"registrationEnabled": s.cfg.Store.RegistrationEnabled(),
		"version":             "1.1.0",
	})
}

// ownedFeed loads a feed if it belongs to the user; anything else is a 404
// so feed IDs cannot be probed across accounts.
func (s *Server) ownedFeed(w http.ResponseWriter, id string, u *store.User) (*store.Feed, bool) {
	f, err := s.cfg.Store.Get(id)
	if err != nil || f.OwnerID != u.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
		return nil, false
	}
	return f, true
}

func (s *Server) handleListFeeds(w http.ResponseWriter, r *http.Request, u *store.User) {
	writeJSON(w, http.StatusOK, s.cfg.Store.ListOwned(u.ID))
}

func (s *Server) handleGetFeed(w http.ResponseWriter, r *http.Request, u *store.User) {
	f, ok := s.ownedFeed(w, r.PathValue("id"), u)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleCreateFeed(w http.ResponseWriter, r *http.Request, u *store.User) {
	var f store.Feed
	raw, ok := readJSONRaw(w, r, &f)
	if !ok {
		return
	}
	applySmartWhitespaceDefault(raw, &f)
	f.OwnerID = u.ID
	if err := validatePatterns(&f); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.cfg.Store.Create(&f); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (s *Server) handleUpdateFeed(w http.ResponseWriter, r *http.Request, u *store.User) {
	if _, ok := s.ownedFeed(w, r.PathValue("id"), u); !ok {
		return
	}
	var f store.Feed
	raw, ok := readJSONRaw(w, r, &f)
	if !ok {
		return
	}
	applySmartWhitespaceDefault(raw, &f)
	f.ID = r.PathValue("id")
	f.OwnerID = u.ID // ownership is never taken over from the request body
	if err := validatePatterns(&f); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := s.cfg.Store.Update(&f); err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]string{"error": err.Error()})
		return
	}
	s.invalidate(f.ID)
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request, u *store.User) {
	id := r.PathValue("id")
	if _, ok := s.ownedFeed(w, id, u); !ok {
		return
	}
	if err := s.cfg.Store.Delete(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
		return
	}
	s.invalidate(id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleRefreshFeed(w http.ResponseWriter, r *http.Request, u *store.User) {
	// Refresh takes no body, but the content-type check keeps it from
	// being the one mutating endpoint a cross-origin HTML form could
	// still drive, forcing source refetches past the TTL.
	if !requireJSONContentType(w, r) {
		return
	}
	id := r.PathValue("id")
	if _, ok := s.ownedFeed(w, id, u); !ok {
		return
	}
	b, err := s.materialize(r.Context(), id, true)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "itemCount": len(b.items)})
}

func validatePatterns(f *store.Feed) error {
	if strings.TrimSpace(f.GlobalPattern) != "" {
		g, err := pattern.Compile(f.GlobalPattern, f.SmartWhitespace)
		if err != nil {
			return fmt.Errorf("global pattern: %w", err)
		}
		if g.NumCaptures() == 0 {
			return errors.New("global pattern needs at least one {%} macro")
		}
	}
	if strings.TrimSpace(f.ItemPattern) != "" {
		if _, err := pattern.Compile(f.ItemPattern, f.SmartWhitespace); err != nil {
			return fmt.Errorf("item pattern: %w", err)
		}
	}
	return nil
}

// ---------- feed generation ----------

func (s *Server) entry(id string) *cacheEntry {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	e, ok := s.cache[id]
	if !ok {
		e = &cacheEntry{}
		s.cache[id] = e
	}
	return e
}

func (s *Server) invalidate(id string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	delete(s.cache, id)
}

// materialize returns the current output items for a feed, fetching the
// source page at most once per TTL (per-feed lock = natural singleflight).
//
// The feed definition is re-read inside the lock so that an edit landing
// between a caller's lookup and its turn at the lock cannot cause output
// built from the superseded definition to be cached for a full TTL.
func (s *Server) materialize(ctx context.Context, feedID string, force bool) (*builtFeed, error) {
	e := s.entry(feedID)
	e.mu.Lock()
	defer e.mu.Unlock()

	f, err := s.cfg.Store.Get(feedID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	ttl := time.Duration(f.TTLMinutes) * time.Minute
	if !force {
		if e.lastGood != nil && now.Sub(e.lastGoodAt) < ttl {
			return e.lastGood, nil
		}
		if e.lastErr != nil && now.Sub(e.lastTryAt) < errRetryInterval {
			if e.lastGood != nil {
				return e.lastGood, nil // serve stale during source outages
			}
			return nil, e.lastErr
		}
	}

	// Detach from the caller's context: one subscriber hanging up must not
	// abort a build other subscribers are waiting on, nor poison the error
	// cache with "context canceled" for a perfectly healthy source.
	buildCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), buildTimeout)
	defer cancel()

	e.lastTryAt = now
	b, err := s.build(buildCtx, f)
	if err != nil {
		e.lastErr = err
		s.cfg.Store.SetStatus(f.ID, err.Error(), 0)
		// A forced refresh exists to report whether a live rebuild works;
		// masking the failure with a stale copy would answer the wrong
		// question.
		if e.lastGood != nil && !force {
			return e.lastGood, nil
		}
		return nil, err
	}
	e.lastErr = nil
	e.lastGood = b
	e.lastGoodAt = time.Now()
	s.cfg.Store.SetStatus(f.ID, "", len(b.items))
	return b, nil
}

// sourceContent fetches a source page, serving the built-in practice page
// in-process for the reserved demo URL (the SSRF guard would otherwise block
// localhost deployments from reaching it).
func (s *Server) sourceContent(ctx context.Context, rawURL, encoding string) (*fetch.Result, error) {
	if strings.TrimSpace(rawURL) == store.DemoSourceURL {
		return &fetch.Result{
			Content:  demoHTML(time.Now()),
			FinalURL: store.DemoSourceURL,
			Status:   http.StatusOK,
		}, nil
	}
	return s.cfg.Fetcher.Get(ctx, rawURL, encoding)
}

func (s *Server) build(ctx context.Context, f *store.Feed) (*builtFeed, error) {
	res, err := s.sourceContent(ctx, f.SourceURL, f.Encoding)
	if err != nil {
		return nil, err
	}
	page := pattern.NormalizePage(res.Content)

	region := page
	var globalCaps []string
	if strings.TrimSpace(f.GlobalPattern) != "" {
		g, err := pattern.Compile(f.GlobalPattern, f.SmartWhitespace)
		if err != nil {
			return nil, fmt.Errorf("global pattern: %w", err)
		}
		globalCaps = g.FindFirst(page)
		if globalCaps == nil {
			return nil, errors.New("global pattern did not match the page")
		}
		region = strings.Join(globalCaps, "")
	}

	it, err := pattern.Compile(f.ItemPattern, f.SmartWhitespace)
	if err != nil {
		return nil, fmt.Errorf("item pattern: %w", err)
	}
	matches := selectMatches(it, region, f.MaxItems, f.Reverse)

	items := make([]feed.Item, 0, len(matches))
	guids := make([]string, 0, len(matches))
	for _, caps := range matches {
		item := renderItem(f, caps, res.FinalURL)
		items = append(items, item)
		guids = append(guids, item.GUID)
	}
	seen := s.cfg.Store.MarkSeen(f.ID, guids, time.Now().UTC())
	for i := range items {
		items[i].PubDate = seen[items[i].GUID]
	}

	meta := renderMeta(f, globalCaps, res.FinalURL)
	meta.LastBuild = time.Now().UTC()
	// SelfURL is deliberately left empty here: it depends on the requesting
	// origin and is filled in per request, so a cached document can never
	// carry another client's Host header.
	return &builtFeed{meta: meta, items: items}, nil
}

// selectMatches applies the item pattern and returns at most maxItems
// matches in output order. For reverse feeds the newest entries are at the
// bottom of the page, so the LAST maxItems matches are kept — taking the
// first ones would freeze the feed as soon as the page outgrows the limit.
func selectMatches(it *pattern.Pattern, region string, maxItems int, reverse bool) [][]string {
	if !reverse {
		return it.FindAll(region, maxItems)
	}
	matches := it.FindLast(region, maxItems)
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}
	return matches
}

// renderItem turns one pattern match into an output item.
func renderItem(f *store.Feed, caps []string, pageURL string) feed.Item {
	title := cleanTitle(pattern.Render(f.ItemTitle, caps))
	link := resolveLink(pattern.Render(f.ItemLink, caps), pageURL)
	content := pattern.Render(f.ItemContent, caps)
	return feed.Item{
		Title:   title,
		Link:    link,
		Content: content,
		GUID:    makeGUID(link, title, caps),
	}
}

func renderMeta(f *store.Feed, globalCaps []string, pageURL string) feed.Meta {
	title := cleanTitle(pattern.Render(f.Title, globalCaps))
	link := resolveLink(pattern.Render(f.Link, globalCaps), pageURL)
	desc := strings.TrimSpace(pattern.Render(f.Description, globalCaps))
	if title == "" {
		if u, err := url.Parse(f.SourceURL); err == nil && u.Host != "" {
			title = u.Host
		} else {
			title = "FeedForge feed"
		}
	}
	if link == "" {
		link = f.SourceURL
	}
	if desc == "" {
		desc = "Generated by FeedForge from " + f.SourceURL
	}
	return feed.Meta{Title: title, Link: link, Description: desc}
}

var (
	tagRe = regexp.MustCompile(`<[^>]*>`)
	wsRe  = regexp.MustCompile(`\s+`)
)

// cleanTitle makes captured HTML usable as a plain-text title: tags are
// stripped, entities decoded, whitespace collapsed.
func cleanTitle(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(wsRe.ReplaceAllString(s, " "))
}

// resolveLink trims a rendered link and resolves it against the source page
// URL so relative hrefs become absolute.
//
// Only http(s) survives. Scraped pages are untrusted input, and a
// javascript: or data: href copied into a feed's <link> would be handed
// straight to every subscriber's reader.
func resolveLink(link, pageURL string) string {
	link = html.UnescapeString(strings.TrimSpace(link))
	if link == "" {
		return ""
	}
	ref, err := url.Parse(link)
	if err != nil {
		return ""
	}
	base, err := url.Parse(pageURL)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	switch strings.ToLower(resolved.Scheme) {
	case "http", "https":
		return resolved.String()
	default:
		return ""
	}
}

func makeGUID(link, title string, caps []string) string {
	basis := link + "\x00" + title
	if strings.Trim(basis, "\x00") == "" {
		basis = strings.Join(caps, "\x1f")
	}
	sum := sha1.Sum([]byte(basis))
	return "feedforge-" + hex.EncodeToString(sum[:10])
}

// ---------- feed output ----------

func (s *Server) handleFeedOutput(w http.ResponseWriter, r *http.Request) {
	file := r.PathValue("file")
	var id, format string
	switch {
	case strings.HasSuffix(file, ".xml"):
		id, format = strings.TrimSuffix(file, ".xml"), "rss"
	case strings.HasSuffix(file, ".rss"):
		id, format = strings.TrimSuffix(file, ".rss"), "rss"
	case strings.HasSuffix(file, ".json"):
		id, format = strings.TrimSuffix(file, ".json"), "json"
	default:
		http.NotFound(w, r)
		return
	}
	if !store.ValidID(id) {
		http.NotFound(w, r)
		return
	}
	if _, err := s.cfg.Store.Get(id); err != nil {
		http.NotFound(w, r)
		return
	}
	b, err := s.materialize(r.Context(), id, false)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "feed generation failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	etag := fmt.Sprintf(`W/"%x-%d-%s"`, b.meta.LastBuild.Unix(), len(b.items), format)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=300")

	// Fill the self-link from the current request rather than from the
	// cached document, so it always reflects this client's origin.
	meta := b.meta
	if format == "json" {
		meta.SelfURL = s.baseURL(r) + "/feeds/" + id + ".json"
		out, err := feed.JSONFeed(meta, b.items)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/feed+json; charset=utf-8")
		_, _ = w.Write(out)
		return
	}
	meta.SelfURL = s.baseURL(r) + "/feeds/" + id + ".xml"
	out, err := feed.RSS(meta, b.items)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write(out)
}

// ---------- preview (powers the wizard) ----------

type previewRequest struct {
	SourceURL       string `json:"sourceUrl"`
	Encoding        string `json:"encoding"`
	GlobalPattern   string `json:"globalPattern"`
	ItemPattern     string `json:"itemPattern"`
	ItemTitle       string `json:"itemTitle"`
	ItemLink        string `json:"itemLink"`
	ItemContent     string `json:"itemContent"`
	Title           string `json:"title"`
	Link            string `json:"link"`
	Description     string `json:"description"`
	SmartWhitespace bool   `json:"smartWhitespace"`
	MaxItems        int    `json:"maxItems"`
	Reverse         bool   `json:"reverse"`
	IncludePage     bool   `json:"includePage"`
	ForceRefetch    bool   `json:"forceRefetch"`
}

type previewItem struct {
	Captures []string `json:"captures"`
	Title    string   `json:"title"`
	Link     string   `json:"link"`
	Content  string   `json:"content"`
	GUID     string   `json:"guid"`
}

type previewResponse struct {
	FetchError    string        `json:"fetchError,omitempty"`
	FinalURL      string        `json:"finalUrl,omitempty"`
	PageLength    int           `json:"pageLength,omitempty"`
	PageExcerpt   string        `json:"pageExcerpt,omitempty"`
	PageTruncated bool          `json:"pageTruncated,omitempty"`
	GlobalError   string        `json:"globalError,omitempty"`
	GlobalCaps    []string      `json:"globalCaptures,omitempty"`
	ItemError     string        `json:"itemError,omitempty"`
	CaptureCount  int           `json:"captureCount"`
	TotalMatches  int           `json:"totalMatches"`
	MatchesOnPage int           `json:"matchesOnPage"`
	Items         []previewItem `json:"items"`
	Meta          *feed.Meta    `json:"meta,omitempty"`
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request, _ *store.User) {
	var req previewRequest
	raw, ok := readJSONRaw(w, r, &req)
	if !ok {
		return
	}
	var probe struct {
		SmartWhitespace *bool `json:"smartWhitespace"`
	}
	if json.Unmarshal(raw, &probe) == nil && probe.SmartWhitespace == nil {
		req.SmartWhitespace = true
	}
	resp := previewResponse{Items: []previewItem{}}

	page, finalURL, err := s.previewFetch(r.Context(), req.SourceURL, req.Encoding, req.ForceRefetch)
	if err != nil {
		resp.FetchError = err.Error()
		writeJSON(w, http.StatusOK, resp)
		return
	}
	resp.FinalURL = finalURL
	resp.PageLength = len(page)
	if req.IncludePage {
		resp.PageExcerpt, resp.PageTruncated = cutString(page, pageExcerptMax)
	}

	region := page
	if strings.TrimSpace(req.GlobalPattern) != "" {
		g, err := pattern.Compile(req.GlobalPattern, req.SmartWhitespace)
		switch {
		case err != nil:
			resp.GlobalError = err.Error()
		case g.NumCaptures() == 0:
			resp.GlobalError = "global pattern needs at least one {%} macro"
		default:
			caps := g.FindFirst(page)
			if caps == nil {
				resp.GlobalError = "global pattern did not match the page"
			} else {
				resp.GlobalCaps = truncateAll(caps, 2000)
				region = strings.Join(caps, "")
			}
		}
	}

	if strings.TrimSpace(req.ItemPattern) != "" && resp.GlobalError == "" {
		it, err := pattern.Compile(req.ItemPattern, req.SmartWhitespace)
		if err != nil {
			resp.ItemError = err.Error()
		} else {
			resp.CaptureCount = it.NumCaptures()
			maxItems := req.MaxItems
			if maxItems <= 0 || maxItems > store.MaxItemsCap {
				maxItems = 25
			}
			matches := selectMatches(it, region, maxItems, req.Reverse)
			resp.TotalMatches = len(matches)
			// Report the true match count so the wizard can warn when the
			// page holds more items than the feed will carry.
			resp.MatchesOnPage = len(it.FindAll(region, 0))
			pf := &store.Feed{
				ItemTitle:   req.ItemTitle,
				ItemLink:    req.ItemLink,
				ItemContent: req.ItemContent,
			}
			shown := matches
			if len(shown) > previewItemsShown {
				shown = shown[:previewItemsShown]
			}
			for _, caps := range shown {
				item := renderItem(pf, caps, finalURL)
				resp.Items = append(resp.Items, previewItem{
					Captures: truncateAll(caps, 2000),
					Title:    item.Title,
					Link:     item.Link,
					Content:  item.Content,
					GUID:     item.GUID,
				})
			}
			mf := &store.Feed{
				Title:       req.Title,
				Link:        req.Link,
				Description: req.Description,
				SourceURL:   req.SourceURL,
			}
			m := renderMeta(mf, resp.GlobalCaps, finalURL)
			resp.Meta = &m
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func truncateAll(in []string, max int) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i], _ = cutString(s, max)
	}
	return out
}

// previewFetch caches fetched pages briefly so that live pattern editing
// doesn't hammer the source site on every keystroke.
func (s *Server) previewFetch(ctx context.Context, rawURL, encoding string, force bool) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", errors.New("source URL is required")
	}
	key := rawURL + "\x00" + encoding
	now := time.Now()

	s.pvMu.Lock()
	if !force {
		if p, ok := s.pvPages[key]; ok && now.Sub(p.at) < previewPageTTL {
			s.pvMu.Unlock()
			return p.content, p.finalURL, nil
		}
	}
	s.pvMu.Unlock()

	res, err := s.sourceContent(ctx, rawURL, encoding)
	if err != nil {
		return "", "", err
	}
	content := pattern.NormalizePage(res.Content)

	s.pvMu.Lock()
	if len(s.pvPages) >= previewCacheMax {
		var oldestKey string
		var oldest time.Time
		for k, p := range s.pvPages {
			if oldestKey == "" || p.at.Before(oldest) {
				oldestKey, oldest = k, p.at
			}
		}
		delete(s.pvPages, oldestKey)
	}
	s.pvPages[key] = &previewPage{content: content, finalURL: res.FinalURL, at: now}
	s.pvMu.Unlock()

	return content, res.FinalURL, nil
}
