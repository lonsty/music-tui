package tui

import tea "github.com/charmbracelet/bubbletea"

// ── Search overlay ────────────────────────────────────────────────────────────

// handleSearchKey is the test-facing entry point for the inline search input.
func (a *App) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeInputSearch, scopeGlobal}, msg, a.handleSearchAction)
}

func (a *App) handleSearchAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionSearchPlay:
		// Play cursor track and close search. Reset lastQuery so typing the
		// same term again starts at row 0.
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		a.lastQuery = ""
		if a.filteredLen() > 0 {
			return a.cmdPlayTrack(a.cursorPos)
		}
		return nil
	case actionSearchCancel:
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		a.searchInput.SetValue("")
		a.applyFilter()
		a.syncSearchLibraryView(false)
		if a.currentTrack != nil {
			if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
				a.cursorPos = pos
			}
		}
		a.syncRowMarquee()
		return nil
	case actionNavDown:
		if a.cursorPos < a.filteredLen()-1 {
			a.cursorPos++
			a.syncRowMarquee()
		}
		return nil
	case actionNavUp:
		if a.cursorPos > 0 {
			a.cursorPos--
			a.syncRowMarquee()
		}
		return nil
	}
	return a.forwardToInput(msg, []Scope{scopeInputSearch})
}
