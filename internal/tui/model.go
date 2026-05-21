package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/library"
)

// ── View ─────────────────────────────────────────────────────────────────────

// view represents the top-level screen being shown.
type view int

const (
	viewNormal     view = iota // Two-panel: track list + mini player
	viewFullscreen             // Full-screen player + lyrics
)

// ── Overlay ───────────────────────────────────────────────────────────────────

// overlay is a modal layer drawn on top of the current view.
type overlay int

const (
	overlayNone   overlay = iota
	overlayHelp            // ? key
	overlaySearch          // / key
	overlayInfo            // i key — track detail
)

// ── Tab ───────────────────────────────────────────────────────────────────────

// tabID identifies the active top-level tab.
type tabID int

const (
	tabLocal  tabID = iota
	tabOnline         // placeholder — not yet implemented
)

// ── Play mode ────────────────────────────────────────────────────────────────

// playMode controls how the next track is chosen when the current one ends.
type playMode int

const (
	playModeSequential playMode = iota // play list top-to-bottom, stop at end
	playModeLoop                       // repeat list indefinitely
	playModeSingle                     // repeat current track
	playModeRandom                     // pick a random next track
)

// playModeIcon returns the Nerd Font glyph for the given mode.
func playModeIcon(m playMode) string {
	switch m {
	case playModeSequential:
		return "󰒿" // nf-md-arrow_right
	case playModeLoop:
		return "󰑖" // nf-md-repeat
	case playModeSingle:
		return "󰑘" // nf-md-repeat_once
	case playModeRandom:
		return "󰒝" // nf-md-shuffle
	}
	return "?"
}

// playModeName returns a short label for display.
func playModeName(m playMode) string {
	switch m {
	case playModeSequential:
		return "Sequential"
	case playModeLoop:
		return "Loop"
	case playModeSingle:
		return "Single"
	case playModeRandom:
		return "Random"
	}
	return ""
}

// ── Tea messages ─────────────────────────────────────────────────────────────

// tickMsg is fired every 500 ms to refresh the progress bar.
type tickMsg time.Time

// trackDoneMsg is sent by the audio player when a track ends naturally.
type trackDoneMsg struct{}

// scanDoneMsg carries the result of a background library scan.
type scanDoneMsg struct {
	tracks []library.Track
	err    error
}

// playResultMsg is returned after a play attempt.
type playResultMsg struct {
	track *library.Track // nil on error
	idx   int
	err   error
}

// noopMsg is returned by Cmds that have no state to communicate.
// Never return nil from a tea.Cmd — it causes a panic.
type noopMsg struct{}

// tick fires a tickMsg after 500 ms.
func tick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
