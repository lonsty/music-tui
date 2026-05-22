package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleKey is the top-level keyboard dispatcher.
func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Global quit — works in every state.
	if msg.String() == "ctrl+c" {
		return tea.Sequence(
			func() tea.Msg { a.player.Stop(); return noopMsg{} },
			tea.Quit,
		)
	}

	// Fullscreen view has its own minimal key set.
	if a.currentView == viewFullscreen {
		return a.handleFullscreenKey(msg)
	}

	// Overlay handlers.
	switch a.activeOvl {
	case overlayHelp:
		return a.handleHelpKey(msg)
	case overlayInfo:
		return a.handleInfoKey(msg)
	case overlaySearch:
		return a.handleSearchKey(msg)
	}

	return a.handleNormalKey(msg)
}

// ── Normal mode ───────────────────────────────────────────────────────────────

func (a *App) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return tea.Sequence(
			func() tea.Msg { a.player.Stop(); return noopMsg{} },
			tea.Quit,
		)

	// ── Navigation ──────────────────────────────────────────────────────────
	case "j", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
			a.syncRowMarquee()
		}

	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
			a.syncRowMarquee()
		}

	case "g":
		a.cursor = 0
		a.syncRowMarquee()

	case "G":
		if len(a.filtered) > 0 {
			a.cursor = len(a.filtered) - 1
			a.syncRowMarquee()
		}

	// ── Playback ─────────────────────────────────────────────────────────────
	case "enter":
		// Second Enter on the currently-playing track → open fullscreen.
		if a.currentTrack != nil &&
			a.cursor < len(a.filtered) &&
			a.filtered[a.cursor].ID == a.currentTrack.ID {
			a.currentView = viewFullscreen
			return nil
		}
		// Otherwise play the selected track.
		a.lastEnterID = ""
		if a.cursor < len(a.filtered) {
			a.lastEnterID = a.filtered[a.cursor].ID
		}
		return a.cmdPlayTrack(a.cursor)

	case " ":
		return a.cmdTogglePause()

	case "n":
		return a.cmdPlayNext()

	case "p":
		return a.cmdPlayPrev()

	// ── Play mode ─────────────────────────────────────────────────────────────
	case "m":
		return a.cmdNextPlayMode()

	// ── Bit-crush effect ──────────────────────────────────────────────────────
	case "b":
		return a.cmdRetroUp()
	case "B":
		return a.cmdRetroDown()

	// ── Volume ────────────────────────────────────────────────────────────────
	case "+", "=":
		a.volume = clampVolume(a.volume + 0.1)
		a.player.SetVolume(a.volume)

	case "-":
		a.volume = clampVolume(a.volume - 0.1)
		a.player.SetVolume(a.volume)

	// ── Views / overlays ──────────────────────────────────────────────────────
	case "f":
		a.currentView = viewFullscreen

	case "i":
		a.activeOvl = overlayInfo

	case "/":
		a.activeOvl = overlaySearch
		a.searchInput.Focus()
		a.searchInput.SetValue("")

	case "?":
		a.activeOvl = overlayHelp

	case "tab":
		a.activeTab = (a.activeTab + 1) % 2

	case "esc":
		a.activeOvl = overlayNone
	}

	return nil
}

// ── Fullscreen mode ───────────────────────────────────────────────────────────

func (a *App) handleFullscreenKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "f":
		a.currentView = viewNormal

	case " ":
		return a.cmdTogglePause()

	case "n":
		return a.cmdPlayNext()

	case "p":
		return a.cmdPlayPrev()

	case "m":
		return a.cmdNextPlayMode()

	case "b":
		return a.cmdRetroUp()
	case "B":
		return a.cmdRetroDown()

	case "+", "=":
		a.volume = clampVolume(a.volume + 0.1)
		a.player.SetVolume(a.volume)

	case "-":
		a.volume = clampVolume(a.volume - 0.1)
		a.player.SetVolume(a.volume)

	case "q":
		return tea.Sequence(
			func() tea.Msg { a.player.Stop(); return noopMsg{} },
			tea.Quit,
		)
	}
	return nil
}

// ── Search overlay ────────────────────────────────────────────────────────────

func (a *App) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		// Play cursor track and close search.
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		if len(a.filtered) > 0 {
			return a.cmdPlayTrack(a.cursor)
		}
		return nil

	case "esc":
		a.activeOvl = overlayNone
		a.searchInput.Blur()
		a.searchInput.SetValue("")
		a.applyFilter()
		return nil

	// Allow cursor navigation while search is open — use arrow keys only,
	// so j/k remain available as text input characters.
	case "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
			a.syncRowMarquee()
		}
		return nil

	case "up":
		if a.cursor > 0 {
			a.cursor--
			a.syncRowMarquee()
		}
		return nil
	}

	// Let textinput handle typing; update filter on every keystroke.
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	a.applyFilter()
	return cmd
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (a *App) handleHelpKey(_ tea.KeyMsg) tea.Cmd {
	a.activeOvl = overlayNone
	return nil
}

// ── Info overlay ──────────────────────────────────────────────────────────────

func (a *App) handleInfoKey(_ tea.KeyMsg) tea.Cmd {
	a.activeOvl = overlayNone
	return nil
}
