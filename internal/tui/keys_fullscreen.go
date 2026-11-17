package tui

import tea "github.com/charmbracelet/bubbletea"

// ── Fullscreen mode ───────────────────────────────────────────────────────────

// handleFullscreenKey is the test-facing entry point for the fullscreen view.
func (a *App) handleFullscreenKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeFullscreen, scopeGlobal}, msg, a.handleFullscreenAction)
}

func (a *App) handleFullscreenAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionBrowseUp:
		if a.synced && len(a.lines) > 0 {
			cur := a.browseCenterIdx
			if cur < 0 {
				cur = a.activeIdx
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
	case actionBrowseDown:
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
	case actionSeekLine:
		if a.synced && a.browseCenterIdx >= 0 {
			target := lyricSeekTarget(a.lines[a.browseCenterIdx], a.offset)
			a.browseCenterIdx = -1
			a.browseTicks = 0
			a.browseExpired = false
			return a.cmdSeekAndResume(target)
		}
	case actionPause:
		return a.cmdTogglePause()
	}
	return nil
}
