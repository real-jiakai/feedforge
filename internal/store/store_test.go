package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A tmp cleaner (or a recreated volume) can remove the feeds/ and seen/
// subdirectories while the server is running; writes must recreate them
// instead of failing with ENOENT until the next restart.
func TestWritesSurviveDeletedSubdirectories(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range []string{"feeds", "seen"} {
		if err := os.RemoveAll(filepath.Join(dir, sub)); err != nil {
			t.Fatal(err)
		}
	}

	f := &Feed{
		Title:       "Survivor",
		SourceURL:   "https://example.com/news",
		ItemPattern: `<li><a href="{%}">{%}</a>`,
	}
	if err := st.Create(f); err != nil {
		t.Fatalf("Create after feeds/ was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "feeds", f.ID+".json")); err != nil {
		t.Fatalf("feed file was not written: %v", err)
	}

	seen := st.MarkSeen(f.ID, []string{"guid-1"}, time.Now().UTC())
	if len(seen) != 1 {
		t.Fatalf("MarkSeen returned %d entries, want 1", len(seen))
	}
	if _, err := os.Stat(filepath.Join(dir, "seen", f.ID+".json")); err != nil {
		t.Fatalf("seen file was not written after seen/ was deleted: %v", err)
	}
}
