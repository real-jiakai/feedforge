// Package store persists feed definitions and per-feed item history as JSON
// files under a data directory. A single process owns the directory; all
// access goes through one Store guarded by a mutex.
package store

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// MaxItemsCap bounds how many items a single feed may contain.
const MaxItemsCap = 500

// DemoSourceURL is the reserved source URL of the built-in practice page.
// It is a sentinel rather than an http URL so that no external host can
// impersonate the demo by spoofing a Host header.
const DemoSourceURL = "feedforge://demo"

// Feed is one feed definition — everything needed to regenerate its output.
type Feed struct {
	ID      string `json:"id"`
	OwnerID string `json:"ownerId,omitempty"` // account that owns this feed

	// Output feed properties. May reference global captures ({%1}, …).
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`

	// Source page.
	SourceURL string `json:"sourceUrl"`
	Encoding  string `json:"encoding,omitempty"` // "" = auto-detect

	// Extraction rules.
	GlobalPattern   string `json:"globalPattern,omitempty"`
	ItemPattern     string `json:"itemPattern"`
	SmartWhitespace bool   `json:"smartWhitespace"`

	// Item templates ({%1}… refer to item captures).
	ItemTitle   string `json:"itemTitle"`
	ItemLink    string `json:"itemLink"`
	ItemContent string `json:"itemContent"`

	// Options.
	MaxItems   int  `json:"maxItems"`   // default 25, capped at MaxItemsCap
	TTLMinutes int  `json:"ttlMinutes"` // min minutes between refetches; default 30
	Reverse    bool `json:"reverse"`    // reverse item order

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Last generation status (informational).
	LastFetchAt   time.Time `json:"lastFetchAt,omitzero"`
	LastError     string    `json:"lastError,omitempty"`
	LastItemCount int       `json:"lastItemCount"`
}

// Normalize applies defaults and caps to user-supplied values.
func (f *Feed) Normalize() {
	if f.MaxItems <= 0 {
		f.MaxItems = 25
	}
	if f.MaxItems > MaxItemsCap {
		f.MaxItems = MaxItemsCap
	}
	if f.TTLMinutes <= 0 {
		f.TTLMinutes = 30
	}
	if f.TTLMinutes > 24*60 {
		f.TTLMinutes = 24 * 60
	}
	f.SourceURL = strings.TrimSpace(f.SourceURL)
	f.Encoding = strings.TrimSpace(f.Encoding)
}

// Validate reports whether the definition is complete enough to save.
func (f *Feed) Validate() error {
	if f.SourceURL == "" {
		return errors.New("sourceUrl is required")
	}
	if strings.TrimSpace(f.ItemPattern) == "" {
		return errors.New("itemPattern is required")
	}
	return nil
}

var ErrNotFound = errors.New("feed not found")

var idRe = regexp.MustCompile(`^[a-z0-9]{4,32}$`)

// Store keeps all feeds in memory, mirrored to JSON files on disk.
type Store struct {
	dir   string
	mu    sync.RWMutex
	feeds map[string]*Feed
	seen  map[string]map[string]time.Time // feedID → guid → first seen

	users        map[string]*User   // userID → user
	userIDByName map[string]string  // lowercase username → userID
	sessions     map[string]Session // token hash → session
	settings     Settings
}

const seenCap = 1000 // per feed; oldest entries pruned beyond this

// Open loads (or initializes) a data directory.
func Open(dir string) (*Store, error) {
	for _, sub := range []string{"feeds", "seen"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("creating data dir: %w", err)
		}
	}
	s := &Store{
		dir:          dir,
		feeds:        make(map[string]*Feed),
		seen:         make(map[string]map[string]time.Time),
		users:        make(map[string]*User),
		userIDByName: make(map[string]string),
		sessions:     make(map[string]Session),
	}
	if err := s.loadAccounts(); err != nil {
		return nil, fmt.Errorf("loading accounts: %w", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "feeds"))
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "feeds", e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		var f Feed
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", e.Name(), err)
		}
		if f.ID == "" || !idRe.MatchString(f.ID) {
			continue
		}
		f.Normalize()
		s.feeds[f.ID] = &f
	}
	return s, nil
}

// ListOwned returns one user's feeds, newest first.
func (s *Store) ListOwned(ownerID string) []*Feed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Feed, 0, len(s.feeds))
	for _, f := range s.feeds {
		if f.OwnerID != ownerID {
			continue
		}
		cp := *f
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}

// Get returns a copy of one feed.
func (s *Store) Get(id string) (*Feed, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.feeds[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *f
	return &cp, nil
}

// Create assigns an ID and persists a new feed.
func (s *Store) Create(f *Feed) error {
	f.Normalize()
	if err := f.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := newID(func(id string) bool { _, ok := s.feeds[id]; return ok })
	if err != nil {
		return err
	}
	f.ID = id
	now := time.Now().UTC()
	f.CreatedAt, f.UpdatedAt = now, now
	cp := *f
	if err := s.writeFeedLocked(&cp); err != nil {
		return err
	}
	s.feeds[id] = &cp
	return nil
}

// Update replaces an existing feed's definition, preserving CreatedAt.
func (s *Store) Update(f *Feed) error {
	f.Normalize()
	if err := f.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.feeds[f.ID]
	if !ok {
		return ErrNotFound
	}
	f.CreatedAt = old.CreatedAt
	f.UpdatedAt = time.Now().UTC()
	// Editing a definition must not wipe the last-generation status the UI
	// shows; only a rebuild may change it.
	f.LastFetchAt = old.LastFetchAt
	f.LastError = old.LastError
	f.LastItemCount = old.LastItemCount
	cp := *f
	if err := s.writeFeedLocked(&cp); err != nil {
		return err
	}
	s.feeds[f.ID] = &cp
	return nil
}

// SetStatus records the outcome of the latest generation attempt.
func (s *Store) SetStatus(id string, fetchErr string, itemCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.feeds[id]
	if !ok {
		return
	}
	f.LastFetchAt = time.Now().UTC()
	f.LastError = fetchErr
	f.LastItemCount = itemCount
	_ = s.writeFeedLocked(f)
}

// Delete removes a feed and its history.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.feeds[id]; !ok {
		return ErrNotFound
	}
	delete(s.feeds, id)
	delete(s.seen, id)
	if err := os.Remove(s.feedPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(s.seenPath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// MarkSeen records first-seen times for the given GUIDs and returns the
// time each was first observed — used as the item's publication date.
//
// It is a no-op for unknown feed IDs, so a build still in flight when its
// feed is deleted cannot resurrect the feed's history file.
func (s *Store) MarkSeen(id string, guids []string, now time.Time) map[string]time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]time.Time, len(guids))
	if _, live := s.feeds[id]; !live {
		for _, g := range guids {
			out[g] = now
		}
		return out
	}

	m, ok := s.seen[id]
	if !ok {
		m = s.loadSeenLocked(id)
		s.seen[id] = m
	}
	changed := false
	current := make(map[string]bool, len(guids))
	for _, g := range guids {
		current[g] = true
		if t, ok := m[g]; ok {
			out[g] = t
			continue
		}
		m[g] = now
		out[g] = now
		changed = true
	}
	if len(m) > seenCap {
		// Evict oldest-first, but never an item still present in this
		// fetch: dropping a live GUID would re-stamp its publication date
		// on the next build and re-float an old item as new.
		type kv struct {
			g string
			t time.Time
		}
		all := make([]kv, 0, len(m))
		for g, t := range m {
			if !current[g] {
				all = append(all, kv{g, t})
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].t.Before(all[j].t) })
		if excess := len(m) - seenCap; excess > 0 {
			if excess > len(all) {
				excess = len(all)
			}
			for _, e := range all[:excess] {
				delete(m, e.g)
			}
			changed = changed || excess > 0
		}
	}
	if changed {
		s.writeSeenLocked(id, m)
	}
	return out
}

func (s *Store) feedPath(id string) string { return filepath.Join(s.dir, "feeds", id+".json") }
func (s *Store) seenPath(id string) string { return filepath.Join(s.dir, "seen", id+".json") }

func (s *Store) writeFeedLocked(f *Feed) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.feedPath(f.ID), raw)
}

func (s *Store) loadSeenLocked(id string) map[string]time.Time {
	m := make(map[string]time.Time)
	raw, err := os.ReadFile(s.seenPath(id))
	if err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func (s *Store) writeSeenLocked(id string, m map[string]time.Time) {
	raw, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = atomicWrite(s.seenPath(id), raw)
}

func atomicWrite(path string, data []byte) error {
	// The parent directory can disappear behind our back — tmp cleaners
	// prune empty directories, volumes get recreated — and every write
	// after that would fail with ENOENT. Recreating it costs one cheap
	// syscall and makes the store self-healing.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// newID generates a random ID that the given predicate reports as free.
func newID(taken func(string) bool) (string, error) {
	for range 10 {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for i := range b {
			b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
		}
		id := string(b)
		if !taken(id) {
			return id, nil
		}
	}
	return "", errors.New("could not generate a unique ID")
}

// ValidID reports whether id has the shape of a store-generated ID.
func ValidID(id string) bool { return idRe.MatchString(id) }
