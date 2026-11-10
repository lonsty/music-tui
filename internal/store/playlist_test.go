package store

import (
	"path/filepath"
	"testing"
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
