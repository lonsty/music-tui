package tui

import (
	"fmt"
	"strings"

	"github.com/lonsty/music-tui/internal/audio"
)

// ── Status bar ────────────────────────────────────────────────────────────────

// stateLabel returns the localised chip-label text for the given playback state,
// including surrounding spaces for visual padding inside the status chip.
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

// currentHintScope returns the most specific scope with a T1 template.
func (a *App) currentHintScope() Scope {
	scopes := a.activeScopes()
	if len(scopes) > 0 && (isInputScope(scopes[0]) || isOverlayScope(scopes[0])) {
		return ""
	}
	for _, s := range scopes {
		if _, ok := shortSlots[s]; ok {
			return s
		}
	}
	return ""
}

// renderStatusBar renders the one-line status bar at the bottom of the screen.
//
// Layout: [state] [mode] [retro] [chip]  hint1 hint2 … hintN
//
// Hints come from the key registry's six-slot template. Slots 0/1/5 are pinned;
// narrow terminals drop slots 4, 3, 2 in that order.
func (a *App) renderStatusBar() string {
	if a.loading {
		return styleStatusLine.Render("  " + iconSpinner() + "  " + T("scanning_library"))
	}
	if a.scanErr != nil {
		return styleStatusLine.Render("  " + iconError() + "  " + fmt.Sprintf(T("sync_error"), a.scanErr.Error()))
	}

	state := a.framePlayback.state

	// During a track change the player briefly passes through StateStopped.
	// Treat it as Playing so the chip does not flash.
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

	// Dynamic statusMsg width: truncate so at least two hint chips remain.
	var msgChip string
	if a.statusMsg != "" {
		msgText := a.statusMsg
		const minHintW = 24
		basePrefix := " " + stateChipStr + retroChip + chipChip + queueChip + " "
		maxMsgW := a.W - strWidth(basePrefix) - minHintW - 2
		if maxMsgW < 8 {
			maxMsgW = 8
		}
		if strWidth(msgText) > maxMsgW {
			msgText = truncate(msgText, maxMsgW-1) + "…"
		}
		msgChip = " " + styleStatusState.Render(" "+msgText+" ")
	}
	prefix := " " + stateChipStr + retroChip + chipChip + queueChip + msgChip + "  "
	prefixW := strWidth(prefix)

	hints := a.shortHints(a.currentHintScope())
	available := a.W - prefixW
	if available < 0 {
		available = 0
	}
	hints = dropShortHints(hints, hintWidth, available)

	var sb strings.Builder
	for _, h := range hints {
		sb.WriteString(styleStatusKey.Render(" " + h.key + " ") +
			styleStatusHintLabel.Render(" " + h.label + " "))
	}

	line := prefix + sb.String()
	return styleStatusLine.Render(line)
}
