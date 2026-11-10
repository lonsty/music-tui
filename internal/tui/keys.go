package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
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
	}

	// Playlist tab has its own key handler.
	if a.activeTab == tabPlaylist {
		return a.handlePlaylistKey(msg)
	}

	return a.handleNormalKey(msg)
}

// ── Normal mode ───────────────────────────────────────────────────────────────

func (a *App) handleNormalKey(msg tea.KeyMsg) tea.Cmd {
	return a.handleSharedTrackListKey(a.localTrackListCtx(), msg)
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

	case "r":
		return a.cmdRetroUp()
	case "R":
		return a.cmdRetroDown()

	case "+", "=":
		a.adjustVolume(+volumeStep)

	case "-":
		a.adjustVolume(-volumeStep)

	// ── Help ──────────────────────────────────────────────────────────────────
	case "?":
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0

	// ── Seek ──────────────────────────────────────────────────────────────────
	case "<":
		return a.cmdSeek(-seekStep)

	case ">":
		return a.cmdSeek(+seekStep)

	// ── Lyrics browse (synced lyrics only) ────────────────────────────────────
	// ↑/k scroll the visible window up (browse cursor moves toward earlier lines).
	// ↓/j scroll down (browse cursor moves toward later lines).
	// The cursor is pinned to an absolute line index so playback scrolling
	// does not move it; the cursor stays fixed until Enter or auto-reset.
	case "up", "k":
		if a.synced && len(a.lines) > 0 {
			cur := a.browseCenterIdx
			if cur < 0 {
				cur = a.activeIdx // start from current playing line
				if cur < 0 {
					cur = 0
				}
			}
			cur--
			if cur < 0 {
				cur = 0
			}
			a.browseCenterIdx = cur
			a.browseTicks = 0
			a.browseExpired = false
		}

	case "down", "j":
		if a.synced && len(a.lines) > 0 {
			cur := a.browseCenterIdx
			if cur < 0 {
				cur = a.activeIdx
				if cur < 0 {
					cur = 0
				}
			}
			cur++
			if cur >= len(a.lines) {
				cur = len(a.lines) - 1
			}
			a.browseCenterIdx = cur
			a.browseTicks = 0
			a.browseExpired = false
		}

	// Enter in browse mode: seek to the pinned line and resume playback.
	case "enter":
		if a.synced && a.browseCenterIdx >= 0 {
			target := a.lines[a.browseCenterIdx].Time
			a.browseCenterIdx = -1
			a.browseTicks = 0
			a.browseExpired = false
			return a.cmdSeekAndResume(target)
		}

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
	a.applyFilter()
	a.syncRowMarquee()
	return cmd
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (a *App) handleHelpKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "?", "esc":
		a.closeOverlay()
	case "up", "k":
		a.scrollOverlay(-1)
	case "down", "j":
		a.scrollOverlay(1)
	}
	return nil
}

// ── Info overlay ──────────────────────────────────────────────────────────────

func (a *App) handleInfoKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "i", "esc":
		a.closeOverlay()
	case "up", "k":
		a.scrollOverlay(-1)
	case "down", "j":
		a.scrollOverlay(1)
	}
	return nil
}

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
	a.settingsActive = (a.settingsActive + 1) % settingsFieldCount
	a.ovlScrollRow = settingsFieldBodyRow[a.settingsActive]
}

// settingsMovePrev moves the selection to the previous field, wrapping around.
func (a *App) settingsMovePrev() {
	a.settingsActive = (a.settingsActive + settingsFieldCount - 1) % settingsFieldCount
	a.ovlScrollRow = settingsFieldBodyRow[a.settingsActive]
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
		if activeLang == LangEN {
			SetLang(LangZH)
		} else {
			SetLang(LangEN)
		}
		if a.st != nil {
			langVal := store.ValLanguageEN
			if activeLang == LangZH {
				langVal = store.ValLanguageZH
			}
			_ = a.st.SetSetting(store.KeyLanguage, langVal)
		}
		a.searchInput.Placeholder = T("search_placeholder")
		return nil

	case settingsFieldFormat:
		a.formatPref = (a.formatPref + 1) % formatPrefCount
		if a.st != nil {
			_ = a.st.SetSetting(store.KeyFormatPreference, formatPrefKey(a.formatPref))
		}
		a.applyFilter()
		return nil

	case settingsFieldIconSet:
		next := (ActiveIconSet() + 1) % iconSetCount
		setIconSet(next)
		if a.st != nil {
			_ = a.st.SetSetting(store.KeyIconSet, iconSetKey(next))
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
			_ = a.st.SetSetting(store.KeyMusicDir, newDir)
		}
	}

	if a.settingsActive == settingsFieldChipOpts && newOpts != a.chip8Options {
		a.chip8Options = newOpts
		if a.st != nil {
			_ = a.st.SetSetting(store.KeyChip8Options, newOpts)
		}
		a.chipPath = ""
		a.chipOrigin = ""
	}
	return nil
}

// ── Playlist tab keys ─────────────────────────────────────────────────────────

// handlePlaylistKey handles keypresses when the Playlist tab is active.
func (a *App) handlePlaylistKey(msg tea.KeyMsg) tea.Cmd {
	// Global quit still works.
	if msg.String() == "q" {
		return a.cmdQuit()
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
	switch msg.String() {
	case "esc":
		a.closeOverlay()
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

// reloadLibrary starts an incremental scan of the current music directory.
// It is shared by the global Ctrl+R handler and the Settings overlay action.
func (a *App) reloadLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
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
		_ = a.st.SetSetting(store.KeyMusicDir, a.musicDir)
	}
	a.closeSettings()
	return a.reloadLibrary()
}
