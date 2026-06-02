package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/lonsty/music-tui/internal/library"
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
	}

	// Playlist tab has its own key handler.
	if a.activeTab == tabPlaylist {
		return a.handlePlaylistKey(msg)
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
		if a.cursorPos < a.filteredLen()-1 {
			a.cursorPos++
			a.syncRowMarquee()
		}

	case "k", "up":
		if a.cursorPos > 0 {
			a.cursorPos--
			a.syncRowMarquee()
		}

	case "g":
		a.cursorPos = 0
		a.syncRowMarquee()

	case "G":
		if a.filteredLen() > 0 {
			a.cursorPos = a.filteredLen() - 1
			a.syncRowMarquee()
		}

	// ── Playback ─────────────────────────────────────────────────────────────
	case "enter":
		// Second Enter on the currently-playing track → open fullscreen.
		if a.currentTrack != nil {
			if ct := a.cursorTrack(); ct != nil && ct.ID == a.currentTrack.ID {
				a.currentView = viewFullscreen
				return nil
			}
		}
		// Otherwise play the selected track.
		a.lastEnterID = ""
		if ct := a.cursorTrack(); ct != nil {
			a.lastEnterID = ct.ID
		}
		return a.cmdPlayTrack(a.cursorPos)

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

	case "l":
		// Toggle the right panel between player mode and lyrics mode.
		// Only has effect when the mini-player panel is visible.
		if a.showMiniPlayer() {
			if a.rightMode == rightPanelLyrics {
				a.rightMode = rightPanelPlayer
			} else {
				a.rightMode = rightPanelLyrics
			}
		}

	case `\`:
		// Collapse or expand the right player panel.
		a.rightCollapsed = !a.rightCollapsed

	case "i":
		a.activeOvl = overlayInfo
		a.ovlScrollRow = 0

	case "/":
		a.activeOvl = overlaySearch
		a.searchInput.Focus()
		a.searchInput.SetValue("")
		// Apply the empty query immediately so filteredIdxs and lastQuery are
		// consistent with the cleared input when search opens.  The list will
		// show the full (unfiltered) results until the user starts typing.
		a.applyFilter()
		a.syncRowMarquee()

	case "?":
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0

	case ",":
		a.openSettings()

	case "L":
		// Like / Unlike — toggle the current cursor track (Library) or
		// playing track in Favorites.
		var target *library.Track
		if a.activeTab == tabLocal {
			target = a.cursorTrack()
		} else if a.currentTrack != nil {
			target = a.currentTrack
		}
		if target != nil {
			return a.cmdToggleFavorite(*target)
		}

	case "a":
		// Add cursor track to a playlist (Library tab only).
		if a.activeTab == tabLocal {
			if ct := a.cursorTrack(); ct != nil {
				a.activeOvl = overlayAddToPlaylist
				a.ovlScrollRow = 0
				// Load the playlist list for the overlay.
				a.ovlPlaylists = a.playlists
				a.ovlPlCursor = 0
				if len(a.ovlPlaylists) == 0 {
					return a.cmdLoadPlaylists()
				}
			}
		}

	case "tab":
		// Cycle through all implemented tabs.
		a.activeTab = (a.activeTab + 1) % tabCount
		// When switching to the Playlist tab, ensure playlists are loaded.
		if a.activeTab == tabPlaylist && len(a.playlists) == 0 {
			return a.cmdLoadPlaylists()
		}

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
			a.statusMsg = iconError() + "  Directory not found: " + newDir
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
		}
	case "k", "up":
		if a.listCursor > 0 {
			a.listCursor--
		}
	case "g":
		a.listCursor = 0
	case "G":
		if len(a.playlists) > 0 {
			a.listCursor = len(a.playlists) - 1
		}
	case "enter":
		if len(a.playlists) == 0 {
			return nil
		}
		a.depth = playlistDepthTracks
		a.trackCursor = 0
		a.confirmDel = false
		pl := a.currentPlaylist()
		return a.cmdLoadPlaylistTracks(pl.ID)
	case "n":
		// New playlist.
		a.confirmDel = false
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
		// Delete selected playlist (not Favorites).
		if pl := a.currentPlaylist(); pl.ID != "" && pl.ID != store.FavoritesPlaylistID {
			if a.confirmDel {
				a.confirmDel = false
				return a.cmdDeletePlaylist(pl.ID)
			}
			a.confirmDel = true
		}
	case "esc":
		a.confirmDel = false
		// Switch back to Local tab on Esc from playlist list root.
		a.activeTab = tabLocal
	case "tab":
		a.activeTab = (a.activeTab + 1) % tabCount
	// Pass through playback keys.
	case " ":
		return a.cmdTogglePause()
	case "f":
		a.currentView = viewFullscreen
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
func (a *App) handlePlaylistTracksKey(msg tea.KeyMsg) tea.Cmd {
	tracks := a.plTracks
	switch msg.String() {
	case "j", "down":
		if a.trackCursor < len(tracks)-1 {
			a.trackCursor++
		}
	case "k", "up":
		if a.trackCursor > 0 {
			a.trackCursor--
		}
	case "g":
		a.trackCursor = 0
	case "G":
		if len(tracks) > 0 {
			a.trackCursor = len(tracks) - 1
		}
	case "enter":
		if len(tracks) == 0 {
			return nil
		}
		pl := a.currentPlaylist()
		return a.cmdPlayPlaylist(pl.ID, a.trackCursor)
	case "x":
		// Remove track from playlist.
		if a.trackCursor >= 0 && a.trackCursor < len(tracks) {
			pl := a.currentPlaylist()
			trackID := tracks[a.trackCursor].ID
			a.statusMsg = T("playlist_removed")
			return a.cmdRemoveTrackFromPlaylist(pl.ID, trackID)
		}
	case "a":
		// Open inline add-tracks panel.
		a.addingTracks = true
		a.addInput.SetValue("")
		a.addInput.Focus()
		a.applyAddFilter()
		a.addCursor = 0
		return textinput.Blink
	case "esc":
		// Return to playlist list.
		a.depth = playlistDepthList
		a.confirmDel = false
	case " ":
		return a.cmdTogglePause()
	case "f":
		a.currentView = viewFullscreen
	case "L":
		if a.currentTrack != nil {
			return a.cmdToggleFavorite(*a.currentTrack)
		}
	case "tab":
		a.activeTab = (a.activeTab + 1) % tabCount
	case ",":
		a.openSettings()
	}
	return nil
}

// handleAddTracksKey handles keys in the inline add-tracks search panel.
func (a *App) handleAddTracksKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		a.addingTracks = false
		a.addInput.Blur()
		a.addInput.SetValue("")
	case "enter":
		// Add the selected track to the current playlist.
		if a.addCursor < 0 || a.addCursor >= len(a.addResults) {
			return nil
		}
		idx := a.addResults[a.addCursor]
		t := a.tracks[idx]
		pl := a.currentPlaylist()
		// Don't close the panel — allow continuous adding.
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
		}
	case "k", "up":
		if a.ovlPlCursor > 0 {
			a.ovlPlCursor--
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
		a.statusMsg = iconError() + "  Directory not found: " + a.musicDir
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
