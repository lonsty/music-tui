package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/lonsty/music-tui/internal/library"
)

// playlistTrackCtx returns the TrackListContext for the Playlist tab's track
// level (depth == playlistDepthTracks).  It re-uses the shared track-list
// engine so that every shared registry action works identically inside
// a playlist without any extra code.
func (a *App) playlistTrackCtx() TrackListContext {
	return TrackListContext{
		// ── Data ─────────────────────────────────────────────────────────
		Count: func() int {
			return len(a.plTracks)
		},
		TrackAt: func(pos int) library.Track {
			if pos < 0 || pos >= len(a.plTracks) {
				return library.Track{}
			}
			return a.plTracks[pos]
		},
		CursorPos: &a.trackCursor,
		PlayingPos: func(id string) int {
			for i, t := range a.plTracks {
				if t.ID == id {
					return i
				}
			}
			return -1
		},
		PageSize: a.trackPageSize,

		// ── Rendering ────────────────────────────────────────────────────
		HeaderTitle: func() string {
			pl := a.currentPlaylist()
			return stylePanelTitle.Render(
				iconWithSpace(iconPlaylist()) + T("playlist_header") +
					breadcrumbSep() + a.playlistDisplayName(pl))
		},
		HeaderRight: func() string {
			return styleTrackMeta.Render(" " + T("playlist_back_hint"))
		},
		// Enable marquee so the selected track row scrolls just like Local tab.
		UseMarquee:    true,
		ShowSearchBar: false,

		// ── Key callbacks ─────────────────────────────────────────────────
		OnEnter: func(a *App, pos int) tea.Cmd {
			pl := a.currentPlaylist()
			return a.cmdPlayPlaylist(pl.ID, pos)
		},
		OnExtraKeys: func(a *App, action ActionID, msg tea.KeyMsg) (bool, tea.Cmd) {
			switch action {
			case actionPlaylistRemove:
				// Remove cursor track from the playlist and arm undo.
				tracks := a.plTracks
				if a.trackCursor >= 0 && a.trackCursor < len(tracks) {
					pl := a.currentPlaylist()
					t := tracks[a.trackCursor]
					a.undoRemove = &undoRemove{
						playlistID: pl.ID,
						trackID:    t.ID,
						position:   a.trackCursor,
						title:      t.DisplayTitle(),
						expiresAt:  time.Now().Add(playlistUndoWindow),
					}
					a.statusMsg = fmt.Sprintf(T("playlist_removed_detail"), t.DisplayTitle())
					a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
					return true, a.cmdRemoveTrackFromPlaylist(pl.ID, t.ID)
				}
				return true, nil
			case actionPlaylistUndo:
				undo := a.undoRemove
				if undo == nil || time.Now().After(undo.expiresAt) {
					a.undoRemove = nil
					a.statusMsg = T("undo_expired")
					a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
					return true, nil
				}
				a.undoRemove = nil
				a.statusMsg = fmt.Sprintf(T("undo_restored"), undo.title)
				a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
				return true, a.cmdInsertTrackAt(undo.playlistID, undo.trackID, undo.position)
			case actionPlaylistMoveDown, actionPlaylistMoveUp:
				delta := 1
				if action == actionPlaylistMoveUp {
					delta = -1
				}
				tracks := a.plTracks
				next := a.trackCursor + delta
				if a.trackCursor < 0 || a.trackCursor >= len(tracks) || next < 0 || next >= len(tracks) {
					return true, nil
				}
				pl := a.currentPlaylist()
				t := tracks[a.trackCursor]
				a.trackCursor = next
				a.syncPlaylistTrackMarquee()
				return true, a.cmdMoveTrack(pl.ID, t.ID, delta)
			case actionPlaylistAddTrack:
				// Open inline add-tracks search panel.
				a.addingTracks = true
				a.addDelta = make(map[string]int)
				a.addInput.SetValue("")
				a.addInput.Focus()
				a.applyAddFilter()
				a.addCursor = 0
				return true, textinput.Blink
			}
			return false, nil
		},
		OnEsc: func(a *App) {
			// Return to the playlist list level.
			a.depth = playlistDepthList
			// Sync the playlist row marquee so the selected row
			// shows the correct playlist name immediately.
			a.syncPlaylistRowMarquee()
		},
		SyncMarquee: func(a *App) {
			// Update mqRow from the playlist track under the cursor,
			// NOT from filteredIdxs[cursorPos] which belongs to the Local tab.
			a.syncPlaylistTrackMarquee()
		},
		LikeTarget: func(a *App) *library.Track {
			// Like the track under the cursor, consistent with Local tab.
			if a.trackCursor >= 0 && a.trackCursor < len(a.plTracks) {
				t := a.plTracks[a.trackCursor]
				return &t
			}
			return nil
		},

		EmptyMsg:   "playlist_tracks_empty",
		EmptyHints: []ActionID{actionPlaylistAddTrack},
	}
}
