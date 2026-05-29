package tui

import (
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
// priority controls which hints are dropped first when the terminal is narrow:
// lower value = higher priority (kept longer).
type statusHint struct {
	key      string
	label    string
	priority int
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
		return styleStatusLine.Render("  " + iconError() + "  " + a.scanErr.Error())
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

	// Fixed prefix: always rendered.
	// Play mode icon is intentionally omitted here — it is shown in the
	// mini/fullscreen player controls and in the collapsed-header status line.
	prefix := " " + stateChipStr + retroChip + chipChip + "  "
	prefixW := strWidth(prefix)

	// statusMsg overrides hints entirely.
	if a.statusMsg != "" {
		line := prefix + styleStatusHintLabel.Render(a.statusMsg)
		return styleStatusLine.Render(line)
	}

	// Build the prioritised hint list for the current state.
	// priority 1 = always shown (quit), priority 5 = dropped first (help, search).
	var hints []statusHint
	if a.currentView == viewFullscreen {
		pauseLabel := T("hint_pause")
		if displayState == audio.StatePaused {
			pauseLabel = T("hint_resume")
		}
		hints = []statusHint{
			{"Esc", T("hint_back"), 1},
			{"Spc", pauseLabel, 1},
			{"q", T("hint_quit"), 1},
			{"n", T("hint_next"), 2},
			{"p", T("hint_prev"), 2},
			{"</>", T("hint_seek"), 3},
			{"+/-", T("hint_vol"), 3},
			{"m", T("hint_mode"), 4},
			{"b", "8-bit", 5},
		}
	} else {
		// l hint: only shown when the right panel is visible and not collapsed.
		// In collapsed state the panel is hidden so the mode switch has no effect.
		lyricsHintLabel := T("hint_lyrics")
		if a.rightMode == rightPanelLyrics {
			lyricsHintLabel = T("hint_player")
		}
		showLyricsHint := a.showMiniPlayer() // false when collapsed or too narrow

		collapseHintLabel := T("hint_collapse")
		if a.rightCollapsed {
			collapseHintLabel = T("hint_expand")
		}

		switch {
		case displayState == audio.StatePlaying:
			hints = []statusHint{
				{"Spc", T("hint_pause"), 1},
				{"q", T("hint_quit"), 1},
				{"n", T("hint_next"), 2},
				{"p", T("hint_prev"), 2},
				{"</>", T("hint_seek"), 3},
				{"+/-", T("hint_vol"), 3},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel, 4})
			}
			hints = append(hints,
				statusHint{`\`, collapseHintLabel, 4},
				statusHint{"/", T("hint_search"), 5},
				statusHint{"?", T("hint_help"), 5},
			)
		case displayState == audio.StatePaused, a.currentTrack != nil:
			hints = []statusHint{
				{"Spc", T("hint_resume"), 1},
				{"q", T("hint_quit"), 1},
				{"n", T("hint_next"), 2},
				{"p", T("hint_prev"), 2},
				{"</>", T("hint_seek"), 3},
				{"+/-", T("hint_vol"), 3},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel, 4})
			}
			hints = append(hints,
				statusHint{`\`, collapseHintLabel, 4},
				statusHint{"/", T("hint_search"), 5},
				statusHint{"?", T("hint_help"), 5},
			)
		default:
			// No track — guide the user.
			hints = []statusHint{
				{"Enter", T("hint_play"), 1},
				{"q", T("hint_quit"), 1},
				{"/", T("hint_search"), 2},
				{`\`, collapseHintLabel, 3},
				{",", T("hint_settings"), 3},
				{"?", T("hint_help"), 4},
			}
		}
	}

	// Render a hint chip: [key] label
	renderHint := func(h statusHint) string {
		return styleStatusKey.Render(" "+h.key+" ") +
			styleStatusHintLabel.Render(" "+h.label+"  ")
	}

	// Compute the display width of a rendered hint (no ANSI codes in the
	// measurement — use raw strings to estimate; close enough for layout).
	hintWidth := func(h statusHint) int {
		// styleStatusKey wraps " key " → 1+len(key)+1
		// styleStatusHintLabel wraps " label  " → 1+len(label)+2
		// Both styles add no extra padding beyond the text.
		return 1 + strWidth(h.key) + 1 + 1 + strWidth(h.label) + 2
	}

	// Drop hints by priority (highest number first) until they fit.
	// Derive the initial cutoff from the hints themselves so no magic number
	// needs to stay in sync when priorities change.
	available := a.W - prefixW
	if available < 0 {
		available = 0
	}
	maxPriority := 0
	for _, h := range hints {
		if h.priority > maxPriority {
			maxPriority = h.priority
		}
	}
	cutoffPriority := maxPriority + 1
	for {
		total := 0
		for _, h := range hints {
			if h.priority < cutoffPriority {
				total += hintWidth(h)
			}
		}
		if total <= available || cutoffPriority <= 1 {
			break
		}
		cutoffPriority--
	}

	var sb strings.Builder
	for _, h := range hints {
		if h.priority < cutoffPriority {
			sb.WriteString(renderHint(h))
		}
	}

	line := prefix + sb.String()
	return styleStatusLine.Render(line)
}
