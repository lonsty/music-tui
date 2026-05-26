package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/store"
)

// cmdQuit saves the session and stops playback before quitting.
// Used by every quit path to avoid code duplication.
func (a *App) cmdQuit() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { a.saveSession(); a.player.Stop(); return noopMsg{} },
		tea.Quit,
	)
}

// openSettings initialises and opens the settings overlay.
func (a *App) openSettings() {
	a.activeOvl = overlaySettings
	a.settingsActive = 0
	a.musicDirInput.SetValue(a.musicDir)
	a.musicDirInput.Focus()
	a.musicDirInput.CursorEnd()
	a.settingsInput.SetValue(a.chip8Options)
	a.settingsInput.Blur()
}

// closeSettings dismisses the settings overlay and unfocuses inputs.
func (a *App) closeSettings() {
	a.activeOvl = overlayNone
	a.musicDirInput.Blur()
	a.settingsInput.Blur()
}

// handleKey is the top-level keyboard dispatcher.
func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Global quit — works in every state.
	if msg.String() == "ctrl+c" {
		return a.cmdQuit()
	}

	// Media keys: handle globally regardless of view/overlay.
	// Terminals do not forward OS-level media keys directly; users can map
	// them to these F-key sequences via skhd (macOS) or xbindkeys (Linux).
	//
	//   F7  → Play / Pause      (maps to ⏯  or XF86AudioPlay)
	//   F8  → Stop              (maps to ⏹  or XF86AudioStop)
	//   F9  → Next track        (maps to ⏭  or XF86AudioNext)
	//   F6  → Previous track    (maps to ⏮  or XF86AudioPrev)
	//   F11 → Volume down       (maps to 🔉 or XF86AudioLowerVolume)
	//   F12 → Volume up         (maps to 🔊 or XF86AudioRaiseVolume)
	switch msg.String() {
	case "f7":
		return a.cmdTogglePause()
	case "f8":
		a.saveSession()
		a.player.Stop()
		a.currentTrack = nil
		return nil
	case "f9":
		return a.cmdPlayNext()
	case "f6":
		return a.cmdPlayPrev()
	case "f11":
		a.adjustVolume(-volumeStep)
		return nil
	case "f12":
		a.adjustVolume(+volumeStep)
		return nil
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
	case overlaySettings:
		return a.handleSettingsKey(msg)
	}

	return a.handleNormalKey(msg)
}

// ── Normal mode ───────────────────────────────────────────────────────────────

func (a *App) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return a.cmdQuit()

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

	// ── 8-bit chip mode ───────────────────────────────────────────────────────
	case "b":
		return a.cmdToggleChip()

	// ── Retro lo-fi effect (r/R) ──────────────────────────────────────────────
	case "r":
		return a.cmdRetroUp()
	case "R":
		return a.cmdRetroDown()

	// ── Volume ────────────────────────────────────────────────────────────────
	case "+", "=":
		a.adjustVolume(+volumeStep)

	case "-":
		a.adjustVolume(-volumeStep)

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "<":
		return a.cmdSeek(-seekStep)

	case ">":
		return a.cmdSeek(+seekStep)

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

	case ",":
		a.openSettings()

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
		return a.cmdToggleChip()

	case ",":
		a.openSettings()

	case "r":
		return a.cmdRetroUp()
	case "R":
		return a.cmdRetroDown()

	case "+", "=":
		a.adjustVolume(+volumeStep)

	case "-":
		a.adjustVolume(-volumeStep)

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "<":
		return a.cmdSeek(-seekStep)

	case ">":
		return a.cmdSeek(+seekStep)

	case "q":
		return a.cmdQuit()

		// normalOpenSettings placeholder
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

// ── Settings overlay ──────────────────────────────────────────────────────────

func (a *App) handleSettingsKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		// Save both fields and close.
		newDir := strings.TrimSpace(a.musicDirInput.Value())
		newOpts := strings.TrimSpace(a.settingsInput.Value())

		dirChanged := newDir != a.musicDir && newDir != ""
		optsChanged := newOpts != a.chip8Options

		if dirChanged {
			// Validate the directory exists before saving.
			if info, err := os.Stat(newDir); err != nil || !info.IsDir() {
				a.statusMsg = "󰅚  Directory not found: " + newDir
				a.closeSettings()
				return nil
			}
			a.musicDir = newDir
			if a.st != nil {
				_ = a.st.SetSetting(store.KeyMusicDir, newDir)
			}
		}
		if optsChanged {
			a.chip8Options = newOpts
			if a.st != nil {
				_ = a.st.SetSetting(store.KeyChip8Options, newOpts)
			}
			// Invalidate the chip cache so new options apply next time.
			a.chipPath = ""
			a.chipOrigin = ""
		}

		a.closeSettings()
		return nil

	case "esc":
		// Discard and close.
		a.closeSettings()
		return nil

	case "tab", "shift+tab":
		// Toggle active input field.
		if a.settingsActive == 0 {
			a.settingsActive = 1
			a.musicDirInput.Blur()
			a.settingsInput.Focus()
		} else {
			a.settingsActive = 0
			a.settingsInput.Blur()
			a.musicDirInput.Focus()
		}
		return nil

	case "ctrl+r":
		// Reload library (runs in background, closes overlay).
		a.musicDir = strings.TrimSpace(a.musicDirInput.Value())
		if a.musicDir == "" {
			a.musicDir = strings.TrimSpace(a.musicDirInput.Placeholder)
		}
		// Validate the directory exists before saving and reloading.
		if info, err := os.Stat(a.musicDir); err != nil || !info.IsDir() {
			a.statusMsg = "󰅚  Directory not found: " + a.musicDir
			a.closeSettings()
			return nil
		}
		if a.st != nil {
			_ = a.st.SetSetting(store.KeyMusicDir, a.musicDir)
		}
		a.closeSettings()
		a.loading = true
		a.statusMsg = "Reloading library…"
		return a.cmdSyncLibrary()
	}

	// Forward typing to the active input.
	var cmd tea.Cmd
	if a.settingsActive == 0 {
		a.musicDirInput, cmd = a.musicDirInput.Update(msg)
	} else {
		a.settingsInput, cmd = a.settingsInput.Update(msg)
	}
	return cmd
}
