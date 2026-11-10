package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
)

func TestQueueEnqueueDedupes(t *testing.T) {
	a := &App{}
	if !a.enqueue("1", true) {
		t.Fatal("enqueue next failed")
	}
	if !a.enqueue("2", false) {
		t.Fatal("enqueue back failed")
	}
	if a.enqueue("1", true) {
		t.Fatal("duplicate enqueue succeeded")
	}
	if len(a.ids) != 2 || a.ids[0] != "1" || a.ids[1] != "2" {
		t.Fatalf("queue = %v, want [1 2]", a.ids)
	}
}

func TestPopPlayableQueueSkipsMissing(t *testing.T) {
	a := &App{LibraryState: LibraryState{tracks: []library.Track{{ID: "2"}}}}
	a.ids = []string{"missing", "2"}
	if got := a.popPlayableQueueID(); got != "2" {
		t.Fatalf("popped %q, want 2", got)
	}
	if len(a.ids) != 0 {
		t.Fatalf("queue not drained: %v", a.ids)
	}
	if got := a.popPlayableQueueID(); got != "" {
		t.Fatalf("empty queue popped %q", got)
	}
}

func TestAdvanceTakesQueueBeforePlayMode(t *testing.T) {
	a := testApp(
		library.Track{ID: "1", Title: "one"},
		library.Track{ID: "2", Title: "two"},
	)
	a.playMode = playModeSingle
	a.currentTrack = &a.tracks[0]
	a.ids = []string{"2"}

	cmd := a.cmdAdvance()
	if cmd == nil {
		t.Fatal("cmdAdvance returned nil with a queued track")
	}
	if len(a.ids) != 0 {
		t.Fatalf("queue = %v, want empty after advance", a.ids)
	}
}

func TestAdvanceFallsBackWhenQueueEmpty(t *testing.T) {
	a := testApp(
		library.Track{ID: "1", Title: "one"},
		library.Track{ID: "2", Title: "two"},
	)
	a.playMode = playModeSingle
	a.currentTrack = &a.tracks[0]

	if cmd := a.cmdAdvance(); cmd == nil {
		t.Fatal("cmdAdvance returned nil with an empty queue")
	}
	if len(a.ids) != 0 {
		t.Fatalf("queue = %v, want empty", a.ids)
	}
}

func TestQueuePlayByIDKeepsFilteredList(t *testing.T) {
	a := testApp(
		library.Track{ID: "1", Title: "one"},
		library.Track{ID: "2", Title: "two"},
	)
	before := append([]int(nil), a.filteredIdxs...)
	a.cmdPlayTrackByID("2")
	if len(a.filteredIdxs) != len(before) {
		t.Fatalf("filteredIdxs changed: %v -> %v", before, a.filteredIdxs)
	}
}

func TestQueueOverlayKeys(t *testing.T) {
	a := &App{QueueState: QueueState{ids: []string{"1", "2", "3"}}}
	a.handleQueueKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if len(a.ids) != 2 || a.ids[0] != "2" {
		t.Fatalf("queue after remove = %v", a.ids)
	}
	a.cursor = 5
	a.handleQueueKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if a.cursor != 0 {
		t.Fatalf("cursor after clamp = %d, want 0", a.cursor)
	}
	a.handleQueueKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if len(a.ids) != 0 {
		t.Fatalf("queue after clear = %v", a.ids)
	}
	a.activeOvl = overlayQueue
	a.handleQueueKey(tea.KeyMsg{Type: tea.KeyEsc})
	if a.activeOvl != overlayNone {
		t.Fatal("Esc did not close the queue overlay")
	}
}

func TestQueueSurvivesTabSwitch(t *testing.T) {
	a := testApp()
	a.ids = []string{"1"}
	a.activeTab = tabPlaylist
	a.activeTab = tabLocal
	if len(a.ids) != 1 {
		t.Fatal("queue was cleared by a tab switch")
	}
}

func TestQueueOverlayRendersEmptyAndItems(t *testing.T) {
	a := &App{W: 100, H: 30}
	if out := a.renderQueueOverlay(); !strings.Contains(out, T("queue_empty")) {
		t.Fatalf("empty queue overlay missing empty hint:\n%s", out)
	}
	a.tracks = []library.Track{{ID: "1", Title: "one", Artist: "A"}}
	a.ids = []string{"1"}
	if out := a.renderQueueOverlay(); !strings.Contains(out, "one") {
		t.Fatalf("queue overlay missing track title:\n%s", out)
	}
}

func TestQueueCountChipRendered(t *testing.T) {
	p, err := audio.NewPlayer()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	a := &App{player: p, W: 120, H: 30, activeTab: tabLocal}
	a.ids = []string{"1", "2"}
	out := a.renderStatusBar()
	if !strings.Contains(out, fmt.Sprintf(T("queue_count"), 2)) {
		t.Fatalf("status bar missing queue chip:\n%s", out)
	}
}
