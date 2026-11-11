package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestPlaylistMoveKeys(t *testing.T) {
	a := testApp(
		library.Track{ID: "a", Title: "a"},
		library.Track{ID: "b", Title: "b"},
	)
	a.playlists = []store.Playlist{{ID: "p", Name: "P"}}
	a.plTracks = []library.Track{{ID: "a", Title: "a"}, {ID: "b", Title: "b"}}
	a.trackCursor = 0
	ctx := a.playlistTrackCtx()

	handled, cmd := ctx.OnExtraKeys(a, keyMsg("J"))
	if !handled || cmd == nil {
		t.Fatal("J did not issue a move")
	}
	if a.trackCursor != 1 {
		t.Fatalf("cursor after J = %d, want 1", a.trackCursor)
	}

	a.trackCursor = 0
	_, cmd = ctx.OnExtraKeys(a, keyMsg("K"))
	if cmd != nil {
		t.Fatal("K at the first position issued a move")
	}
}

func TestPlaylistUndoWindow(t *testing.T) {
	a := testApp(library.Track{ID: "a", Title: "a"})
	a.playlists = []store.Playlist{{ID: "p", Name: "P"}}
	a.plTracks = []library.Track{{ID: "a", Title: "a"}}
	a.trackCursor = 0
	ctx := a.playlistTrackCtx()

	_, cmd := ctx.OnExtraKeys(a, keyMsg("x"))
	if cmd == nil || a.undoRemove == nil {
		t.Fatal("x did not arm undo")
	}
	if a.undoRemove.position != 0 || a.undoRemove.trackID != "a" {
		t.Fatalf("undo state = %#v", a.undoRemove)
	}

	_, cmd = ctx.OnExtraKeys(a, keyMsg("u"))
	if cmd == nil {
		t.Fatal("u inside the window did not restore")
	}
	if a.undoRemove != nil {
		t.Fatal("undo state not cleared after restore")
	}
	if a.statusMsg == "" {
		t.Fatal("missing restore status")
	}

	a.undoRemove = &undoRemove{
		playlistID: "p",
		trackID:    "a",
		position:   0,
		title:      "a",
		expiresAt:  time.Now().Add(-time.Second),
	}
	_, cmd = ctx.OnExtraKeys(a, keyMsg("u"))
	if cmd != nil {
		t.Fatal("expired undo issued a command")
	}
	if a.statusMsg != T("undo_expired") {
		t.Fatalf("status = %q, want expired", a.statusMsg)
	}
}

func TestCreatePlaylistFromAddOverlay(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	a := testApp(library.Track{ID: "t1", Title: "one"})
	a.st = st
	a.nameInput = textinput.New()
	a.activeOvl = overlayAddToPlaylist

	a.handleAddToPlaylistKey(keyMsg("n"))
	if !a.overlayCreating || a.ovlNewTrackID != "t1" {
		t.Fatalf("create mode not armed: %#v", a.PlaylistState)
	}

	a.nameInput.SetValue("Mix")
	cmd := a.handleAddToPlaylistKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("create did not issue a command")
	}
	if a.activeOvl != overlayNone || a.overlayCreating {
		t.Fatal("overlay did not close after create")
	}
	msg := cmd().(playlistsLoadedMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}

	pls, err := st.GetPlaylists()
	if err != nil {
		t.Fatal(err)
	}
	var created *store.Playlist
	for i := range pls {
		if pls[i].Name == "Mix" {
			created = &pls[i]
		}
	}
	if created == nil || created.TrackCount != 1 {
		t.Fatalf("created playlist = %#v, want 1 track", created)
	}
}

func TestDuplicateAddReportsAndKeepsCount(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	pl, err := st.CreatePlaylist("P")
	if err != nil {
		t.Fatal(err)
	}
	a := &App{st: st}
	if msg := a.cmdAddTrackToPlaylist(pl.ID, "t1")().(playlistTracksLoadedMsg); msg.err != nil || msg.duplicate {
		t.Fatalf("first add = %#v", msg)
	}
	msg := a.cmdAddTrackToPlaylist(pl.ID, "t1")().(playlistTracksLoadedMsg)
	if !msg.duplicate {
		t.Fatal("duplicate add was not reported")
	}
	count, _ := st.PlaylistTrackCount(pl.ID)
	if count != 1 {
		t.Fatalf("count after duplicate = %d, want 1", count)
	}

	pls, _ := st.GetPlaylists()
	for _, p := range pls {
		if p.ID == pl.ID && p.TrackCount != 1 {
			t.Fatalf("cached count = %d, want 1", p.TrackCount)
		}
	}
}
