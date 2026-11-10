package store

import (
	"path/filepath"
	"testing"

	"github.com/lonsty/music-tui/internal/library"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestMoveAndInsertTrack(t *testing.T) {
	st := openTestStore(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := st.UpsertTrack(library.Track{ID: id, Path: "/music/" + id + ".mp3", Title: id}, 1); err != nil {
			t.Fatal(err)
		}
	}
	pl, err := st.CreatePlaylist("Ordered")
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"a", "b", "c"} {
		if err := st.AddTrackToPlaylist(pl.ID, id, i); err != nil {
			t.Fatal(err)
		}
	}

	order := func() string {
		tracks, err := st.GetPlaylistTracks(pl.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := ""
		for _, tr := range tracks {
			out += tr.ID
		}
		return out
	}

	if err := st.MoveTrack(pl.ID, "a", 1); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bac" {
		t.Fatalf("after move down = %s, want bac", got)
	}
	if err := st.MoveTrack(pl.ID, "c", -1); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bca" {
		t.Fatalf("after move up = %s, want bca", got)
	}
	if err := st.MoveTrack(pl.ID, "b", -1); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bca" {
		t.Fatalf("boundary move changed order: %s", got)
	}
	if err := st.MoveTrack(pl.ID, "a", 1); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bca" {
		t.Fatalf("boundary move down changed order: %s", got)
	}

	if err := st.RemoveTrackFromPlaylist(pl.ID, "b"); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertTrackAt(pl.ID, "b", 0); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bca" {
		t.Fatalf("after undo insert = %s, want bca", got)
	}
	if err := st.InsertTrackAt(pl.ID, "b", 0); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "bca" {
		t.Fatalf("duplicate insert changed order: %s", got)
	}
}

func TestGetPlaylistsIncludesTrackCounts(t *testing.T) {
	st := openTestStore(t)
	if err := st.EnsureFavorites(); err != nil {
		t.Fatal(err)
	}
	empty, err := st.CreatePlaylist("Empty")
	if err != nil {
		t.Fatal(err)
	}
	full, err := st.CreatePlaylist("Full")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddTrackToPlaylist(full.ID, "track-a", 0); err != nil {
		t.Fatal(err)
	}
	if err := st.AddTrackToPlaylist(full.ID, "track-b", 1); err != nil {
		t.Fatal(err)
	}

	pls, err := st.GetPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	counts := make(map[string]int, len(pls))
	for _, pl := range pls {
		counts[pl.ID] = pl.TrackCount
	}
	if counts[FavoritesPlaylistID] != 0 {
		t.Errorf("favorites count = %d, want 0", counts[FavoritesPlaylistID])
	}
	if counts[empty.ID] != 0 {
		t.Errorf("empty count = %d, want 0", counts[empty.ID])
	}
	if counts[full.ID] != 2 {
		t.Errorf("full count = %d, want 2", counts[full.ID])
	}

	if err := st.RemoveTrackFromPlaylist(full.ID, "track-a"); err != nil {
		t.Fatal(err)
	}
	pls, err = st.GetPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	for _, pl := range pls {
		if pl.ID == full.ID && pl.TrackCount != 1 {
			t.Errorf("full count after remove = %d, want 1", pl.TrackCount)
		}
	}
}
