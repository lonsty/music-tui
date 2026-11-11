package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestSyncDirCancelledBeforeStart(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(t.TempDir(), "music.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, string(rune('a'+i))+".mp3")
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	var added int
	go func() {
		added, _, _, _ = SyncDir(ctx, dir, st, "", nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled SyncDir did not return within 5s")
	}
	if added != 0 {
		t.Fatalf("cancelled SyncDir added %d tracks, want 0", added)
	}
}

func TestSyncDirWriteFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := Open(filepath.Join(t.TempDir(), "music.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	added, updated, deleted, syncErr := SyncDir(context.Background(), dir, st, "", nil)
	if syncErr == nil {
		t.Fatal("expected an error from a closed database")
	}
	if added != 0 || updated != 0 || deleted != 0 {
		t.Fatalf("counts = (%d,%d,%d), want all zero", added, updated, deleted)
	}
}

func TestMigrationUpgradeFromV1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "music.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Roll the database back to the v1 shape: drop v2 objects and its marker.
	if _, err := st.db.Exec(`
		DROP INDEX IF EXISTS idx_tracks_order;
		DROP INDEX IF EXISTS idx_playlist_tracks_pos;
		DELETE FROM schema_migrations WHERE version = 2;
	`); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := Open(path)
	if err != nil {
		t.Fatalf("upgrade open: %v", err)
	}
	t.Cleanup(func() { _ = upgraded.Close() })

	for _, idx := range []string{"idx_tracks_order", "idx_playlist_tracks_pos"} {
		var n int
		if err := upgraded.db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, idx,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("index %q missing after upgrade", idx)
		}
	}
}

func queryPlan(t *testing.T, st *Store, query string) string {
	t.Helper()
	rows, err := st.db.Query("EXPLAIN QUERY PLAN " + query)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	var out string
	for rows.Next() {
		var (
			id, parent, notUsed int
			detail              string
		)
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatal(err)
		}
		out += detail + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAllTracksQueryUsesOrderIndex(t *testing.T) {
	st := openTestStore(t)
	plan := queryPlan(t, st, allTracksQuery)
	if !strings.Contains(plan, "idx_tracks_order") {
		t.Fatalf("plan does not use idx_tracks_order:\n%s", plan)
	}
	if strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("plan still sorts in a temporary B-tree:\n%s", plan)
	}
}

func TestPlaylistTracksQueryUsesPositionIndex(t *testing.T) {
	st := openTestStore(t)
	plan := queryPlan(t, st, `
		SELECT track_id FROM playlist_tracks
		WHERE playlist_id = 'x'
		ORDER BY position
	`)
	if !strings.Contains(plan, "idx_playlist_tracks_pos") {
		t.Fatalf("plan does not use idx_playlist_tracks_pos:\n%s", plan)
	}
	if strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("plan still sorts in a temporary B-tree:\n%s", plan)
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
