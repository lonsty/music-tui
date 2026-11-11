package tui

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

func testApp(tracks ...library.Track) *App {
	a := &App{
		searchInput: textinput.New(),
		mqRow:       NewMarquee("", marqueeSep),
		LibraryState: LibraryState{
			tracks: tracks,
		},
	}
	a.applyFilter()
	return a
}

func TestVolumePercent(t *testing.T) {
	tests := []struct {
		volume float64
		want   int
	}{
		{0, 0},
		{0.5, 50},
		{1.0, 100},
		{1.5, 100},
		{2.0, 100},
	}
	for _, tt := range tests {
		if got := volumePercent(tt.volume); got != tt.want {
			t.Errorf("volumePercent(%v) = %d, want %d", tt.volume, got, tt.want)
		}
	}
}

func TestInitStartsLibraryScan(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	a := &App{st: st, musicDir: t.TempDir()}
	a.Init()
	if !a.loading {
		t.Fatal("Init did not enter the loading state")
	}

	noStore := &App{}
	noStore.Init()
	if noStore.loading {
		t.Fatal("Init started a scan without a store")
	}
}

func TestGlobalCtrlRRespectsTextInputs(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	ctrlR := tea.KeyMsg{Type: tea.KeyCtrlR}

	a := &App{st: st, musicDir: t.TempDir()}
	a.handleKey(ctrlR)
	if !a.loading {
		t.Fatal("Ctrl+R in a main view did not start a scan")
	}

	editing := &App{st: st, musicDir: t.TempDir(), activeOvl: overlaySettings, settingsEditing: true}
	editing.handleKey(ctrlR)
	if editing.loading {
		t.Fatal("Ctrl+R in settings editing mode started a scan")
	}

	naming := &App{st: st, musicDir: t.TempDir(), PlaylistState: PlaylistState{inputMode: playlistInputCreate}}
	naming.handleKey(ctrlR)
	if naming.loading {
		t.Fatal("Ctrl+R in playlist naming mode started a scan")
	}

	adding := &App{st: st, musicDir: t.TempDir(), OverlayState: OverlayState{addingTracks: true}}
	adding.handleKey(ctrlR)
	if adding.loading {
		t.Fatal("Ctrl+R in add-tracks mode started a scan")
	}

	searching := testApp()
	searching.st = st
	searching.musicDir = t.TempDir()
	searching.activeOvl = overlaySearch
	searching.handleKey(ctrlR)
	if searching.loading {
		t.Fatal("Ctrl+R in search mode started a scan")
	}
}

func TestSearchFilterStaysVisibleAndEscClears(t *testing.T) {
	a := testApp(
		library.Track{ID: "1", Title: "foo"},
		library.Track{ID: "2", Title: "bar"},
	)
	a.searchInput.SetValue("foo")
	a.lastQuery = "foo"
	a.applyFilter()
	if a.filteredLen() != 1 {
		t.Fatalf("filtered len = %d, want 1", a.filteredLen())
	}

	a.activeOvl = overlaySearch
	a.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.searchInput.Value() != "foo" {
		t.Fatal("Enter cleared the search query")
	}
	if a.activeOvl != overlayNone {
		t.Fatal("Enter did not close the search overlay")
	}

	a.handleSharedTrackListKey(a.localTrackListCtx(), tea.KeyMsg{Type: tea.KeyEsc})
	if a.searchInput.Value() != "" {
		t.Fatal("Esc did not clear the search query")
	}
	if a.filteredLen() != 2 {
		t.Fatalf("filtered len after clear = %d, want 2", a.filteredLen())
	}
}

func TestJumpToPlayingAndPaging(t *testing.T) {
	tracks := make([]library.Track, 10)
	for i := range tracks {
		tracks[i] = library.Track{ID: string(rune('a' + i)), Title: "t"}
	}
	a := testApp(tracks...)
	cur := 0
	ctx := TrackListContext{
		Count:     a.filteredLen,
		TrackAt:   a.localTrackListCtx().TrackAt,
		CursorPos: &cur,
		PlayingPos: func(id string) int {
			return a.filteredPos(id)
		},
		PageSize: func() int { return 3 },
	}
	a.currentTrack = &tracks[7]

	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cur != 7 {
		t.Fatalf("cursor after c = %d, want 7", cur)
	}

	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyPgDown})
	if cur != 9 {
		t.Fatalf("cursor after PageDown = %d, want 9", cur)
	}
	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyPgDown})
	if cur != 9 {
		t.Fatalf("cursor past end = %d, want 9", cur)
	}
	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyPgUp})
	if cur != 6 {
		t.Fatalf("cursor after PageUp = %d, want 6", cur)
	}

	a.currentTrack = nil
	a.handleSharedTrackListKey(ctx, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if a.statusMsg != T("no_track_selected") {
		t.Fatalf("status = %q, want no-track hint", a.statusMsg)
	}
}

func TestDeletePlaylistConfirmKeys(t *testing.T) {
	a := &App{activeOvl: overlayDeletePlaylist}
	a.handleDeletePlaylistKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.activeOvl != overlayNone {
		t.Fatal("Esc did not cancel delete")
	}

	a.activeOvl = overlayDeletePlaylist
	a.handleDeletePlaylistKey(tea.KeyMsg{Type: tea.KeyEnter})
	if a.activeOvl != overlayNone {
		t.Fatal("Enter did not close the delete overlay")
	}
}

func TestChineseErrorStrings(t *testing.T) {
	old := ActiveLang()
	defer SetLang(old)
	SetLang(LangZH)

	if got := T("sync_error"); got == "sync_error" || got == "" {
		t.Fatalf("missing zh sync_error translation: %q", got)
	}
	if got := T("unknown_artist"); got != "未知艺术家" {
		t.Fatalf("unknown_artist = %q", got)
	}
}
