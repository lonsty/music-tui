package tui

import tea "github.com/charmbracelet/bubbletea"

// ── Help / info overlays ──────────────────────────────────────────────────────

func (a *App) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeOverlayHelp, scopeGlobal}, msg, a.handleOverlayScrollAction)
}

func (a *App) handleInfoKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeOverlayInfo, scopeGlobal}, msg, a.handleOverlayScrollAction)
}

// handleOverlayScrollAction handles scrollable overlays (help, track info).
func (a *App) handleOverlayScrollAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionCancel:
		a.closeOverlay()
	case actionNavDown, actionSettingsScrollDown:
		a.scrollOverlay(1)
	case actionNavUp, actionSettingsScrollUp:
		a.scrollOverlay(-1)
	}
	return nil
}
