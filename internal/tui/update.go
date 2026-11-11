package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/store"
)

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.W, a.H = msg.Width, msg.Height
		a.progressBar.Width = a.miniPlayerW() - 6
		a.lyricsVP.Width = a.fullLyricsW() - 2
		a.lyricsVP.Height = a.fullBodyH() - 4
		return a, nil

	case scanDoneMsg:
		a.loading = false
		a.scanCancel = nil
		a.scanErr = msg.err
		// Update tracks regardless of error: cmdSyncLibrary always returns
		// whatever is in the DB even when syncErr is non-nil, so we show
		// the existing library and surface the error in the status bar.
		if msg.tracks != nil {
			a.tracks = msg.tracks
			a.rebuildTrackPos()
			a.groupsDirty = true
			// Re-apply the current search query and format preference so the
			// filtered list stays consistent after a library reload.
			a.applyFilter()
			a.rebuildShuffleIDs()
			// Re-anchor currentIdx to the playing track's ID so the position
			// is correct after the library list is replaced.
			// If the playing track is no longer in the library, stop playback.
			if a.currentTrack != nil {
				if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
					// Track still exists — update cursor to its new position.
					a.cursorPos = pos
				} else {
					// Track no longer exists — stop immediately.
					a.player.Stop()
					a.currentTrack = nil
					a.cursorPos = 0
					a.statusMsg = T("track_removed")
					a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				}
			}
			// Always clamp cursor and sync the row marquee after a
			// library reload, even when no track is playing.  Without
			// this the first render frame shows a stale/empty row.
			if n := a.filteredLen(); n > 0 {
				if a.cursorPos >= n {
					a.cursorPos = n - 1
				}
				if a.cursorPos < 0 {
					a.cursorPos = 0
				}
				a.syncRowMarquee()
			} else {
				a.cursorPos = 0
				a.mqRow.SetText("")
			}
		}
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf(T("sync_error"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		} else if a.libScanFromReload {
			// Only show "Loaded N tracks" when the scan was explicitly
			// triggered by Ctrl+R, not on every startup.
			a.statusMsg = fmt.Sprintf(T("loaded_tracks"), len(a.tracks))
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			a.libScanFromReload = false
		}
		return a, nil

	case playResultMsg:
		if msg.err != nil {
			a.statusMsg = fmt.Sprintf(T("playback_error"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}

		// Remember whether we were in chip mode before the track change.
		wasChipMode := a.chipMode

		// Reset all chip state unconditionally on track change.
		// Any in-flight conversion/crossfade goroutine will finish naturally
		// but its result will be ignored because chipOrigin won't match.
		a.chipMode = false
		a.chipBusy = false
		a.chipConverting = false

		a.currentTrack = msg.track
		// Position the cursor on the newly playing track.
		if msg.track != nil {
			if pos := a.filteredPos(msg.track.ID); pos >= 0 {
				a.cursorPos = pos
			}
		}
		a.statusMsg = ""
		a.syncMarquees()
		// Sync the correct row marquee depending on the active view.
		// When the Playlist tab's track level is visible, mqRow must
		// reflect plTracks[trackCursor]; otherwise it follows the
		// Local tab's cursorPos.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
			a.syncPlaylistTrackMarquee()
		} else {
			a.syncRowMarquee()
		}

		// Reset lyrics and kick off background load for the new track.
		a.lines = nil
		a.activeIdx = -1
		a.prevActiveIdx = -1
		a.trackID = ""
		a.synced = false
		a.offset = 0
		a.maxTextW = 0
		a.lyricsLoading = msg.track != nil // loading until lyricsLoadedMsg arrives
		a.browseCenterIdx = -1
		a.browseTicks = 0
		a.browseExpired = false

		// Reset isFavorite for the new track; will be updated by cmdCheckFavorite.
		a.isFavorite = false

		var cmds []tea.Cmd
		// If we were in chip mode, automatically start converting the new track.
		if wasChipMode {
			cmds = append(cmds, a.cmdToggleChip())
		}
		if msg.track != nil {
			cmds = append(cmds, a.cmdLoadLyrics(*msg.track))
			cmds = append(cmds, a.cmdCheckFavorite(msg.track.ID))
		}
		return a, tea.Batch(cmds...)

	case playlistsLoadedMsg:
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		} else {
			a.playlists = msg.playlists
			// Clamp cursor after reload (e.g. after delete).
			if a.listCursor >= len(a.playlists) {
				a.listCursor = len(a.playlists) - 1
			}
			if a.listCursor < 0 {
				a.listCursor = 0
			}
			// Sync the playlist row marquee after any list reload.
			a.syncPlaylistRowMarquee()
			// Recompute the right-column width cache after list changes.
			a.refreshPlListMaxColW()
			if msg.successMsg != "" {
				a.statusMsg = msg.successMsg
				a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			}
		}
		// Restore track-level view on startup.
		restoreCmd := a.maybeRestorePlaylistTracks()
		return a, restoreCmd

	case playlistTracksLoadedMsg:
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		if msg.duplicate {
			a.statusMsg = T("playlist_duplicate")
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		// Keep the cached list counts fresh for every playlist mutation.
		for i := range a.playlists {
			if a.playlists[i].ID == msg.playlistID {
				a.playlists[i].TrackCount = msg.count
				a.refreshPlListMaxColW()
				break
			}
		}
		// Only replace the visible track list when the message is about the
		// playlist currently open.  Adding from another tab must not overwrite
		// the track list being viewed.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks &&
			a.currentPlaylist().ID == msg.playlistID {
			a.plTracks = msg.tracks
			if a.trackCursor >= len(a.plTracks) {
				a.trackCursor = len(a.plTracks) - 1
			}
			if a.trackCursor < 0 {
				a.trackCursor = 0
			}
			// Anchor cursor to the currently playing track when entering
			// a playlist's track list (pendingTrackAnchor == true),
			// matching the behaviour of mature players (cmus, ncmpcpp)
			// that jump to the playing song when entering a new list.
			// Only anchor on explicit entry, not on background reloads
			// (e.g. favoriteChangedMsg) which should preserve the cursor.
			if a.pendingTrackAnchor {
				a.anchorPlaylistTrackCursor()
			}
			a.pendingTrackAnchor = false
			// Sync mqRow so the selected row renders the correct track
			// text immediately, rather than showing stale content from a
			// previous tab or playlist.
			a.syncPlaylistTrackMarquee()
		}
		return a, nil

	case favoriteChangedMsg:
		if msg.trackID == "" {
			return a, nil
		}
		// Update isFavorite only when this is about the currently playing track.
		if a.currentTrack != nil && msg.trackID == a.currentTrack.ID {
			a.isFavorite = msg.isFavorite
		}
		// Show feedback only for explicit user actions (trackTitle is set by
		// cmdToggleFavorite; cmdCheckFavorite leaves it empty).
		if msg.trackTitle == "" {
			return a, nil
		}
		{
			if msg.isFavorite {
				a.statusMsg = iconHeartFilled() + " " + msg.trackTitle
			} else {
				a.statusMsg = iconHeartEmpty() + " " + msg.trackTitle
			}
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		// If we're viewing the Favorites playlist, reload its track list.
		// The playlist list is also refreshed so cached counts stay accurate.
		if a.activeTab == tabPlaylist &&
			a.depth == playlistDepthTracks &&
			a.currentPlaylist().ID == store.FavoritesPlaylistID {
			return a, tea.Batch(
				a.cmdLoadPlaylistTracks(store.FavoritesPlaylistID),
				a.cmdLoadPlaylists(),
			)
		}
		return a, a.cmdLoadPlaylists()

	case playlistPlayMsg:
		// Replace the local library filteredIdxs with the playlist track order.
		// First, ensure all playlist tracks are present in a.tracks; add missing ones.
		if msg.err != nil {
			a.statusMsg = T("playlist_load_error") + ": " + msg.err.Error()
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		existing := make(map[string]int, len(a.tracks))
		for i, t := range a.tracks {
			existing[t.ID] = i
		}
		newIdxs := make([]int, 0, len(msg.tracks))
		for _, pt := range msg.tracks {
			if idx, ok := existing[pt.ID]; ok {
				newIdxs = append(newIdxs, idx)
			}
		}
		if len(newIdxs) == 0 {
			a.statusMsg = T("no_tracks_hint")
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		// Warn when some playlist tracks are no longer in the library.
		if len(newIdxs) < len(msg.tracks) {
			a.statusMsg = fmt.Sprintf(T("playlist_tracks_missing"), len(msg.tracks)-len(newIdxs))
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		}
		a.filteredIdxs = newIdxs
		a.rebuildFilteredPos()
		pos := msg.startPos
		if pos >= len(newIdxs) {
			pos = 0
		}
		a.cursorPos = pos
		a.syncRowMarquee()
		return a, a.cmdPlayTrack(pos)

	case noopMsg:
		return a, nil

	case playModeChangedMsg:
		a.playMode = msg.mode
		if a.playMode == playModeRandom {
			a.rebuildShuffleIDs()
		}
		return a, nil

	case retroChangedMsg:
		a.retroIdx = msg.idx
		if a.player != nil {
			a.player.SetRetroPreset(msg.idx)
		}
		return a, nil

	case chip8DoneMsg:
		if msg.err != nil {
			// Conversion failed — unlock and surface the error.
			a.chipBusy = false
			a.chipConverting = false
			a.statusMsg = fmt.Sprintf(T("chip_convert_failed"), msg.err.Error())
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return a, nil
		}
		// Cache the result; conversion phase is done, crossfade phase begins.
		a.chipPath = msg.chipPath
		a.chipOrigin = msg.originPath
		a.chipConverting = false
		pos := a.player.Position()
		// Crossfade runs in a tea.Cmd goroutine; state mutations happen via
		// chipCrossfadeDoneMsg so we never write App fields from a goroutine.
		return a, func() tea.Msg {
			_ = a.player.CrossfadeTo(msg.chipPath, pos)
			return chipCrossfadeDoneMsg{chipMode: true}
		}

	case chipCrossfadeDoneMsg:
		a.chipMode = msg.chipMode
		a.chipBusy = false
		return a, nil

	case lyricsLoadedMsg:
		// Discard stale results — track may have changed while loading.
		if a.currentTrack != nil && msg.trackID == a.currentTrack.ID {
			if msg.err != nil {
				// Fetch failed (e.g. network timeout, context cancelled).
				// Leave lines nil so the UI shows "No lyrics" rather than stale data.
				a.lines = nil
				a.synced = false
			} else {
				a.lines = msg.lines
				a.offset = msg.offset
				a.activeIdx = -1
				a.trackID = msg.trackID
				a.maxTextW = maxLineWidth(msg.lines)
				// Determine whether any line carries a real timestamp.
				// Pure plain-text lyrics (all Time=0) are shown statically
				// without any active-line highlight.
				a.synced = false
				for _, l := range msg.lines {
					if l.Time > 0 {
						a.synced = true
						break
					}
				}
			}
		}
		a.lyricsLoading = false
		return a, nil

	case playbackEndedMsg:
		// Sequential mode reached the end of the list — clear the playing
		// state so the status bar correctly shows Stopped.
		a.currentTrack = nil
		a.isFavorite = false
		a.syncMarquees()
		// Re-anchor cursor row text so row 0 isn't blank.  Sync the
		// correct marquee depending on the active view.
		if a.activeTab == tabPlaylist && a.depth == playlistDepthTracks {
			a.syncPlaylistTrackMarquee()
		} else {
			a.syncRowMarquee()
		}
		return a, nil

	case tickMsg:
		a.tickMarquees()
		a.syncLyricsActive()
		// Clear transient status messages after their expiry time.
		if !a.statusMsgExpiry.IsZero() && time.Now().After(a.statusMsgExpiry) {
			a.statusMsg = ""
			a.statusMsgExpiry = time.Time{}
		}
		// Browse cursor expiry: count ticks while cursor is active.
		// Do NOT reset immediately — wait for the next line change so the
		// snap-back happens at a natural lyric boundary (smooth transition).
		if a.browseCenterIdx >= 0 && !a.browseExpired {
			a.browseTicks++
			if a.browseTicks >= browseFadeOutTicks {
				a.browseExpired = true
			}
		}
		return a, tick()

	case trackDoneMsg:
		return a, a.cmdAdvance()

	case tea.KeyMsg:
		return a, a.handleKey(msg)

	case progress.FrameMsg:
		prog, cmd := a.progressBar.Update(msg)
		a.progressBar = prog.(progress.Model)
		return a, cmd

	case viewport.Model:
		a.lyricsVP = msg
		return a, nil
	}

	// Forward to search input while search overlay is active.
	if a.activeOvl == overlaySearch {
		var cmd tea.Cmd
		before := a.searchInput.Value()
		a.searchInput, cmd = a.searchInput.Update(msg)
		// Cursor-blink and other non-text messages must not re-run the whole
		// library filter; only an actual query change does.
		if a.searchInput.Value() != before {
			a.applyFilter()
		}
		return a, cmd
	}

	return a, nil
}
