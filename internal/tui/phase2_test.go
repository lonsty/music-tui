package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
)

func phase2Tracks() []library.Track {
	return []library.Track{
		{ID: "1", Title: "Blue", Artist: "A", AlbumArtist: "A", Album: "Kind", TrackNumber: "1", FileFormat: "FLAC"},
		{ID: "2", Title: "Green", Artist: "A", AlbumArtist: "A", Album: "Kind", TrackNumber: "2", FileFormat: "MP3"},
		{ID: "3", Title: "Red", Artist: "B", AlbumArtist: "B", Album: "Other", TrackNumber: "1", FileFormat: "MP3"},
		{ID: "4", Title: "Untagged"},
	}
}

func TestSearchQuerySemantics(t *testing.T) {
	tracks := phase2Tracks()
	tests := []struct {
		query string
		want  []string
	}{
		{"", []string{"1", "2", "3", "4"}},
		{"blue", []string{"1"}},
		{"BLUE", []string{"1"}},
		{"s:a", []string{"1", "2"}},
		{"a:kind", []string{"1", "2"}},
		{"t:red", []string{"3"}},
		{"f:flac", []string{"1"}},
		{"b", []string{"1", "3"}}, // title/artist match
	}
	for _, tt := range tests {
		idxs := selectTrackIndices(tracks, tt.query, formatPrefAll)
		var got []string
		for _, i := range idxs {
			got = append(got, tracks[i].ID)
		}
		if strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("query %q = %v, want %v", tt.query, got, tt.want)
		}
	}
}

func TestAddPanelUsesSameSearchSemantics(t *testing.T) {
	tracks := phase2Tracks()
	a := testApp(tracks...)
	a.addInput = textinput.New()

	for _, q := range []string{"", "blue", "s:a", "a:kind", "t:red", "f:flac"} {
		a.addInput.SetValue(q)
		a.applyAddFilter()
		want := selectTrackIndices(tracks, q, a.formatPref)
		if strings.Join(intsToStrings(a.addResults), ",") != strings.Join(intsToStrings(want), ",") {
			t.Errorf("add filter %q = %v, want %v", q, a.addResults, want)
		}
	}
}

func intsToStrings(v []int) []string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = string(rune('0' + n))
	}
	return out
}

func TestBuildTrackGroups(t *testing.T) {
	tracks := phase2Tracks()
	albums := buildTrackGroups(tracks, libraryViewAlbums)
	if len(albums) != 3 {
		t.Fatalf("album groups = %d, want 3", len(albums))
	}
	if len(albums[0].IDs) != 2 || albums[0].Title != "Kind" {
		t.Fatalf("first album = %#v", albums[0])
	}
	if albums[2].Title != "" {
		t.Fatalf("unknown album title = %q, want empty", albums[2].Title)
	}

	artists := buildTrackGroups(tracks, libraryViewArtists)
	if len(artists) != 3 {
		t.Fatalf("artist groups = %d, want 3", len(artists))
	}
	if len(artists[0].IDs) != 2 || artists[0].Title != "A" {
		t.Fatalf("first artist = %#v", artists[0])
	}
}

func TestDrilldownAndEscRestoresGroupView(t *testing.T) {
	a := testApp(phase2Tracks()...)
	a.view = libraryViewAlbums
	a.prevView = libraryViewTracks
	a.groupCursor = 0

	if cmd := a.enterGroup(); cmd != nil {
		t.Fatal("enterGroup returned a command")
	}
	if a.view != libraryViewTracks || a.drilldown == nil {
		t.Fatal("enterGroup did not switch to the drilled track list")
	}
	if a.filteredLen() != 2 {
		t.Fatalf("drilled tracks = %d, want 2", a.filteredLen())
	}
	if got := a.filteredTrack(0).ID; got != "1" {
		t.Fatalf("first drilled track = %s, want 1", got)
	}

	a.handleSharedTrackListKey(a.localTrackListCtx(), tea.KeyMsg{Type: tea.KeyEsc})
	if a.view != libraryViewAlbums || a.drilldown != nil {
		t.Fatal("Esc did not restore the album view")
	}
	if a.groupCursor != 0 {
		t.Fatalf("group cursor = %d, want 0", a.groupCursor)
	}
	if a.filteredLen() != 4 {
		t.Fatalf("tracks after exit = %d, want 4", a.filteredLen())
	}
}

func TestSearchForcesTracksThenRestoresView(t *testing.T) {
	a := testApp(phase2Tracks()...)
	a.view = libraryViewAlbums
	a.prevView = libraryViewTracks

	a.syncSearchLibraryView(true)
	if a.view != libraryViewTracks {
		t.Fatal("active search did not force the track view")
	}
	a.syncSearchLibraryView(false)
	if a.view != libraryViewAlbums {
		t.Fatal("cleared search did not restore the album view")
	}
}

func TestEmptySearchResultShowsQuery(t *testing.T) {
	a := testApp(phase2Tracks()...)
	a.W, a.H = 100, 30
	a.activeOvl = overlayNone
	a.searchInput.SetValue("zzz")
	a.applyFilter()

	out := a.renderSharedTrackList(a.localTrackListCtx(), 40, 10)
	if !strings.Contains(out, "zzz") {
		t.Fatalf("empty result does not mention the query:\n%s", out)
	}
}
