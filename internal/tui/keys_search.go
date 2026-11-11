package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Search overlay ────────────────────────────────────────────────────────────

func (a *App) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		// Play cursor track and close search.
		// Reset lastQuery so the next time the same term is typed it still
		// jumps to row 0 (as the user expects a fresh search).
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		a.lastQuery = ""
		if a.filteredLen() > 0 {
			return a.cmdPlayTrack(a.cursorPos)
		}
		return nil

	case "esc":
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		a.searchInput.SetValue("")
		a.applyFilter()
		a.syncSearchLibraryView(false)
		// After restoring the full list, position the cursor on the currently
		// playing track so the user can see where they were.  visibleWindow
		// will centre the playing row in the visible area automatically.
		if a.currentTrack != nil {
			if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
				a.cursorPos = pos
			}
		}
		a.syncRowMarquee()
		return nil

	// Allow cursor navigation while search is open — arrow keys only.
	// j/k are intentionally left for text input.
	case "down":
		if a.cursorPos < a.filteredLen()-1 {
			a.cursorPos++
			a.syncRowMarquee()
		}
		return nil

	case "up":
		if a.cursorPos > 0 {
			a.cursorPos--
			a.syncRowMarquee()
		}
		return nil
	}

	// Let textinput handle typing; update filter on every keystroke.
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	a.syncSearchLibraryView(strings.TrimSpace(a.searchInput.Value()) != "")
	a.applyFilter()
	a.syncRowMarquee()
	return cmd
}
