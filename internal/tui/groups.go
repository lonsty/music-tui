package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
)

// rebuildGroups refreshes the album and artist caches from the tracks that
// pass the current format preference.  Call it whenever tracks or the format
// preference change; the render path never rebuilds groups.
func (a *App) rebuildGroups() {
	preferred := applyFormatPreference(a.tracks, a.formatPref)
	a.albumGroups = buildTrackGroups(preferred, libraryViewAlbums)
	a.artistGroups = buildTrackGroups(preferred, libraryViewArtists)
	a.groupsBuilt = true
	a.groupsDirty = false
	a.groupsPref = a.formatPref
	if a.groupCursor >= a.activeGroupCount() {
		a.groupCursor = a.activeGroupCount() - 1
	}
	if a.groupCursor < 0 {
		a.groupCursor = 0
	}
}

// buildTrackGroups groups tracks by album or artist while preserving the
// incoming sort order.  Each group carries the track IDs so drilling in does
// not need another pass over the library.
func buildTrackGroups(tracks []library.Track, kind libraryView) []trackGroup {
	var groups []trackGroup
	index := make(map[string]int, len(tracks))
	for _, t := range tracks {
		var key, title, subtitle string
		if kind == libraryViewAlbums {
			artist := t.DisplayAlbumArtist()
			key = strings.ToLower(artist) + "\x00" + strings.ToLower(t.Album)
			title = t.Album
			subtitle = artist
		} else {
			artist := t.DisplayAlbumArtist()
			key = strings.ToLower(artist)
			title = artist
		}
		if i, ok := index[key]; ok {
			groups[i].IDs = append(groups[i].IDs, t.ID)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, trackGroup{
			Key:      key,
			Title:    title,
			Subtitle: subtitle,
			IDs:      []string{t.ID},
		})
	}
	return groups
}

// activeGroups returns the group cache for the current view, rebuilding it
// lazily when tracks or the format preference changed since it was built.
func (a *App) activeGroups() []trackGroup {
	if !a.groupsBuilt || a.groupsDirty {
		a.rebuildGroups()
	}
	if a.view == libraryViewArtists {
		return a.artistGroups
	}
	return a.albumGroups
}

func (a *App) activeGroupCount() int { return len(a.activeGroups()) }

// groupTitle localises empty album/artist names.
func (a *App) groupTitle(g trackGroup) string {
	if strings.TrimSpace(g.Title) != "" {
		return g.Title
	}
	if a.view == libraryViewAlbums {
		return T("unknown_album")
	}
	return T("unknown_artist")
}

// groupSubtitle localises the unspecified artist sentinel.
func (a *App) groupSubtitle(g trackGroup) string {
	if g.Subtitle == "" || g.Subtitle == "Unknown Artist" {
		return T("unknown_artist")
	}
	return g.Subtitle
}

// cycleLibraryView advances Tracks → Albums → Artists → Tracks.
func (a *App) cycleLibraryView() tea.Cmd {
	if strings.TrimSpace(a.searchInput.Value()) != "" {
		a.statusMsg = T("clear_search_first")
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		return nil
	}
	if a.drilldown != nil {
		a.drilldown = nil
		a.applyFilter()
	}
	a.view = (a.view + 1) % libraryViewCount
	a.groupCursor = 0
	return nil
}

// enterGroup drills into the selected album or artist.
func (a *App) enterGroup() tea.Cmd {
	groups := a.activeGroups()
	cur := a.groupCursor
	if cur < 0 || cur >= len(groups) {
		return nil
	}
	g := groups[cur]
	ids := make(map[string]bool, len(g.IDs))
	for _, id := range g.IDs {
		ids[id] = true
	}
	a.drilldown = &groupRef{kind: a.view, key: g.Key, ids: ids}
	a.view = libraryViewTracks
	a.cursorPos = 0
	a.applyFilter()
	a.syncRowMarquee()
	return nil
}

// exitDrilldown returns to the grouped view the user came from.
func (a *App) exitDrilldown() {
	kind := libraryViewAlbums
	if a.drilldown != nil {
		kind = a.drilldown.kind
	}
	a.drilldown = nil
	a.view = kind
	a.applyFilter()
}

// jumpToPlayingGroup selects the group containing the current track.
func (a *App) jumpToPlayingGroup() {
	if a.currentTrack == nil {
		a.statusMsg = T("no_track_selected")
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		return
	}
	for i, g := range a.activeGroups() {
		for _, id := range g.IDs {
			if id == a.currentTrack.ID {
				a.groupCursor = i
				return
			}
		}
	}
	a.statusMsg = T("track_not_in_list")
	a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
}

// syncSearchLibraryView forces the flat track list while a search query is
// active and restores the previous grouped view when it is cleared.
func (a *App) syncSearchLibraryView(active bool) {
	if active {
		if a.view != libraryViewTracks {
			a.prevView = a.view
			a.view = libraryViewTracks
		}
		a.drilldown = nil
		return
	}
	if a.prevView != libraryViewTracks {
		a.view = a.prevView
		a.prevView = libraryViewTracks
		a.groupCursor = 0
	}
}

// handleGroupKey is the test-facing entry point for grouped library views.
func (a *App) handleGroupKey(msg tea.KeyMsg) tea.Cmd {
	scopes := []Scope{scopeGroup, scopeLocalGroups, scopeGlobal}
	return a.runAction(scopes, msg, a.handleGroupAction)
}

// handleGroupAction handles the album/artist list layer. Unhandled actions
// fall through to the shared track-list engine.
func (a *App) handleGroupAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	groups := a.activeGroups()
	switch action {
	case actionNavDown:
		if a.groupCursor < len(groups)-1 {
			a.groupCursor++
		}
		return nil
	case actionNavUp:
		if a.groupCursor > 0 {
			a.groupCursor--
		}
		return nil
	case actionNavTop:
		a.groupCursor = 0
		return nil
	case actionNavBottom:
		if len(groups) > 0 {
			a.groupCursor = len(groups) - 1
		}
		return nil
	case actionDrillDown:
		return a.enterGroup()
	case actionJumpPlaying:
		a.jumpToPlayingGroup()
		return nil
	case actionGroupBack, actionCancel:
		if a.drilldown != nil {
			a.exitDrilldown()
		}
		if strings.TrimSpace(a.searchInput.Value()) != "" {
			a.searchInput.SetValue("")
			a.searchInput.Blur()
			a.lastQuery = ""
			a.applyFilter()
			a.syncSearchLibraryView(false)
		}
		return nil
	}
	return a.handleSharedTrackListAction(a.localTrackListCtx(), action, msg)
}
