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
// engine so that all Local features (m/b/r/R/+/-/<>/?/i/L/n/p/space/…) work
// identically inside a playlist without any extra code.
func (a *App) playlistTrackCtx() TrackListContext {
	return TrackListContext{
		// ── Data ─────────────────────────────────────────────────────────
		Tracks: func() []library.Track {
			return a.plTracks
		},
		CursorPos: &a.trackCursor,
		TotalCount: func() int {
			return a.plTrackCount
		},
		PlayingPos: func(id string) int {
			for i, t := range a.plTracks {
				if t.ID == id {
					return i
				}
			}
			return -1
		},

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
		OnExtraKeys: func(a *App, msg tea.KeyMsg) (bool, tea.Cmd) {
			switch msg.String() {
			case "x":
				// Remove cursor track from the playlist.
				tracks := a.plTracks
				if a.trackCursor >= 0 && a.trackCursor < len(tracks) {
					pl := a.currentPlaylist()
					t := tracks[a.trackCursor]
					a.statusMsg = fmt.Sprintf(T("playlist_removed_detail"), t.DisplayTitle())
					a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
					return true, a.cmdRemoveTrackFromPlaylist(pl.ID, t.ID)
				}
				return true, nil
			case "a":
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

		EmptyMsg: "playlist_tracks_empty",

		// ── Status-bar hints ──────────────────────────────────────────────
		ExtraHints: func(a *App) []statusHint { return nil },
	}
}
