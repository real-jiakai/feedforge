package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/real-jiakai/feedforge/internal/fetch"
	"github.com/real-jiakai/feedforge/internal/store"
)

// Recipes are checked against saved copies of the real pages. The fixtures
// keep each site's structural quirks — notably a lead item whose markup
// differs from the rest of the list — because those are exactly what a
// naively written pattern gets wrong.

func recipeByID(t *testing.T, id string) Recipe {
	t.Helper()
	for _, r := range Recipes {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("recipe %q not found", id)
	return Recipe{}
}

// serveFixture serves a testdata file as the source page and returns a
// FeedForge instance pointed at it.
func serveFixture(t *testing.T, name string) (*httptest.Server, *httptest.Server) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(source.Close)

	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := httptest.NewServer(New(Config{
		Store:   st,
		Fetcher: fetch.New(true, 5*1024*1024, 10*time.Second),
		WebFS:   fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ui")}},
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	t.Cleanup(app.Close)
	return app, source
}

// runRecipe applies a recipe's patterns to a fixture and returns the items.
func runRecipe(t *testing.T, r Recipe, fixture string) []previewItem {
	t.Helper()
	app, source := serveFixture(t, fixture)
	c := adminClient(t, app) // preview requires a signed-in user

	body := map[string]any{
		"sourceUrl":       source.URL,
		"globalPattern":   r.Feed.GlobalPattern,
		"itemPattern":     r.Feed.ItemPattern,
		"itemTitle":       r.Feed.ItemTitle,
		"itemLink":        r.Feed.ItemLink,
		"itemContent":     r.Feed.ItemContent,
		"smartWhitespace": r.Feed.SmartWhitespace,
		"maxItems":        r.Feed.MaxItems,
		"reverse":         r.Feed.Reverse,
	}
	resp := postJSON(t, c, app.URL+"/api/preview", body)
	defer resp.Body.Close()

	var pv previewResponse
	if err := json.NewDecoder(resp.Body).Decode(&pv); err != nil {
		t.Fatal(err)
	}
	if pv.FetchError != "" || pv.GlobalError != "" || pv.ItemError != "" {
		t.Fatalf("recipe %q errored: fetch=%q global=%q item=%q",
			r.ID, pv.FetchError, pv.GlobalError, pv.ItemError)
	}
	return pv.Items
}

func TestRecipeOSSInsight(t *testing.T) {
	items := runRecipe(t, recipeByID(t, "ossinsight"), "ossinsight_blog.html")
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(items), items)
	}
	// The first post is the featured one, whose <h2> class differs from the
	// grid posts' — a pattern that hard-codes the class would miss it.
	first := items[0]
	if first.Title != "The Agent Memory Race of 2026: 5 Repos, 4 Architectures, 1 Unsolved Problem" {
		t.Errorf("featured title = %q", first.Title)
	}
	if !strings.HasSuffix(first.Link, "/blog/agent-memory-race-2026") {
		t.Errorf("featured link = %q", first.Link)
	}
	if items[1].Title == "" || items[2].Title == "" {
		t.Errorf("grid posts lost their titles: %+v", items[1:])
	}
	for i, it := range items {
		if len(it.Captures) != 3 {
			t.Errorf("item %d: got %d captures, want 3", i, len(it.Captures))
		}
		if !strings.HasPrefix(it.Link, "http") {
			t.Errorf("item %d: link not absolute: %q", i, it.Link)
		}
	}
}

func TestRecipeBytesDev(t *testing.T) {
	items := runRecipe(t, recipeByID(t, "bytes"), "bytes_archives.html")
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4 (1 featured + 3 list): %+v", len(items), items)
	}
	// Issue 507 is rendered with font-bold and a <p> title; the list items
	// use font-semibold and a <div>. Both must land in the feed, newest
	// first — losing the featured one would silently drop every new issue
	// until it scrolled into the list.
	if got, want := items[0].Title, "Issue 507: Something worse than semver"; got != want {
		t.Errorf("featured title = %q, want %q", got, want)
	}
	if got, want := items[1].Title, "Issue 506: One way ticket to TypeScript hell... and back"; got != want {
		t.Errorf("second title = %q, want %q", got, want)
	}
	if !strings.HasSuffix(items[0].Link, "/archives/507") {
		t.Errorf("featured link = %q", items[0].Link)
	}
	for i, it := range items {
		if len(it.Captures) != 4 {
			t.Errorf("item %d: got %d captures, want 4", i, len(it.Captures))
		}
		if it.Captures[1] == "" {
			t.Errorf("item %d: missing date capture", i)
		}
	}
}

func TestRecipesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Recipes {
		if seen[r.ID] {
			t.Errorf("duplicate recipe ID %q", r.ID)
		}
		seen[r.ID] = true
		if r.Name == "" || r.Note == "" || r.NoteZH == "" {
			t.Errorf("recipe %q is missing display text", r.ID)
		}
		f := r.Feed
		f.Normalize()
		if err := f.Validate(); err != nil {
			t.Errorf("recipe %q is not a valid feed: %v", r.ID, err)
		}
		if err := validatePatterns(&f); err != nil {
			t.Errorf("recipe %q has invalid patterns: %v", r.ID, err)
		}
	}
}

func TestRecipesEndpoint(t *testing.T) {
	app, _, _ := newTestServer(t)
	resp, err := http.Get(app.URL + "/api/recipes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got []Recipe
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(Recipes) {
		t.Errorf("got %d recipes, want %d", len(got), len(Recipes))
	}
}

func TestOnlyTheTwoIntendedRecipesShip(t *testing.T) {
	// This instance deliberately offers just Bytes.dev and OSSInsight —
	// other sites are already covered by other open-source projects.
	want := map[string]bool{"ossinsight": true, "bytes": true}
	if len(Recipes) != len(want) {
		t.Fatalf("got %d recipes, want %d", len(Recipes), len(want))
	}
	for _, r := range Recipes {
		if !want[r.ID] {
			t.Errorf("unexpected recipe %q", r.ID)
		}
	}
}
