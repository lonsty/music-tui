package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/store"
)

// ── Settings overlay ──────────────────────────────────────────────────────────

func (a *App) handleSettingsKey(msg tea.KeyMsg) tea.Cmd {
	// ── Editing mode: text input is active ────────────────────────────────
	// In editing mode only control keys are handled; all other keystrokes are
	// forwarded to the active text input.  Esc exits editing without closing.
	if a.settingsEditing {
		switch msg.String() {
		case "esc":
			// Exit editing mode, return to browse mode.
			a.settingsEditing = false
			a.musicDirInput.Blur()
			a.settingsInput.Blur()
			return nil
		case "enter":
			// Commit the edited value and exit editing mode.
			return a.settingsCommitField()
		case "ctrl+r":
			return a.settingsReloadLibrary()
		default:
			// Forward to the active text input.
			var cmd tea.Cmd
			switch a.settingsActive {
			case settingsFieldMusicDir:
				a.musicDirInput, cmd = a.musicDirInput.Update(msg)
			case settingsFieldChipOpts:
				a.settingsInput, cmd = a.settingsInput.Update(msg)
			}
			return cmd
		}
	}

	// ── Browse mode: navigate fields with ↑/↓, activate with Enter ────────
	switch msg.String() {
	case "esc":
		a.closeSettings()
		return nil

	case "up", "k":
		a.settingsMovePrev()
		return nil

	case "down", "j":
		a.settingsMoveNext()
		return nil

	case "enter":
		return a.settingsActivateField()

	case "ctrl+r":
		return a.settingsReloadLibrary()
	}
	return nil
}

// settingsMoveNext moves the selection to the next field, wrapping around,
// and scrolls ovlScrollRow to keep the selected field visible.
func (a *App) settingsMoveNext() {
	rows := settingsFieldRows()
	a.settingsActive = (a.settingsActive + 1) % settingsFieldCount
	a.ovlScrollRow = rows[a.settingsActive]
}

// settingsMovePrev moves the selection to the previous field, wrapping around.
func (a *App) settingsMovePrev() {
	rows := settingsFieldRows()
	a.settingsActive = (a.settingsActive + settingsFieldCount - 1) % settingsFieldCount
	a.ovlScrollRow = rows[a.settingsActive]
}

// settingsActivateField handles Enter in browse mode.
// For text fields it enters editing mode; for toggle fields it cycles the value.
func (a *App) settingsActivateField() tea.Cmd {
	switch a.settingsActive {
	case settingsFieldMusicDir:
		a.settingsEditing = true
		a.musicDirInput.Focus()
		a.musicDirInput.CursorEnd()
		return nil

	case settingsFieldChipOpts:
		a.settingsEditing = true
		a.settingsInput.Focus()
		a.settingsInput.CursorEnd()
		return nil

	case settingsFieldLanguage:
		if ActiveLang() == LangEN {
			SetLang(LangZH)
		} else {
			SetLang(LangEN)
		}
		if a.st != nil {
			langVal := store.ValLanguageEN
			if ActiveLang() == LangZH {
				langVal = store.ValLanguageZH
			}
			a.saveSetting(store.KeyLanguage, langVal)
		}
		a.searchInput.Placeholder = T("search_placeholder")
		return nil

	case settingsFieldFormat:
		a.formatPref = (a.formatPref + 1) % formatPrefCount
		if a.st != nil {
			a.saveSetting(store.KeyFormatPreference, formatPrefKey(a.formatPref))
		}
		a.applyFilter()
		return nil

	case settingsFieldIconSet:
		next := (ActiveIconSet() + 1) % iconSetCount
		setIconSet(next)
		if a.st != nil {
			a.saveSetting(store.KeyIconSet, iconSetKey(next))
		}
		return nil
	}
	return nil
}

// settingsCommitField saves the current text-field value and exits editing mode.
func (a *App) settingsCommitField() tea.Cmd {
	a.settingsEditing = false
	a.musicDirInput.Blur()
	a.settingsInput.Blur()

	newDir := strings.TrimSpace(a.musicDirInput.Value())
	newOpts := strings.TrimSpace(a.settingsInput.Value())

	if a.settingsActive == settingsFieldMusicDir && newDir != a.musicDir && newDir != "" {
		if info, err := os.Stat(newDir); err != nil || !info.IsDir() {
			a.statusMsg = fmt.Sprintf(T("dir_not_found"), newDir)
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return nil
		}
		a.musicDir = newDir
		if a.st != nil {
			a.saveSetting(store.KeyMusicDir, newDir)
		}
	}

	if a.settingsActive == settingsFieldChipOpts && newOpts != a.chip8Options {
		a.chip8Options = newOpts
		if a.st != nil {
			a.saveSetting(store.KeyChip8Options, newOpts)
		}
		a.chipPath = ""
		a.chipOrigin = ""
	}
	return nil
}
