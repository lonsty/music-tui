package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
)

// ── View ─────────────────────────────────────────────────────────────────────

// tickInterval is the period between progress-refresh ticks.
// It is also the time unit for browseFadeOutTicks in actions.go.
const tickInterval = 500 * time.Millisecond

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
	overlayNone     overlay = iota
	overlayHelp             // ? key
	overlaySearch           // / key
	overlayInfo             // i key — track detail
	overlaySettings         // , key — settings
)

// ── Tab ───────────────────────────────────────────────────────────────────────

// tabID identifies the active top-level tab.
type tabID int

const (
	tabLocal  tabID = iota
	tabOnline       // placeholder — not yet implemented
)

// ── Play mode ────────────────────────────────────────────────────────────────

// playMode controls how the next track is chosen when the current one ends.
type playMode int

const (
	playModeSequential playMode = iota // play list top-to-bottom, stop at end
	playModeLoop                       // repeat list indefinitely
	playModeSingle                     // repeat current track
	playModeRandom                     // pick a random next track
	playModeCount                      // sentinel — must be the last constant
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

// chip8DoneMsg is sent by the background goroutine that runs p2chip.
// On success chipPath holds the converted mp3 path; err is non-nil on failure.
type chip8DoneMsg struct {
	originPath string // Track.Path of the source file
	chipPath   string // path to the generated 8-bit mp3
	err        error
}

// chipCrossfadeDoneMsg is sent after a chip-mode crossfade (on or off) completes.
// chipMode is the new desired state: true = now playing 8-bit, false = back to original.
type chipCrossfadeDoneMsg struct{ chipMode bool }

// lyricsLoadedMsg is sent by cmdLoadLyrics when lyrics have been fetched.
// lines is nil when no .lrc file exists for the track (not an error).
type lyricsLoadedMsg struct {
	trackID string
	lines   []lyrics.Line
}

// tick fires a tickMsg after tickInterval.
func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
