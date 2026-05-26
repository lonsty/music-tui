package tui

import (
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

func (a *App) renderStatusBar() string {
	if a.loading {
		return styleStatusLine.Render("  󰔟  " + T("scanning_library"))
	}
	if a.scanErr != nil {
		return styleStatusLine.Render("  󰅚  " + a.scanErr.Error())
	}

	state := a.player.State()

	// During a track change the player briefly passes through StateStopped
	// (old stream torn down, new stream not yet started).  Showing "Stopped"
	// for those few frames causes a distracting flash.  When a track is
	// loaded, treat the transient stopped state as Playing so the chip stays
	// stable until the new stream reports its real state.
	displayState := state
	if displayState == audio.StateStopped && a.currentTrack != nil {
		displayState = audio.StatePlaying
	}

	stateLabel := stateLabel(displayState)
	stateChip := styleStatusState.Render(stateLabel)

	// Build hint chips: [key] label
	hint := func(key, label string) string {
		return styleStatusKey.Render(" "+key+" ") +
			styleStatusHintLabel.Render(" "+label+"  ")
	}

	var hints string
	if a.currentView == viewFullscreen {
		pauseLabel := T("hint_pause")
		if displayState == audio.StatePaused {
			pauseLabel = T("hint_resume")
		}
		hints = hint("Esc", T("hint_back")) + hint("Spc", pauseLabel) +
			hint("n", T("hint_next")) + hint("p", T("hint_prev")) +
			hint("</>", T("hint_seek")) + hint("+/-", T("hint_vol")) +
			hint("m", T("hint_mode")) + hint("b", "8-bit") + hint("q", T("hint_quit"))
	} else {
		switch {
		case displayState == audio.StatePlaying:
			hints = hint("Spc", T("hint_pause")) + hint("n", T("hint_next")) + hint("p", T("hint_prev")) +
				hint("</>", T("hint_seek")) + hint("+/-", T("hint_vol")) +
				hint("/", T("hint_search")) + hint("?", T("hint_help")) + hint("q", T("hint_quit"))
		case displayState == audio.StatePaused:
			hints = hint("Spc", T("hint_resume")) + hint("n", T("hint_next")) + hint("p", T("hint_prev")) +
				hint("</>", T("hint_seek")) + hint("+/-", T("hint_vol")) +
				hint("/", T("hint_search")) + hint("?", T("hint_help")) + hint("q", T("hint_quit"))
		case a.currentTrack != nil:
			// Stopped but a track is loaded (e.g. briefly between track changes).
			// Show playback hints to avoid a flash of "Enter Play" during seeks.
			hints = hint("Spc", T("hint_resume")) + hint("n", T("hint_next")) + hint("p", T("hint_prev")) +
				hint("</>", T("hint_seek")) + hint("+/-", T("hint_vol")) +
				hint("/", T("hint_search")) + hint("?", T("hint_help")) + hint("q", T("hint_quit"))
		default:
			// Truly stopped with no track — guide the user to load one.
			hints = hint("Enter", T("hint_play")) + hint("/", T("hint_search")) +
				hint(",", T("hint_settings")) + hint("?", T("hint_help")) + hint("q", T("hint_quit"))
		}
	}

	if a.statusMsg != "" {
		hints = styleStatusHintLabel.Render("  " + a.statusMsg)
	}

	modeChip := styleModeIcon.Render(" " + playModeIcon(a.playMode) + " ")

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

	line := " " + stateChip + "  " + modeChip + retroChip + chipChip + "  " + hints
	// No Width — don't pad with background colour to the right edge.
	return styleStatusLine.Render(line)
}
