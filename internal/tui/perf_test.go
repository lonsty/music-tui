package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
)

func perfTracks() []library.Track {
	return []library.Track{
		{ID: "1", Title: "one", Artist: "a", Album: "x", FileFormat: "MP3"},
		{ID: "2", Title: "two", Artist: "b", Album: "y", FileFormat: "FLAC"},
		{ID: "3", Title: "three", Artist: "c", Album: "z", FileFormat: "MP3"},
	}
}

func TestIdentityIndicesReused(t *testing.T) {
	a := testApp(perfTracks()...)
	first := a.filteredIdxs
	a.applyFilter()
	second := a.filteredIdxs
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("expected a non-empty identity table")
	}
	if &first[0] != &second[0] {
		t.Fatal("empty-query filtering allocated a new index table")
	}
}

func TestSearchDoesNotRebuildGroups(t *testing.T) {
	a := testApp(perfTracks()...)
	a.groupsBuilt = true
	a.groupsDirty = false
	a.albumGroups = []trackGroup{{Key: "sentinel"}}
	a.groupsPref = a.formatPref

	a.searchInput.SetValue("one")
	a.applyFilter()

	if a.groupsDirty {
		t.Fatal("search query marked groups dirty")
	}
	if len(a.albumGroups) != 1 || a.albumGroups[0].Key != "sentinel" {
		t.Fatal("search query rebuilt the group cache")
	}
}

func TestNonTextMessageDoesNotRefilter(t *testing.T) {
	a := testApp(perfTracks()...)
	a.activeOvl = overlaySearch
	a.searchInput.Focus()
	a.searchInput.SetValue("one")
	a.applyFilter()

	before := a.filteredIdxs
	a.Update(struct{}{})
	if &before[0] != &a.filteredIdxs[0] {
		t.Fatal("a non-text message rebuilt the filtered index table")
	}

	// A real edit must still refilter.
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if a.searchInput.Value() != "onex" {
		t.Fatalf("search input value = %q, want onex", a.searchInput.Value())
	}
	if len(a.filteredIdxs) != 0 {
		t.Fatalf("filtered %d tracks for a no-match query, want 0", len(a.filteredIdxs))
	}
}

func TestQueueLookupUsesPositionIndex(t *testing.T) {
	a := testApp(perfTracks()...)
	got := a.findTrackByID("2")
	if got == nil || got.Title != "two" {
		t.Fatalf("findTrackByID = %+v, want track two", got)
	}
	if a.findTrackByID("missing") != nil {
		t.Fatal("findTrackByID returned a track for an unknown ID")
	}
	if len(a.trackPosByID) != len(a.tracks) {
		t.Fatal("track position index was not built")
	}
}

func TestLyricsWidthComputedOnce(t *testing.T) {
	a := testApp()
	a.synced = true
	a.lines = []lyrics.Line{
		{Time: 0, Text: "short"},
		{Time: time.Second, Text: "a much longer line"},
	}
	a.activeIdx = 0

	_ = a.renderLyricsScroll(60, 10)
	want := strWidth("a much longer line")
	if a.maxTextW != want {
		t.Fatalf("maxTextW = %d, want %d", a.maxTextW, want)
	}
}

func TestMarqueeCachesTextWidth(t *testing.T) {
	m := NewMarquee("hello", marqueeSep)
	if m.textW != 5 {
		t.Fatalf("textW = %d, want 5", m.textW)
	}
	m.SetText("世界")
	if m.textW != 4 {
		t.Fatalf("textW = %d, want 4 for two CJK characters", m.textW)
	}
}

func TestCoverPlaceholderIsCached(t *testing.T) {
	a := testApp(perfTracks()...)
	a.currentTrack = &a.tracks[0]

	first := a.getCoverArt(40, 20)
	second := a.getCoverArt(40, 20)
	if first != second {
		t.Fatal("placeholder changed between identical calls")
	}
	if !a.coverPlaceholder {
		t.Fatal("placeholder result was not marked as cached placeholder")
	}
}

func TestRenderUsesFramePlaybackSnapshot(t *testing.T) {
	a := testApp(perfTracks()...)
	a.currentTrack = &a.tracks[0]
	a.mqTitle = NewMarquee("", marqueeSep)
	a.mqMeta = NewMarquee("", marqueeSep)
	a.mqArtist = NewMarquee("", marqueeSep)
	a.mqAlbum = NewMarquee("", marqueeSep)
	a.progressBar = progress.New(progress.WithoutPercentage())
	a.syncMarquees()
	a.framePlayback = playbackFrame{
		position: 90 * time.Second,
		duration: 120 * time.Second,
	}

	// No player is set: rendering must not touch a.player.
	out := a.buildMiniPlayerContent(60, 24)
	if !strings.Contains(out, "01:30 / 02:00") {
		t.Fatalf("rendered time does not use the frame snapshot:\n%s", out)
	}
}

func TestSettingsOverlayUsesPredeclaredStyles(t *testing.T) {
	a := testApp(perfTracks()...)
	a.W, a.H = 100, 30
	a.activeOvl = overlaySettings
	a.musicDirInput = textinput.New()
	if out := a.renderSettingsOverlay(); out == "" {
		t.Fatal("settings overlay rendered empty")
	}
}
