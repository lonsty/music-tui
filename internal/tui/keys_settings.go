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

// handleSettingsKey is the test-facing entry point for the settings overlay.
func (a *App) handleSettingsKey(msg tea.KeyMsg) tea.Cmd {
	scopes := []Scope{scopeOverlaySettingsBrowse, scopeGlobal}
	if a.settingsEditing {
		scopes = []Scope{scopeOverlaySettingsEdit, scopeGlobal}
	}
	return a.runAction(scopes, msg, func(action ActionID, msg tea.KeyMsg) tea.Cmd {
		return a.handleSettingsAction(action, msg, scopes[0])
	})
}

func (a *App) handleSettingsAction(action ActionID, msg tea.KeyMsg, top Scope) tea.Cmd {
	if top == scopeOverlaySettingsEdit {
		switch action {
		case actionSettingsCancel:
			a.settingsEditing = false
			a.musicDirInput.Blur()
			a.settingsInput.Blur()
			return nil
		case actionSettingsCommit:
			return a.settingsCommitField()
		}
		return a.forwardToInput(msg, []Scope{top})
	}

	switch action {
	case actionSettingsClose, actionCancel:
		a.closeSettings()
		return nil
	case actionSettingsScrollUp:
		a.settingsMovePrev()
		return nil
	case actionSettingsScrollDown:
		a.settingsMoveNext()
		return nil
	case actionSettingsEdit:
		return a.settingsActivateField()
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

// settingsActivateField handles the edit action in browse mode.
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
