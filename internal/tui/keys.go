package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

// cmdQuit saves the session and stops playback before quitting.
func (a *App) cmdQuit() tea.Cmd {
	return tea.Sequence(
		func() tea.Msg { a.saveSession(); a.player.Stop(); return noopMsg{} },
		tea.Quit,
	)
}

// quitOrConfirm requires a second quit action within the status-message window.
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

// restoreLocalFilter re-applies the search filter so the Local tab shows the
// full library after returning from the Playlist tab.
func (a *App) restoreLocalFilter() {
	a.applyFilter()
	if a.currentTrack != nil {
		if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
			a.cursorPos = pos
		}
	}
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

func (a *App) closeSettings() {
	a.activeOvl = overlayNone
	a.ovlScrollRow = 0
	a.settingsEditing = false
	a.musicDirInput.Blur()
	a.settingsInput.Blur()
}

func (a *App) closeOverlay() {
	a.activeOvl = overlayNone
	a.ovlScrollRow = 0
}

func (a *App) scrollOverlay(delta int) {
	a.ovlScrollRow += delta
	if a.ovlScrollRow < 0 {
		a.ovlScrollRow = 0
	}
}

func (a *App) trackByID(id string) *library.Track {
	if pos, ok := a.trackPosByID[id]; ok && pos >= 0 && pos < len(a.tracks) {
		t := a.tracks[pos]
		return &t
	}
	for i := range a.tracks {
		if a.tracks[i].ID == id {
			t := a.tracks[i]
			return &t
		}
	}
	return nil
}

// likeTarget returns the track the like action applies to. Grouped views use
// the first track of the selected group; track lists use the cursor row.
func (a *App) likeTarget() *library.Track {
	if a.activeTab == tabLocal && a.view != libraryViewTracks && a.currentView == viewNormal {
		groups := a.activeGroups()
		if a.groupCursor >= 0 && a.groupCursor < len(groups) {
			ids := groups[a.groupCursor].IDs
			if len(ids) > 0 {
				return a.trackByID(ids[0])
			}
		}
		return nil
	}
	ctx := a.activeTrackListCtx()
	if ctx.LikeTarget != nil {
		return ctx.LikeTarget(a)
	}
	return nil
}

func (a *App) activeTrackListCtx() TrackListContext {
	if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
		return a.playlistTrackCtx()
	}
	return a.localTrackListCtx()
}

// runAction resolves a key against the given scopes and runs the action.
// Shared global actions are handled before the scope-specific handler.
func (a *App) runAction(scopes []Scope, msg tea.KeyMsg, handler func(ActionID, tea.KeyMsg) tea.Cmd) tea.Cmd {
	action, ok := registry.Resolve(scopes, msg, a)
	if !ok {
		return a.forwardToInput(msg, scopes)
	}
	if action != actionQuit {
		a.quitArmedAt = time.Time{}
	}
	if cmd, handled := a.globalAction(action, msg); handled {
		return cmd
	}
	return handler(action, msg)
}

// handleKey resolves a key through the registry, then runs the action.
func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction(a.activeScopes(), msg, a.dispatchAction)
}

// globalAction handles actions whose behaviour is identical in every scope.
func (a *App) globalAction(action ActionID, msg tea.KeyMsg) (tea.Cmd, bool) {
	switch action {
	case actionForceQuit:
		return a.cmdQuit(), true
	case actionQuit:
		return a.quitOrConfirm(), true
	case actionHelp:
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0
		return nil, true
	case actionReload:
		if a.activeOvl == overlaySettings {
			return a.settingsReloadLibrary(), true
		}
		return a.reloadLibrary(), true
	case actionPause:
		return a.cmdTogglePause(), true
	case actionNext:
		return a.cmdAdvance(), true
	case actionPrev:
		return a.cmdPlayPrev(), true
	case actionSeekBack:
		return a.cmdSeek(-seekStep), true
	case actionSeekForward:
		return a.cmdSeek(+seekStep), true
	case actionVolumeUp:
		a.adjustVolume(+volumeStep)
		return nil, true
	case actionVolumeDown:
		a.adjustVolume(-volumeStep)
		return nil, true
	case actionMode:
		return a.cmdNextPlayMode(), true
	case actionChip:
		return a.cmdToggleChip(), true
	case actionLofiDown:
		return a.cmdRetroUp(), true
	case actionLofiUp:
		return a.cmdRetroDown(), true
	case actionOffsetEarlier:
		a.adjustLyricsOffset(-lyricsOffsetStep)
		return nil, true
	case actionOffsetLater:
		a.adjustLyricsOffset(+lyricsOffsetStep)
		return nil, true
	case actionMediaPlay:
		return a.cmdTogglePause(), true
	case actionMediaStop:
		a.saveSession()
		a.player.Stop()
		a.currentTrack = nil
		return nil, true
	case actionMediaNext:
		return a.cmdAdvance(), true
	case actionMediaPrev:
		return a.cmdPlayPrev(), true
	case actionMediaVolDown:
		a.adjustVolume(-volumeStep)
		return nil, true
	case actionMediaVolUp:
		a.adjustVolume(+volumeStep)
		return nil, true
	case actionFullscreen:
		if a.currentView == viewFullscreen {
			a.currentView = viewNormal
		} else {
			a.currentView = viewFullscreen
		}
		return nil, true
	case actionRightPanel:
		if a.showMiniPlayer() {
			if a.rightMode == rightPanelLyrics {
				a.rightMode = rightPanelPlayer
			} else {
				a.rightMode = rightPanelLyrics
			}
		}
		return nil, true
	case actionCollapse:
		a.rightCollapsed = !a.rightCollapsed
		return nil, true
	case actionSwitchTab:
		return a.switchTab(), true
	case actionQueueOpen:
		a.activeOvl = overlayQueue
		a.cursor = 0
		a.ovlScrollRow = 0
		return nil, true
	case actionSettingsOpen:
		a.openSettings()
		return nil, true
	case actionTrackInfo:
		a.activeOvl = overlayInfo
		a.ovlScrollRow = 0
		return nil, true
	case actionSearch:
		a.activeOvl = overlaySearch
		a.searchInput.Focus()
		a.searchInput.CursorEnd()
		return nil, true
	case actionLike:
		if target := a.likeTarget(); target != nil {
			return a.cmdToggleFavorite(*target), true
		}
		return nil, true
	case actionViewTracks:
		return a.setLibraryView(libraryViewTracks), true
	case actionViewAlbums:
		return a.setLibraryView(libraryViewAlbums), true
	case actionViewArtists:
		return a.setLibraryView(libraryViewArtists), true
	}
	return nil, false
}

func (a *App) setLibraryView(v libraryView) tea.Cmd {
	if a.activeTab != tabLocal || a.currentView != viewNormal {
		return nil
	}
	if strings.TrimSpace(a.searchInput.Value()) != "" {
		a.statusMsg = T("clear_search_first")
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		return nil
	}
	if a.drilldown != nil {
		a.drilldown = nil
		a.applyFilter()
	}
	if a.view == v {
		return nil
	}
	a.view = v
	a.prevView = libraryViewTracks
	a.groupCursor = 0
	a.rebuildGroups()
	a.syncRowMarquee()
	return nil
}

func (a *App) switchTab() tea.Cmd {
	a.activeTab = (a.activeTab + 1) % tabCount
	switch a.activeTab {
	case tabLocal:
		a.restoreLocalFilter()
	case tabPlaylist:
		if len(a.playlists) == 0 {
			return a.cmdLoadPlaylists()
		}
		a.syncPlaylistRowMarquee()
		if a.depth == playlistDepthTracks {
			a.anchorPlaylistTrackCursor()
			a.syncPlaylistTrackMarquee()
			if pl := a.currentPlaylist(); pl.ID != "" {
				return a.cmdLoadPlaylistTracks(pl.ID)
			}
		}
		return a.cmdLoadPlaylists()
	}
	return nil
}

// dispatchAction routes a resolved action to the scope-specific handler.
func (a *App) dispatchAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	scopes := a.activeScopes()
	top := scopes[0]
	switch {
	case isInputScope(top):
		return a.handleInputAction(action, msg, top)
	case top == scopeConfirm:
		return a.handleDeletePlaylistAction(action, msg)
	case top == scopeOverlayQueue:
		return a.handleQueueAction(action, msg)
	case top == scopeOverlayHelp, top == scopeOverlayInfo:
		return a.handleOverlayScrollAction(action, msg)
	case top == scopeOverlayAddList, top == scopeOverlayAddCreate:
		return a.handleAddToPlaylistAction(action, msg, top)
	case top == scopeOverlaySettingsBrowse, top == scopeOverlaySettingsEdit:
		return a.handleSettingsAction(action, msg, top)
	case top == scopeFullscreen:
		return a.handleFullscreenAction(action, msg)
	case top == scopeGroup:
		return a.handleGroupAction(action, msg)
	case top == scopePlaylistList:
		return a.handlePlaylistListAction(action, msg)
	default:
		return a.handleSharedTrackListAction(a.activeTrackListCtx(), action, msg)
	}
}

// forwardToInput sends unbound keys to the active text input. Input scopes are
// modal: only their own bindings are resolved before this point.
func (a *App) forwardToInput(msg tea.KeyMsg, scopes []Scope) tea.Cmd {
	if len(scopes) == 0 || !isInputScope(scopes[0]) {
		return nil
	}
	var cmd tea.Cmd
	switch scopes[0] {
	case scopeInputSearch:
		a.searchInput, cmd = a.searchInput.Update(msg)
		a.syncSearchLibraryView(strings.TrimSpace(a.searchInput.Value()) != "")
		a.applyFilter()
		a.syncRowMarquee()
	case scopeInputAddTracks:
		a.addInput, cmd = a.addInput.Update(msg)
		a.applyAddFilter()
		a.addCursor = 0
	case scopeInputName, scopeOverlayAddCreate:
		a.nameInput, cmd = a.nameInput.Update(msg)
	case scopeOverlaySettingsEdit:
		switch a.settingsActive {
		case settingsFieldMusicDir:
			a.musicDirInput, cmd = a.musicDirInput.Update(msg)
		case settingsFieldChipOpts:
			a.settingsInput, cmd = a.settingsInput.Update(msg)
		}
	}
	return cmd
}

// reloadLibrary starts an incremental scan of the current music directory.
func (a *App) reloadLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
		return nil
	}
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
		return nil
	}
	if a.st != nil {
		a.saveSetting(store.KeyMusicDir, a.musicDir)
	}
	// The settings overlay stays open while the incremental scan runs.
	return a.reloadLibrary()
}
