package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lonsty/music-tui/internal/library"
)

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "music.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := first.SetSetting("k", "v"); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	t.Cleanup(func() { _ = second.Close() })

	got, err := second.GetSetting("k")
	if err != nil {
		t.Fatal(err)
	}
	if got != "v" {
		t.Fatalf("setting after reopen = %q, want v", got)
	}
}

func TestSyncDirAddUpdateDelete(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(t.TempDir(), "music.db")
	st, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	path := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}

	added, updated, deleted, err := SyncDir(context.Background(), dir, st, "", nil)
	if err != nil {
		t.Fatalf("SyncDir: %v", err)
	}
	if added != 1 || updated != 0 || deleted != 0 {
		t.Fatalf("first sync = (%d,%d,%d), want (1,0,0)", added, updated, deleted)
	}

	// Unchanged file: nothing to do.
	added, updated, deleted, err = SyncDir(context.Background(), dir, st, "", nil)
	if err != nil {
		t.Fatalf("SyncDir: %v", err)
	}
	if added != 0 || updated != 0 || deleted != 0 {
		t.Fatalf("second sync = (%d,%d,%d), want (0,0,0)", added, updated, deleted)
	}

	// Changed mtime: re-parsed as an update.
	newMtime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, newMtime, newMtime); err != nil {
		t.Fatal(err)
	}
	added, updated, deleted, err = SyncDir(context.Background(), dir, st, "", nil)
	if err != nil {
		t.Fatalf("SyncDir: %v", err)
	}
	if added != 0 || updated != 1 || deleted != 0 {
		t.Fatalf("after touch = (%d,%d,%d), want (0,1,0)", added, updated, deleted)
	}

	// Removed file: pruned.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	added, updated, deleted, err = SyncDir(context.Background(), dir, st, "", nil)
	if err != nil {
		t.Fatalf("SyncDir: %v", err)
	}
	if added != 0 || updated != 0 || deleted != 1 {
		t.Fatalf("after delete = (%d,%d,%d), want (0,0,1)", added, updated, deleted)
	}

	tracks, err := st.AllTracks()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 0 {
		t.Fatalf("AllTracks after prune = %d tracks, want 0", len(tracks))
	}
}

func TestSyncDirProgress(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(t.TempDir(), "music.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	const n = 4
	for i := 0; i < n; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".mp3")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var calls []int
	added, _, _, err := SyncDir(context.Background(), dir, st, "", func(done, total int) {
		if total != n {
			t.Errorf("progress total = %d, want %d", total, n)
		}
		calls = append(calls, done)
	})
	if err != nil {
		t.Fatal(err)
	}
	if added != n {
		t.Fatalf("added = %d, want %d", added, n)
	}
	if len(calls) != n || calls[len(calls)-1] != n {
		t.Fatalf("progress calls = %v, want %d calls ending at %d", calls, n, n)
	}
}

func TestAllTracksOrder(t *testing.T) {
	st := openTestStore(t)
	rows := []library.Track{
		{ID: "b", Path: "/b.mp3", Title: "B", Artist: "A", Album: "Same", TrackNumber: "2", Year: "2000"},
		{ID: "a", Path: "/a.mp3", Title: "A", Artist: "A", Album: "Same", TrackNumber: "1", Year: "2000"},
		{ID: "c", Path: "/c.mp3", Title: "C", Artist: "A", Album: "Earlier", TrackNumber: "1", Year: "1999"},
	}
	for _, row := range rows {
		if err := st.UpsertTrack(row, 1); err != nil {
			t.Fatal(err)
		}
	}

	got, err := st.AllTracks()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"c", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %d tracks, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("order[%d] = %q, want %q", i, got[i].ID, id)
		}
	}
}
