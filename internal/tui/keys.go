package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

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

// quitOrConfirm makes q require a second press within the status-message
// window, preventing an accidental single-key quit.
func (a *App) quitOrConfirm() tea.Cmd {
	now := time.Now()
	if !a.quitArmedAt.IsZero() && now.Sub(a.quitArmedAt) <= statusMsgDuration {
		a.quitArmedAt = time.Time{}
		return a.cmdQuit()
	}
	a.quitArmedAt = now
	a.statusMsg = T("quit_confirm")
	a.statusMsgExpiry = now.Add(statusMsgDuration)
	return nil
}

// openSettings initialises and opens the settings overlay.
func (a *App) openSettings() {
	a.activeOvl = overlaySettings
	a.ovlScrollRow = 0
	a.settingsActive = settingsFieldMusicDir
	a.settingsEditing = false
	a.musicDirInput.SetValue(a.musicDir)
	a.musicDirInput.Blur()
	a.settingsInput.SetValue(a.chip8Options)
	a.settingsInput.Blur()
}

// restoreLocalFilter re-applies the search filter so that the Local tab shows
// the full (unfiltered or search-filtered) library after returning from the
// Playlist tab, which may have replaced filteredIdxs with a playlist subset.
// The cursor is anchored to the currently playing track when possible.
func (a *App) restoreLocalFilter() {
	a.applyFilter()
	if a.currentTrack != nil {
		if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
			a.cursorPos = pos
		}
	}
	// Clamp cursor after returning from a different tab (e.g. Playlist)
	// where the track list may be shorter than the previous position.
	if n := a.filteredLen(); n > 0 {
		if a.cursorPos >= n {
			a.cursorPos = n - 1
		}
		if a.cursorPos < 0 {
			a.cursorPos = 0
		}
	} else {
		a.cursorPos = 0
	}
	a.syncRowMarquee()
}

// closeSettings dismisses the settings overlay and unfocuses inputs.
func (a *App) closeSettings() {
	a.activeOvl = overlayNone
	a.ovlScrollRow = 0
	a.settingsEditing = false
	a.musicDirInput.Blur()
	a.settingsInput.Blur()
}

// closeOverlay dismisses any non-settings overlay and resets scroll position.
func (a *App) closeOverlay() {
	a.activeOvl = overlayNone
	a.ovlScrollRow = 0
}

// scrollOverlay moves the overlay scroll position by delta rows, clamped to
// valid range.  Actual clamping against content length happens in the renderer.
func (a *App) scrollOverlay(delta int) {
	a.ovlScrollRow += delta
	if a.ovlScrollRow < 0 {
		a.ovlScrollRow = 0
	}
}

// handleKey is the top-level keyboard dispatcher.
func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	// Global quit — works in every state.
	if msg.String() == "ctrl+c" {
		return a.cmdQuit()
	}
	// Any non-q key disarms a pending quit confirmation.
	if msg.String() != "q" {
		a.quitArmedAt = time.Time{}
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
		return a.cmdAdvance()
	case "f6":
		return a.cmdPlayPrev()
	case "f11":
		a.adjustVolume(-volumeStep)
		return nil
	case "f12":
		a.adjustVolume(+volumeStep)
		return nil
	}

	// Global library rescan. Text inputs (settings editing, search, playlist
	// naming, add-tracks search) keep Ctrl+R for themselves or ignore it.
	if msg.String() == "ctrl+r" &&
		a.activeOvl == overlayNone &&
		!a.settingsEditing &&
		a.inputMode == playlistInputNone &&
		!a.addingTracks {
		return a.reloadLibrary()
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
	case overlayAddToPlaylist:
		return a.handleAddToPlaylistKey(msg)
	case overlayDeletePlaylist:
		return a.handleDeletePlaylistKey(msg)
	case overlayQueue:
		return a.handleQueueKey(msg)
	}

	// Playlist tab has its own key handler.
	if a.activeTab == tabPlaylist {
		return a.handlePlaylistKey(msg)
	}

	return a.handleNormalKey(msg)
}

// ── Normal mode ───────────────────────────────────────────────────────────────

func (a *App) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	if a.view != libraryViewTracks {
		return a.handleGroupKey(msg)
	}
	return a.handleSharedTrackListKey(a.localTrackListCtx(), msg)
}

// reloadLibrary starts an incremental scan of the current music directory.
// It is shared by the global Ctrl+R handler and the Settings overlay action.
func (a *App) reloadLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
		return nil
	}
	// A scan is already running: do not stack another one on top of it.
	if a.loading {
		return nil
	}
	a.loading = true
	a.statusMsg = T("reloading_library")
	a.statusMsgExpiry = time.Time{}
	a.libScanFromReload = true
	return a.cmdSyncLibrary()
}

// settingsReloadLibrary saves the current dir value and triggers a library scan.
func (a *App) settingsReloadLibrary() tea.Cmd {
	a.settingsEditing = false
	a.musicDirInput.Blur()
	a.settingsInput.Blur()

	a.musicDir = strings.TrimSpace(a.musicDirInput.Value())
	if a.musicDir == "" {
		a.musicDir = strings.TrimSpace(a.musicDirInput.Placeholder)
	}
	if info, err := os.Stat(a.musicDir); err != nil || !info.IsDir() {
		a.statusMsg = fmt.Sprintf(T("dir_not_found"), a.musicDir)
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		a.closeSettings()
		return nil
	}
	if a.st != nil {
		a.saveSetting(store.KeyMusicDir, a.musicDir)
	}
	a.closeSettings()
	return a.reloadLibrary()
}
