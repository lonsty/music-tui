package tui

import (
	"fmt"
	"strings"

	"github.com/lonsty/music-tui/internal/audio"
)

// ── Status bar ────────────────────────────────────────────────────────────────

// stateLabel returns the localised chip-label text for the given playback state,
// including surrounding spaces for visual padding inside the status chip.
// A function is used instead of a package-level var so the active language is
// read at render time, supporting runtime language switching.
func stateLabel(s audio.State) string {
	switch s {
	case audio.StatePlaying:
		return T("state_playing")
	case audio.StatePaused:
		return T("state_paused")
	default:
		return T("state_stopped")
	}
}

// statusHint is a single keyboard-shortcut hint shown in the status bar.
// When the terminal is too narrow to show all hints, they are dropped one
// by one from right to left.
type statusHint struct {
	key   string
	label string
}

// renderStatusBar renders the one-line status bar at the bottom of the screen.
//
// Layout: [state] [mode] [retro] [chip]  hint1 hint2 … hintN
//
// When the terminal width (a.W) is insufficient to show all hints, lower-
// priority hints are dropped from right to left until the rendered line fits.
// The prefix (state + mode chips) is always shown; if even that does not fit
// it is truncated by the terminal rather than by us.
func (a *App) renderStatusBar() string {
	if a.loading {
		return styleStatusLine.Render("  " + iconSpinner() + "  " + T("scanning_library"))
	}
	if a.scanErr != nil {
		return styleStatusLine.Render("  " + iconError() + "  " + fmt.Sprintf(T("sync_error"), a.scanErr.Error()))
	}

	state := audio.StateStopped
	if a.player != nil {
		state = a.player.State()
	}

	// During a track change the player briefly passes through StateStopped
	// (old stream torn down, new stream not yet started).  Showing "Stopped"
	// for those few frames causes a distracting flash.  When a track is
	// loaded, treat the transient stopped state as Playing so the chip stays
	// stable until the new stream reports its real state.
	displayState := state
	if displayState == audio.StateStopped && a.currentTrack != nil {
		displayState = audio.StatePlaying
	}

	stateChipStr := styleStatusState.Render(stateLabel(displayState))

	var retroChip string
	if a.retroIdx > 0 {
		retroChip = "  " + styleStatusState.Render(" "+retroLabel(a.retroIdx)+" ")
	}

	var chipChip string
	switch {
	case a.chipConverting:
		chipChip = "  " + styleStatusState.Render(T("chip_converting"))
	case a.chipBusy && a.chipMode:
		chipChip = "  " + styleStatusState.Render(T("chip_switching"))
	case a.chipBusy:
		chipChip = "  " + styleStatusState.Render(T("chip_busy"))
	case a.chipMode:
		chipChip = "  " + styleStatusState.Render(T("chip_active"))
	}

	var queueChip string
	if len(a.ids) > 0 {
		queueChip = "  " + styleStatusState.Render(" "+fmt.Sprintf(T("queue_count"), len(a.ids))+" ")
	}

	// Fixed prefix: always rendered.
	// Play mode icon is intentionally omitted here — it is shown in the
	// mini/fullscreen player controls and in the collapsed-header status line.
	//
	// Dynamic statusMsg width: when the status message is long, truncate it
	// so that at least a few hint chips remain visible.  Otherwise the status
	// bar would show only the message and no usable key hints.
	var msgChip string
	if a.statusMsg != "" {
		msgText := a.statusMsg
		// Reserve space for 2 hint chips (~12 cols each) plus the fixed prefix.
		const minHintW = 24
		basePrefix := " " + stateChipStr + retroChip + chipChip + queueChip + " "
		maxMsgW := a.W - strWidth(basePrefix) - minHintW - 2 // -2 = trailing "  "
		if maxMsgW < 8 {
			maxMsgW = 8
		}
		if strWidth(msgText) > maxMsgW {
			msgText = truncate(msgText, maxMsgW-1) + "\u2026" // single ellipsis char
		}
		msgChip = " " + styleStatusState.Render(" "+msgText+" ")
	}
	prefix := " " + stateChipStr + retroChip + chipChip + queueChip + msgChip + "  "
	prefixW := strWidth(prefix)

	// Build the prioritised hint list for the current state.
	// priority 1 = always shown (quit), priority 5 = dropped first (help, search).
	var hints []statusHint
	if a.currentView == viewFullscreen {
		pauseLabel := T("hint_pause")
		if displayState == audio.StatePaused {
			pauseLabel = T("hint_resume")
		}
		hints = []statusHint{
			{"?", T("hint_help")},
			{"Spc", pauseLabel},
			{"n/p", T("hint_next_prev")},
			{"</>", T("hint_seek")},
			{"+/-", T("hint_vol")},
			{"m", T("hint_mode")},
			{"b", "8-bit"},
			{"r/R", T("hint_retro")},
			{"Esc", T("hint_back")},
			{"q", T("hint_quit")},
		}
	} else if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
		// Playlist track level: delegate to the shared hint builder which
		// adds x/a hints and suppresses the "/" search hint.
		hints = buildTrackListHints(a, a.playlistTrackCtx(), displayState)
	} else if a.activeTab == tabPlaylist {
		hints = []statusHint{
			{"?", T("hint_help")},
			{"Enter", T("hint_enter_playlist")},
			{"n", T("hint_playlist_new")},
			{"r", T("hint_playlist_rename")},
			{"d", T("hint_playlist_delete")},
			{",", T("hint_settings")},
			{"Tab", T("hint_switch_tab")},
			{"q", T("hint_quit")},
		}
	} else if a.view != libraryViewTracks {
		hints = []statusHint{
			{"?", T("hint_help")},
			{"Enter", T("hint_open_group")},
			{"v", T("hint_view")},
			{"c", T("hint_jump_playing")},
			{"/", T("hint_search")},
			{",", T("hint_settings")},
			{"Tab", T("hint_switch_tab")},
			{"q", T("hint_quit")},
		}
	} else {
		// Local tab (and Online placeholder): delegate to the shared hint builder.
		hints = buildTrackListHints(a, a.localTrackListCtx(), displayState)
	}

	// Render a hint chip: [key] label
	renderHint := func(h statusHint) string {
		return styleStatusKey.Render(" "+h.key+" ") +
			styleStatusHintLabel.Render(" "+h.label+" ")
	}

	// Compute the display width of a rendered hint (no ANSI codes in the
	// measurement — use raw strings to estimate; close enough for layout).
	hintWidth := func(h statusHint) int {
		// styleStatusKey wraps " key " → 1+len(key)+1
		// styleStatusHintLabel wraps " label " → 1+len(label)+1
		return 1 + strWidth(h.key) + 1 + 1 + strWidth(h.label) + 1
	}

	// Drop hints from right to left until the remaining ones fit.
	available := a.W - prefixW
	if available < 0 {
		available = 0
	}
	// end is the first index to exclude (one past the last visible hint).
	end := len(hints)
	for end > 0 {
		total := 0
		for i := 0; i < end; i++ {
			total += hintWidth(hints[i])
		}
		if total <= available {
			break
		}
		end--
	}

	var sb strings.Builder
	for i := 0; i < end; i++ {
		sb.WriteString(renderHint(hints[i]))
	}

	line := prefix + sb.String()
	return styleStatusLine.Render(line)
}
