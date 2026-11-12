package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/store"
)

// ── Playlist tab keys ─────────────────────────────────────────────────────────

// handlePlaylistKey is the test-facing entry point for the Playlist tab.
func (a *App) handlePlaylistKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction(a.activeScopes(), msg, a.dispatchAction)
}

func (a *App) handlePlaylistListKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopePlaylistList, scopeGlobal}, msg, a.handlePlaylistListAction)
}

func (a *App) handlePlaylistListAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionNavDown:
		if a.listCursor < len(a.playlists)-1 {
			a.listCursor++
			a.syncPlaylistRowMarquee()
		}
	case actionNavUp:
		if a.listCursor > 0 {
			a.listCursor--
			a.syncPlaylistRowMarquee()
		}
	case actionNavTop:
		a.listCursor = 0
		a.syncPlaylistRowMarquee()
	case actionNavBottom:
		if len(a.playlists) > 0 {
			a.listCursor = len(a.playlists) - 1
			a.syncPlaylistRowMarquee()
		}
	case actionOpenPlaylist:
		if len(a.playlists) == 0 {
			return nil
		}
		a.depth = playlistDepthTracks
		a.trackCursor = 0
		a.pendingTrackAnchor = true
		pl := a.currentPlaylist()
		return a.cmdLoadPlaylistTracks(pl.ID)
	case actionPlaylistNew:
		a.inputMode = playlistInputCreate
		a.nameInput.SetValue("")
		a.nameInput.Placeholder = T("playlist_new_prompt")
		a.nameInput.Focus()
		return textinput.Blink
	case actionPlaylistRename:
		if pl := a.currentPlaylist(); pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			a.inputMode = playlistInputRename
			a.nameInput.SetValue(pl.Name)
			a.nameInput.Placeholder = T("playlist_rename_prompt")
			a.nameInput.Focus()
			return textinput.Blink
		}
	case actionPlaylistDelete:
		if pl := a.currentPlaylist(); pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			a.activeOvl = overlayDeletePlaylist
			a.ovlScrollRow = 0
		}
	case actionPause:
		return a.cmdTogglePause()
	}
	return nil
}

// handlePlaylistTracksKey is the test-facing entry point for playlist tracks.
func (a *App) handlePlaylistTracksKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopePlaylistTracks, scopeTracklist, scopeGlobal}, msg,
		func(action ActionID, msg tea.KeyMsg) tea.Cmd {
			return a.handleSharedTrackListAction(a.playlistTrackCtx(), action, msg)
		})
}

// handleInputAction routes actions resolved in text-input scopes.
func (a *App) handleInputAction(action ActionID, msg tea.KeyMsg, top Scope) tea.Cmd {
	switch top {
	case scopeInputName:
		return a.handleNameInputAction(action, msg)
	case scopeInputAddTracks:
		return a.handleAddTracksAction(action, msg)
	case scopeInputSearch:
		return a.handleSearchAction(action, msg)
	case scopeOverlayAddCreate:
		return a.handleAddCreateAction(action, msg)
	case scopeOverlaySettingsEdit:
		return a.handleSettingsAction(action, msg, top)
	}
	return a.forwardToInput(msg, []Scope{top})
}

func (a *App) handleNameInputAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionCommitName:
		name := strings.TrimSpace(a.nameInput.Value())
		prevMode := a.inputMode
		a.inputMode = playlistInputNone
		a.nameInput.SetValue("")
		a.nameInput.Blur()
		if name == "" {
			return nil
		}
		if prevMode == playlistInputCreate {
			return a.cmdCreatePlaylist(name)
		}
		if pl := a.currentPlaylist(); pl.ID != "" {
			return a.cmdRenamePlaylist(pl.ID, name)
		}
	case actionCancel:
		a.inputMode = playlistInputNone
		a.nameInput.SetValue("")
		a.nameInput.Blur()
	}
	return nil
}

// handleAddTracksKey is the test-facing entry point for the add-tracks panel.
func (a *App) handleAddTracksKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeInputAddTracks, scopeGlobal}, msg, a.handleAddTracksAction)
}

func (a *App) handleAddTracksAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionCancel:
		a.addingTracks = false
		a.addInput.Blur()
		a.addInput.SetValue("")
		var addN, remN int
		for _, v := range a.addDelta {
			if v > 0 {
				addN++
			} else if v < 0 {
				remN++
			}
		}
		if addN > 0 || remN > 0 {
			a.statusMsg = fmt.Sprintf(T("playlist_add_remove_summary"), addN, remN)
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		a.addDelta = nil
		return nil
	case actionAddTracksToggle:
		if a.addCursor < 0 || a.addCursor >= len(a.addResults) {
			return nil
		}
		idx := a.addResults[a.addCursor]
		t := a.tracks[idx]
		pl := a.currentPlaylist()
		inPlaylist := false
		for _, pt := range a.plTracks {
			if pt.ID == t.ID {
				inPlaylist = true
				break
			}
		}
		if a.addDelta[t.ID] > 0 {
			inPlaylist = true
		} else if a.addDelta[t.ID] < 0 {
			inPlaylist = false
		}
		if a.addDelta[t.ID] > 0 {
			a.addDelta[t.ID] = 0
			return a.cmdRemoveTrackFromPlaylist(pl.ID, t.ID)
		}
		if a.addDelta[t.ID] < 0 {
			a.addDelta[t.ID] = 0
			return a.cmdAddTrackToPlaylist(pl.ID, t.ID)
		}
		if inPlaylist {
			a.addDelta[t.ID] = -1
			return a.cmdRemoveTrackFromPlaylist(pl.ID, t.ID)
		}
		a.addDelta[t.ID] = +1
		return a.cmdAddTrackToPlaylist(pl.ID, t.ID)
	case actionNavDown:
		if a.addCursor < len(a.addResults)-1 {
			a.addCursor++
		}
		return nil
	case actionNavUp:
		if a.addCursor > 0 {
			a.addCursor--
		}
		return nil
	}
	return a.forwardToInput(msg, []Scope{scopeInputAddTracks})
}

// handleAddToPlaylistKey is the test-facing entry point for the picker overlay.
func (a *App) handleAddToPlaylistKey(msg tea.KeyMsg) tea.Cmd {
	scopes := []Scope{scopeOverlayAddList, scopeGlobal}
	if a.overlayCreating {
		scopes = []Scope{scopeOverlayAddCreate, scopeGlobal}
	}
	return a.runAction(scopes, msg, func(action ActionID, msg tea.KeyMsg) tea.Cmd {
		return a.handleAddToPlaylistAction(action, msg, scopes[0])
	})
}

func (a *App) handleAddToPlaylistAction(action ActionID, msg tea.KeyMsg, top Scope) tea.Cmd {
	if top == scopeOverlayAddCreate {
		return a.handleAddCreateAction(action, msg)
	}
	switch action {
	case actionCancel:
		a.closeOverlay()
	case actionPlaylistNew:
		ct := a.cursorTrack()
		if ct == nil {
			return nil
		}
		a.overlayCreating = true
		a.ovlNewTrackID = ct.ID
		a.nameInput.Placeholder = T("playlist_new_prompt")
		a.nameInput.SetValue("")
		a.nameInput.Focus()
		return textinput.Blink
	case actionNavDown:
		if a.ovlPlCursor < len(a.ovlPlaylists)-1 {
			a.ovlPlCursor++
			a.keepOvlCursorVisible()
		}
	case actionNavUp:
		if a.ovlPlCursor > 0 {
			a.ovlPlCursor--
			a.keepOvlCursorVisible()
		}
	case actionConfirm:
		if a.ovlPlCursor < 0 || a.ovlPlCursor >= len(a.ovlPlaylists) {
			a.closeOverlay()
			return nil
		}
		pl := a.ovlPlaylists[a.ovlPlCursor]
		ct := a.cursorTrack()
		if ct == nil {
			a.closeOverlay()
			return nil
		}
		a.closeOverlay()
		a.statusMsg = fmt.Sprintf(T("playlist_added"), pl.Name)
		return a.cmdAddTrackToPlaylist(pl.ID, ct.ID)
	}
	return nil
}

func (a *App) handleAddCreateAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionCancel:
		a.overlayCreating = false
		a.nameInput.Blur()
		a.nameInput.SetValue("")
		return nil
	case actionCreateAndAdd:
		name := strings.TrimSpace(a.nameInput.Value())
		if name == "" {
			return nil
		}
		trackID := a.ovlNewTrackID
		a.overlayCreating = false
		a.ovlNewTrackID = ""
		a.nameInput.Blur()
		a.nameInput.SetValue("")
		a.closeOverlay()
		return a.cmdCreatePlaylistAndAdd(name, trackID)
	}
	return a.forwardToInput(msg, []Scope{scopeOverlayAddCreate})
}

// handleDeletePlaylistKey is the test-facing entry point for the confirm overlay.
func (a *App) handleDeletePlaylistKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeConfirm, scopeGlobal}, msg, a.handleDeletePlaylistAction)
}

func (a *App) handleDeletePlaylistAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionDeleteConfirm:
		a.activeOvl = overlayNone
		pl := a.currentPlaylist()
		if pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			return a.cmdDeletePlaylist(pl.ID)
		}
	case actionCancel:
		a.activeOvl = overlayNone
	}
	return nil
}
