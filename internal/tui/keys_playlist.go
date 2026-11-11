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

// handlePlaylistKey handles keypresses when the Playlist tab is active.
func (a *App) handlePlaylistKey(msg tea.KeyMsg) tea.Cmd {
	// Global quit still works.
	if msg.String() == "q" {
		return a.quitOrConfirm()
	}

	// When name input is active, only Esc/Enter are control keys.
	if a.inputMode != playlistInputNone {
		switch msg.String() {
		case "esc":
			a.inputMode = playlistInputNone
			a.nameInput.SetValue("")
			a.nameInput.Blur()
		case "enter":
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
			// rename
			if pl := a.currentPlaylist(); pl.ID != "" {
				return a.cmdRenamePlaylist(pl.ID, name)
			}
		default:
			var cmd tea.Cmd
			a.nameInput, cmd = a.nameInput.Update(msg)
			return cmd
		}
		return nil
	}

	// Add-tracks mode: forward to dedicated handler.
	if a.addingTracks {
		return a.handleAddTracksKey(msg)
	}

	if a.depth == playlistDepthTracks {
		return a.handlePlaylistTracksKey(msg)
	}
	return a.handlePlaylistListKey(msg)
}

// handlePlaylistListKey handles keys at the playlist-list depth.
func (a *App) handlePlaylistListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		if a.listCursor < len(a.playlists)-1 {
			a.listCursor++
			a.syncPlaylistRowMarquee()
		}
	case "k", "up":
		if a.listCursor > 0 {
			a.listCursor--
			a.syncPlaylistRowMarquee()
		}
	case "g":
		a.listCursor = 0
		a.syncPlaylistRowMarquee()
	case "G":
		if len(a.playlists) > 0 {
			a.listCursor = len(a.playlists) - 1
			a.syncPlaylistRowMarquee()
		}
	case "enter":
		if len(a.playlists) == 0 {
			return nil
		}
		a.depth = playlistDepthTracks
		a.trackCursor = 0
		a.pendingTrackAnchor = true
		pl := a.currentPlaylist()
		return a.cmdLoadPlaylistTracks(pl.ID)
	case "n":
		// New playlist.
		a.inputMode = playlistInputCreate
		a.nameInput.SetValue("")
		a.nameInput.Placeholder = T("playlist_new_prompt")
		a.nameInput.Focus()
		return textinput.Blink
	case "r":
		// Rename selected playlist (not Favorites).
		if pl := a.currentPlaylist(); pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			a.inputMode = playlistInputRename
			a.nameInput.SetValue(pl.Name)
			a.nameInput.Placeholder = T("playlist_rename_prompt")
			a.nameInput.Focus()
			return textinput.Blink
		}
	case "d":
		// Delete selected playlist (not Favorites) — open confirm overlay.
		if pl := a.currentPlaylist(); pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			a.activeOvl = overlayDeletePlaylist
			a.ovlScrollRow = 0
		}
	case "tab":
		a.activeTab = (a.activeTab + 1) % tabCount
		if a.activeTab == tabLocal {
			a.restoreLocalFilter()
		}
	case "?":
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0
	// Pass through playback and panel keys.
	case " ":
		return a.cmdTogglePause()
	case "f":
		a.currentView = viewFullscreen
	case "l":
		// Toggle the right panel between player mode and lyrics mode.
		if a.showMiniPlayer() {
			if a.rightMode == rightPanelLyrics {
				a.rightMode = rightPanelPlayer
			} else {
				a.rightMode = rightPanelLyrics
			}
		}
	case `\`:
		a.rightCollapsed = !a.rightCollapsed
	case "L":
		if a.currentTrack != nil {
			return a.cmdToggleFavorite(*a.currentTrack)
		}
	case ",":
		a.openSettings()
	}
	return nil
}

// handlePlaylistTracksKey handles keys at the playlist-tracks depth.
// It delegates entirely to the shared track-list engine with the playlist
// context, so the Playlist track level inherits every Local-tab feature.
func (a *App) handlePlaylistTracksKey(msg tea.KeyMsg) tea.Cmd {
	return a.handleSharedTrackListKey(a.playlistTrackCtx(), msg)
}

// handleAddTracksKey handles keys in the inline add-tracks search panel.
func (a *App) handleAddTracksKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.addingTracks = false
		a.addInput.Blur()
		a.addInput.SetValue("")
		// Compute net add/remove counts from the delta map.
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
	case "enter":
		// Toggle: if already in playlist → remove; otherwise → add.
		if a.addCursor < 0 || a.addCursor >= len(a.addResults) {
			return nil
		}
		idx := a.addResults[a.addCursor]
		t := a.tracks[idx]
		pl := a.currentPlaylist()
		// Check if track is already in the playlist, accounting for
		// delta changes made during this add-tracks session.
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
		// Don't close the panel — allow continuous editing.
		// Toggle semantics: if this track was already toggled in this
		// session, cancel the previous operation (set delta back to 0).
		// Otherwise, toggle between add (+1) and remove (-1).
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
	case "j", "down":
		if a.addCursor < len(a.addResults)-1 {
			a.addCursor++
		}
	case "k", "up":
		if a.addCursor > 0 {
			a.addCursor--
		}
	default:
		var cmd tea.Cmd
		a.addInput, cmd = a.addInput.Update(msg)
		a.applyAddFilter()
		a.addCursor = 0
		return cmd
	}
	return nil
}

// handleAddToPlaylistKey handles keys in the overlayAddToPlaylist overlay.
func (a *App) handleAddToPlaylistKey(msg tea.KeyMsg) tea.Cmd {
	if a.overlayCreating {
		switch msg.String() {
		case "esc":
			a.overlayCreating = false
			a.nameInput.Blur()
			a.nameInput.SetValue("")
			return nil
		case "enter":
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
		default:
			var cmd tea.Cmd
			a.nameInput, cmd = a.nameInput.Update(msg)
			return cmd
		}
	}

	switch msg.String() {
	case "esc":
		a.closeOverlay()
	case "n":
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
	case "j", "down":
		if a.ovlPlCursor < len(a.ovlPlaylists)-1 {
			a.ovlPlCursor++
			a.keepOvlCursorVisible()
		}
	case "k", "up":
		if a.ovlPlCursor > 0 {
			a.ovlPlCursor--
			a.keepOvlCursorVisible()
		}
	case "enter":
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

// handleDeletePlaylistKey handles key presses inside the delete-confirm overlay.
// Enter confirms the deletion; any other key (Esc, …) cancels.
func (a *App) handleDeletePlaylistKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		a.activeOvl = overlayNone
		pl := a.currentPlaylist()
		if pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			return a.cmdDeletePlaylist(pl.ID)
		}
		return nil
	default:
		a.activeOvl = overlayNone
		return nil
	}
}
