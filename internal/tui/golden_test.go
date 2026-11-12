package tui

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/store"
)

// updateGolden regenerates testdata/golden when set.  Run:
//
//	go test ./internal/tui -run Golden -update-golden
var updateGolden = flag.Bool("update-golden", false, "rewrite golden render fixtures")

// goldenTracks is a deterministic fixture: no cover art, fixed durations and
// metadata so terminal art rendering cannot depend on the host filesystem.
func goldenTracks() []library.Track {
	return []library.Track{
		{ID: "t1", Path: "/music/01.flac", Title: "Blue in Green", Artist: "Miles Davis", AlbumArtist: "Miles Davis", Album: "Kind of Blue", TrackNumber: "1", Year: "1959", FileFormat: "FLAC", Duration: 5*60*1e9 + 30e9},
		{ID: "t2", Path: "/music/02.flac", Title: "Freddie Freeloader", Artist: "Miles Davis", AlbumArtist: "Miles Davis", Album: "Kind of Blue", TrackNumber: "2", Year: "1959", FileFormat: "FLAC", Duration: 9*60*1e9 + 45e9},
		{ID: "t3", Path: "/music/03.mp3", Title: "So What", Artist: "Miles Davis", AlbumArtist: "Miles Davis", Album: "Kind of Blue", TrackNumber: "3", Year: "1959", FileFormat: "MP3", Duration: 9*60*1e9 + 20e9},
		{ID: "t4", Path: "/music/04.flac", Title: "Naima", Artist: "John Coltrane", AlbumArtist: "John Coltrane", Album: "Giant Steps", TrackNumber: "1", Year: "1960", FileFormat: "FLAC", Duration: 4*60*1e9 + 20e9},
		{ID: "t5", Path: "/music/05.mp3", Title: "Giant Steps", Artist: "John Coltrane", AlbumArtist: "John Coltrane", Album: "Giant Steps", TrackNumber: "2", Year: "1960", FileFormat: "MP3", Duration: 4*60*1e9 + 45e9},
	}
}

func goldenLyrics() []lyrics.Line {
	return []lyrics.Line{
		{Time: 0, Text: "First line"},
		{Time: 5e9, Text: "Second line"},
		{Time: 10e9, Text: "Third line"},
		{Time: 15e9, Text: "Fourth line"},
		{Time: 20e9, Text: "Fifth line"},
		{Time: 25e9, Text: "Sixth line"},
	}
}

// newGoldenApp builds a fully initialised App with no player, so rendering
// uses the zero playback frame.  Every field touched by View is set.
func newGoldenApp(t *testing.T) *App {
	t.Helper()

	oldLang, oldIcons := ActiveLang(), ActiveIconSet()
	t.Cleanup(func() {
		SetLang(oldLang)
		setIconSet(oldIcons)
	})
	SetLang(LangEN)
	setIconSet(iconSetNerdFont)

	a := &App{
		musicDir:  "/music",
		W:         100,
		H:         30,
		activeTab: tabLocal,

		searchInput:   textinput.New(),
		settingsInput: textinput.New(),
		musicDirInput: textinput.New(),

		progressBar: progress.New(
			progress.WithGradient("#89B4FA", "#CBA6F7"),
			progress.WithoutPercentage(),
		),

		mqTitle:  NewMarquee("", marqueeSep),
		mqMeta:   NewMarquee("", marqueeSep),
		mqArtist: NewMarquee("", marqueeSep),
		mqAlbum:  NewMarquee("", marqueeSep),
		mqRow:    NewMarquee("", marqueeSep),
		mqPlRow:  NewMarquee("", marqueeSep),

		PlaybackState: PlaybackState{volume: 0.8, playMode: playModeLoop},
		LibraryState:  LibraryState{tracks: goldenTracks(), formatPref: formatPrefAll},
		LyricsState:   LyricsState{activeIdx: -1, prevActiveIdx: -1, browseCenterIdx: -1},
	}

	a.applyFilter()
	a.rebuildGroups()
	a.currentTrack = &a.tracks[0]
	a.syncMarquees()
	a.syncRowMarquee()
	return a
}

func TestGoldenRender(t *testing.T) {
	cases := []struct {
		name string
		prep func(a *App)
	}{
		{"normal", func(a *App) {
			a.cursorPos = 1
			a.syncRowMarquee()
		}},
		{"search", func(a *App) {
			a.activeOvl = overlaySearch
			a.searchInput.SetValue("blue")
			a.applyFilter()
		}},
		{"album-groups", func(a *App) {
			a.view = libraryViewAlbums
		}},
		{"fullscreen", func(a *App) {
			a.currentView = viewFullscreen
			a.lines = goldenLyrics()
			a.activeIdx = 2
			a.synced = true
			a.mqTitle.SetText(a.currentTrack.DisplayTitle())
			a.mqArtist.SetText(a.currentTrack.DisplayArtist())
			a.mqAlbum.SetText(a.currentTrack.Album)
		}},
		{"fullscreen-plain-lyrics", func(a *App) {
			a.currentView = viewFullscreen
			a.lines = goldenLyrics()
			a.synced = false
			a.activeIdx = -1
		}},
		{"help", func(a *App) {
			a.activeOvl = overlayHelp
			// Tall viewport so the golden captures every generated section.
			a.H = 120
		}},
		{"settings", func(a *App) {
			a.activeOvl = overlaySettings
			a.musicDirInput.SetValue("/music")
		}},
		{"queue", func(a *App) {
			a.activeOvl = overlayQueue
			a.ids = []string{"t2", "t4"}
			a.cursor = 0
		}},
		{"playlist-list", func(a *App) {
			a.activeTab = tabPlaylist
			a.depth = playlistDepthList
			a.playlists = []store.Playlist{
				{ID: store.FavoritesPlaylistID, Name: "Favorites", TrackCount: 3},
				{ID: "p2", Name: "Late Night", TrackCount: 2},
			}
			a.plListMaxColW = 2
		}},
		{"add-to-playlist", func(a *App) {
			a.activeOvl = overlayAddToPlaylist
			a.ovlPlaylists = []store.Playlist{
				{ID: store.FavoritesPlaylistID, Name: "Favorites"},
				{ID: "p2", Name: "Late Night"},
			}
		}},
	}

	dir := filepath.Join("testdata", "golden")
	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newGoldenApp(t)
			tc.prep(a)
			// Rendering twice must be stable; the second call is what golden
			// fixtures capture, so any first-frame-only state shows up here.
			_ = a.View()
			got := a.View()

			path := filepath.Join(dir, tc.name+".golden")
			if *updateGolden {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden (run with -update-golden): %v", err)
			}
			if got != string(want) {
				t.Fatalf("render mismatch for %s\n--- want ---\n%q\n--- got ---\n%q", tc.name, want, got)
			}
			})
	}

	// The help snapshot must cover the full generated table, not only the
	// first few chips.
	data, err := os.ReadFile(filepath.Join(dir, "help.golden"))
	if err != nil {
		t.Fatalf("read help golden: %v", err)
	}
	if lines := strings.Count(string(data), "\n"); lines <= 29 {
		t.Fatalf("help golden has %d lines, want more than 29", lines)
	}
}
