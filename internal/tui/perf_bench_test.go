package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
)

// benchTracks10K builds a deterministic 10k-track library with repeated
// album/artist structure so format-preference grouping stays realistic.
func benchTracks10K() []library.Track {
	tracks := make([]library.Track, 10000)
	for i := range tracks {
		format := "MP3"
		if i%3 == 0 {
			format = "FLAC"
		}
		tracks[i] = library.Track{
			ID:          fmt.Sprintf("id-%d", i),
			Path:        fmt.Sprintf("/music/%d/file-%d.mp3", i/10, i),
			Title:       fmt.Sprintf("Track %d", i),
			Artist:      fmt.Sprintf("Artist %d", i/20),
			AlbumArtist: fmt.Sprintf("Artist %d", i/20),
			Album:       fmt.Sprintf("Album %d", i/100),
			TrackNumber: fmt.Sprintf("%d", i%100+1),
			Year:        "2000",
			FileFormat:  format,
			Duration:    3 * time.Minute,
		}
	}
	return tracks
}

// benchApp10K builds a renderable App backed by the 10k-track library.
func benchApp10K(b *testing.B) *App {
	b.Helper()
	a := &App{
		musicDir:  "/music",
		W:         120,
		H:         40,
		activeTab: tabLocal,

		searchInput: textinput.New(),
		progressBar: progress.New(progress.WithoutPercentage()),

		mqTitle:  NewMarquee("", marqueeSep),
		mqMeta:   NewMarquee("", marqueeSep),
		mqArtist: NewMarquee("", marqueeSep),
		mqAlbum:  NewMarquee("", marqueeSep),
		mqRow:    NewMarquee("", marqueeSep),
		mqPlRow:  NewMarquee("", marqueeSep),

		PlaybackState: PlaybackState{volume: 0.8},
		LibraryState:  LibraryState{tracks: benchTracks10K(), formatPref: formatPrefAll},
		LyricsState:   LyricsState{activeIdx: -1, prevActiveIdx: -1, browseCenterIdx: -1},
	}
	a.applyFilter()
	a.cursorPos = 5000
	a.currentTrack = &a.tracks[0]
	a.syncMarquees()
	a.syncRowMarquee()
	return a
}

func BenchmarkApplyFilter10K(b *testing.B) {
	cases := []struct {
		name string
		set  func(*App)
	}{
		{"Empty", func(a *App) { a.searchInput.SetValue("") }},
		{"Prefix", func(a *App) { a.searchInput.SetValue("a:album 5") }},
		{"LosslessFirst", func(a *App) { a.formatPref = formatPrefLosslessFirst }},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			a := benchApp10K(b)
			tc.set(a)
			b.ResetTimer()
			for range b.N {
				a.applyFilter()
			}
		})
	}
}

func BenchmarkSelectTrackIndices10K(b *testing.B) {
	tracks := benchTracks10K()
	cases := []struct {
		name  string
		query string
		pref  formatPreference
	}{
		{"Empty", "", formatPrefAll},
		{"Title", "t:track 99", formatPrefAll},
		{"LosslessFirst", "artist 5", formatPrefLosslessFirst},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = selectTrackIndices(tracks, tc.query, tc.pref)
			}
		})
	}
}

func BenchmarkViewNormal10K(b *testing.B) {
	a := benchApp10K(b)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = a.View()
	}
}

func BenchmarkViewFullscreen10K(b *testing.B) {
	a := benchApp10K(b)
	a.currentView = viewFullscreen
	a.lines = make([]lyrics.Line, 300)
	for i := range a.lines {
		a.lines[i] = lyrics.Line{Time: time.Duration(i) * time.Second, Text: fmt.Sprintf("Lyric line %d", i)}
	}
	a.synced = true
	a.activeIdx = 150
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = a.View()
	}
}

func BenchmarkRenderQueueOverlay200(b *testing.B) {
	a := benchApp10K(b)
	a.activeOvl = overlayQueue
	a.ids = make([]string, 200)
	for i := range a.ids {
		a.ids[i] = fmt.Sprintf("id-%d", i*37)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = a.renderQueueOverlay()
	}
}

func BenchmarkUpdateKey10K(b *testing.B) {
	a := benchApp10K(b)
	msg := tea.KeyMsg{Type: tea.KeyDown}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		a.Update(msg)
	}
}
