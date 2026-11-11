package tui

import tea "github.com/charmbracelet/bubbletea"

// ── Help overlay ──────────────────────────────────────────────────────────────

func (a *App) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "?", "esc":
		a.closeOverlay()
	case "up", "k":
		a.scrollOverlay(-1)
	case "down", "j":
		a.scrollOverlay(1)
	}
	return nil
}

// ── Info overlay ──────────────────────────────────────────────────────────────

func (a *App) handleInfoKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "i", "esc":
		a.closeOverlay()
	case "up", "k":
		a.scrollOverlay(-1)
	case "down", "j":
		a.scrollOverlay(1)
	}
	return nil
}
