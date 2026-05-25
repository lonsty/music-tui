package tui

import (
	"github.com/lonsty/music-tui/internal/audio"
)

// ── Status bar ────────────────────────────────────────────────────────────────

func (a *App) renderStatusBar() string {
	if a.loading {
		return styleStatusLine.Render("  󰔟  Scanning library…")
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

	stateLabel := map[audio.State]string{
		audio.StateStopped: "  Stopped ",
		audio.StatePlaying: "  Playing ",
		audio.StatePaused:  "  Paused  ",
	}[displayState]
	stateChip := styleStatusState.Render(stateLabel)

	// Build hint chips: [key] label
	hint := func(key, label string) string {
		return styleStatusKey.Render(" "+key+" ") +
			styleStatusHintLabel.Render(" "+label+"  ")
	}

	var hints string
	if a.currentView == viewFullscreen {
		pauseLabel := "Pause"
		if displayState == audio.StatePaused {
			pauseLabel = "Resume"
		}
		hints = hint("Esc", "Back") + hint("Spc", pauseLabel) +
			hint("n", "Next") + hint("p", "Prev") +
			hint("+/-", "Vol") + hint("m", "Mode") +
			hint("b", "8-bit") + hint("q", "Quit")
	} else {
		switch {
		case displayState == audio.StatePlaying:
			hints = hint("Spc", "Pause") + hint("n", "Next") + hint("p", "Prev") +
				hint("+/-", "Vol") + hint("/", "Search") + hint("?", "Help") + hint("q", "Quit")
		case displayState == audio.StatePaused:
			hints = hint("Spc", "Resume") + hint("n", "Next") + hint("p", "Prev") +
				hint("+/-", "Vol") + hint("/", "Search") + hint("?", "Help") + hint("q", "Quit")
		case a.currentTrack != nil:
			// Stopped but a track is loaded (e.g. briefly between track changes).
			// Show playback hints to avoid a flash of "Enter Play" during seeks.
			hints = hint("Spc", "Resume") + hint("n", "Next") + hint("p", "Prev") +
				hint("+/-", "Vol") + hint("/", "Search") + hint("?", "Help") + hint("q", "Quit")
		default:
			// Truly stopped with no track — guide the user to load one.
			hints = hint("Enter", "Play") + hint("/", "Search") +
				hint(",", "Settings") + hint("?", "Help") + hint("q", "Quit")
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
		chipChip = "  " + styleStatusState.Render(" 8-bit Converting… ")
	case a.chipBusy && a.chipMode:
		chipChip = "  " + styleStatusState.Render(" 8-bit Switching… ")
	case a.chipBusy:
		chipChip = "  " + styleStatusState.Render(" 8-bit… ")
	case a.chipMode:
		chipChip = "  " + styleStatusState.Render(" 8-bit ")
	}

	line := " " + stateChip + "  " + modeChip + retroChip + chipChip + "  " + hints
	// No Width — don't pad with background colour to the right edge.
	return styleStatusLine.Render(line)
}
