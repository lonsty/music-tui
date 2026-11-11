package tui

import tea "github.com/charmbracelet/bubbletea"

// ── Fullscreen mode ───────────────────────────────────────────────────────────

func (a *App) handleFullscreenKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "f":
		a.currentView = viewNormal

	case " ":
		return a.cmdTogglePause()

	case "n":
		return a.cmdAdvance()

	case "p":
		return a.cmdPlayPrev()

	case "m":
		return a.cmdNextPlayMode()

	case "b":
		return a.cmdToggleChip()

	case "r":
		return a.cmdRetroUp()
	case "R":
		return a.cmdRetroDown()

	case "+", "=":
		a.adjustVolume(+volumeStep)

	case "-":
		a.adjustVolume(-volumeStep)

	// ── Help ──────────────────────────────────────────────────────────────────
	case "?":
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "<":
		return a.cmdSeek(-seekStep)

	case ">":
		return a.cmdSeek(+seekStep)

	// ── Lyrics timing offset ──────────────────────────────────────────────────
	case "[":
		a.adjustLyricsOffset(-lyricsOffsetStep)

	case "]":
		a.adjustLyricsOffset(+lyricsOffsetStep)

	// ── Lyrics browse (synced lyrics only) ────────────────────────────────────
	// ↑/k scroll the visible window up (browse cursor moves toward earlier lines).
	// ↓/j scroll down (browse cursor moves toward later lines).
	// The cursor is pinned to an absolute line index so playback scrolling
	// does not move it; the cursor stays fixed until Enter or auto-reset.
	case "up", "k":
		if a.synced && len(a.lines) > 0 {
			cur := a.browseCenterIdx
			if cur < 0 {
				cur = a.activeIdx // start from current playing line
				if cur < 0 {
					cur = 0
				}
			}
			cur--
			if cur < 0 {
				cur = 0
			}
			a.browseCenterIdx = cur
			a.browseTicks = 0
			a.browseExpired = false
		}

	case "down", "j":
		if a.synced && len(a.lines) > 0 {
			cur := a.browseCenterIdx
			if cur < 0 {
				cur = a.activeIdx
				if cur < 0 {
					cur = 0
				}
			}
			cur++
			if cur >= len(a.lines) {
				cur = len(a.lines) - 1
			}
			a.browseCenterIdx = cur
			a.browseTicks = 0
			a.browseExpired = false
		}

	// Enter in browse mode: seek to the pinned line and resume playback.
	case "enter":
		if a.synced && a.browseCenterIdx >= 0 {
			target := lyricSeekTarget(a.lines[a.browseCenterIdx], a.offset)
			a.browseCenterIdx = -1
			a.browseTicks = 0
			a.browseExpired = false
			return a.cmdSeekAndResume(target)
		}

	case "q":
		return a.quitOrConfirm()

		// normalOpenSettings placeholder
	}
	return nil
}
