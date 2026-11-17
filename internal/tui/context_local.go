package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
)

// localTrackListCtx returns the TrackListContext for the Local tab.
// It wires up Local-specific data accessors and callbacks so that
// renderSharedTrackList / handleSharedTrackListKey produce behaviour
// identical to the original renderTrackList / handleNormalKey.
func (a *App) localTrackListCtx() TrackListContext {
	return TrackListContext{
		// ── Data ─────────────────────────────────────────────────────────
		Count: a.filteredLen,
		TrackAt: func(pos int) library.Track {
			if pos < 0 || pos >= len(a.filteredIdxs) {
				return library.Track{}
			}
			return a.tracks[a.filteredIdxs[pos]]
		},
		CursorPos: &a.cursorPos,
		PlayingPos: func(id string) int {
			return a.filteredPos(id)
		},
		PageSize: a.trackPageSize,

		// ── Rendering ────────────────────────────────────────────────────
		HeaderTitle: func() string {
			label := T("library_title")
			if a.drilldown != nil {
				for i := range a.tracks {
					if !a.drilldown.ids[a.tracks[i].ID] {
						continue
					}
					if a.drilldown.kind == libraryViewAlbums {
						label += " · " + a.tracks[i].Album
					} else {
						label += " · " + a.tracks[i].DisplayAlbumArtist()
					}
					break
				}
				label += " · ← " + T("hint_back")
			}
			return stylePanelTitle.Render(iconWithSpace(iconLibrary()) + label)
		},
		HeaderRight:   func() string { return "" },
		UseMarquee:    true,
		ShowSearchBar: true,

		// ── Key callbacks ─────────────────────────────────────────────────
		OnEnter: func(a *App, pos int) tea.Cmd {
			return a.cmdPlayTrack(pos)
		},
		OnExtraKeys: func(a *App, action ActionID, msg tea.KeyMsg) (bool, tea.Cmd) {
			if action == actionAddToPlaylist {
				// Add cursor track to a playlist.
				if ct := a.cursorTrack(); ct != nil {
					a.activeOvl = overlayAddToPlaylist
					a.ovlScrollRow = 0
					a.ovlPlaylists = a.playlists
					a.ovlPlCursor = 0
					if len(a.ovlPlaylists) == 0 {
						return true, a.cmdLoadPlaylists()
					}
				}
				return true, nil
			}
			return false, nil
		},
		OnEsc: func(a *App) {
			a.activeOvl = overlayNone
		},
		LikeTarget: func(a *App) *library.Track {
			return a.cursorTrack()
		},

		EmptyHints: []ActionID{actionSettingsOpen},
	}
}
