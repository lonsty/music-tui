package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/lyrics"
	"github.com/lonsty/music-tui/internal/lyrics/online"
	"github.com/lonsty/music-tui/internal/store"
)

// ── Marquee helpers ───────────────────────────────────────────────────────────

// syncRowMarquee updates mqRow to match the current cursor position.
// The text is the middle-column content: Album · Artist · Title.
func (a *App) syncRowMarquee() {
	t := a.cursorTrack()
	if t == nil {
		a.mqRow.SetText("")
		return
	}
	a.mqRow.SetText(rowMidText(*t))
}

// syncMarquees updates all Marquee texts from the current track.
// Call whenever the current track changes.
func (a *App) syncMarquees() {
	// Invalidate cover art cache whenever the track changes.
	a.coverRendered = ""

	if a.currentTrack == nil {
		a.mqTitle.SetText("")
		a.mqMeta.SetText("")
		a.mqArtist.SetText("")
		a.mqAlbum.SetText("")
		return
	}
	t := a.currentTrack
	a.mqTitle.SetText(t.DisplayTitle())
	a.mqMeta.SetText(displayArtist(*t) + " · " + t.Album)
	a.mqArtist.SetText(displayArtist(*t))
	a.mqAlbum.SetText(t.Album)
}

// getCoverArt renders the current track's embedded cover art inside a square
// bordered box that fits within maxOuterCols columns and maxOuterRows character
// rows.
//
// The largest visual square is chosen: each character row equals 2 pixel rows,
// so outerCols = outerRows*2.  Both maxOuterCols and maxOuterRows constrain the
// result.  The image is scaled with its original aspect ratio preserved
// ("contain"): the longest edge fills the inner area and the shorter edge is
// centred with blank padding.  Falls back to buildCoverPlaceholderSized when no
// cover art is available or rendering fails.
//
// Results are cached and reused as long as the track and box dimensions are
// unchanged.
func (a *App) getCoverArt(maxOuterCols, maxOuterRows int) string {
	outerRows := maxOuterCols / 2
	if maxOuterRows > 0 && outerRows > maxOuterRows {
		outerRows = maxOuterRows
	}
	if outerRows < coverMinSize {
		outerRows = coverMinSize
	}
	outerCols := outerRows * 2

	if a.currentTrack == nil {
		return a.cachedPlaceholder(outerCols, outerRows)
	}

	// Prefer in-memory CoverArt (fresh scan); fall back to CoverPath (DB load).
	coverData := a.currentTrack.CoverArt
	if len(coverData) == 0 && a.currentTrack.CoverPath != "" {
		if data, err := os.ReadFile(a.currentTrack.CoverPath); err == nil {
			coverData = data
			// Cache back into the track to avoid re-reading every frame.
			a.currentTrack.CoverArt = data
		}
	}

	if len(coverData) == 0 {
		return a.cachedPlaceholder(outerCols, outerRows)
	}
	if a.coverRendered != "" && a.coverW == outerCols && !a.coverPlaceholder {
		return a.coverRendered
	}

	rendered := renderCoverArt(coverData, outerCols, outerRows)
	if rendered == "" {
		return a.cachedPlaceholder(outerCols, outerRows)
	}
	a.coverRendered = rendered
	a.coverW = outerCols
	a.coverPlaceholder = false
	return a.coverRendered
}

// cachedPlaceholder returns the no-art placeholder, reusing the previous
// string when the box size has not changed.
func (a *App) cachedPlaceholder(outerCols, outerRows int) string {
	if a.coverRendered != "" && a.coverW == outerCols && a.coverPlaceholder {
		return a.coverRendered
	}
	placeholder := buildCoverPlaceholderSized(outerCols, outerRows)
	a.coverRendered = placeholder
	a.coverW = outerCols
	a.coverPlaceholder = true
	return placeholder
}

// tickMarquees advances all Marquee scroll positions.
// It does NOT update marquee text — that is handled by syncRowMarquee()
// (called on cursor movement) and syncMarquees() (called on track change).
// Keeping text updates out of the tick path avoids a rowMidText() allocation
// every 500 ms regardless of whether the cursor or track has changed.
func (a *App) tickMarquees() {
	listW := a.trackListInnerW()
	const leftFixW = 2
	const rightColW = 10
	midAvail := listW - leftFixW - rightColW
	if midAvail < 4 {
		midAvail = 4
	}

	// Playlist list level: left icon (2) + right count (8).
	const plRightColW = 8
	plMidAvail := listW - leftFixW - plRightColW
	if plMidAvail < 4 {
		plMidAvail = 4
	}

	miniW := a.miniPlayerW() - 4
	if miniW <= 0 {
		// Collapsed / narrow: use a small threshold so that even short
		// titles (e.g. 10 CJK chars = 20 display cols) trigger scrolling.
		miniW = 16
	}
	fullW := a.fullPlayerInnerW() - 4
	if fullW <= 0 {
		fullW = 20
	}

	a.mqRow.Tick(midAvail)
	a.mqPlRow.Tick(plMidAvail)
	a.mqTitle.Tick(miniW)
	a.mqMeta.Tick(miniW)
	a.mqArtist.Tick(fullW)
	a.mqAlbum.Tick(fullW)
}

// playlistDisplayName returns the display name for a playlist, using the
// localised name when it is the built-in Favorites playlist.
func (a *App) playlistDisplayName(pl store.Playlist) string {
	if pl.ID == store.FavoritesPlaylistID {
		return T("playlist_favorites_name")
	}
	return pl.Name
}

// keepOvlCursorVisible adjusts ovlScrollRow so that ovlPlCursor is inside
// the visible body area of the playlist-picker overlay.
func (a *App) keepOvlCursorVisible() {
	a.scrollOverlayToCursor(a.ovlPlCursor)
}

// refreshPlListMaxColW recomputes the cached right-column width for playlist
// track counts.  Must be called after playlists are loaded or track counts
// change (add/remove).  Without a store the minimum width (4) is used.
func (a *App) refreshPlListMaxColW() {
	const minW = 4
	a.plListMaxColW = minW
	for _, pl := range a.playlists {
		if w := strWidth(fmt.Sprintf(T("playlist_track_count"), pl.TrackCount)); w > a.plListMaxColW {
			a.plListMaxColW = w
		}
	}
}

// maybeRestorePlaylistTracks triggers a track load for the playlist that was
// last open when the app exited.  Returns the load command or nil.
// Only effective once (clears restorePlaylistID after the first call).
func (a *App) maybeRestorePlaylistTracks() tea.Cmd {
	if a.restorePlaylistID == "" || a.depth != playlistDepthTracks {
		return nil
	}
	plID := a.restorePlaylistID
	a.restorePlaylistID = ""
	// Verify the playlist still exists.
	for _, pl := range a.playlists {
		if pl.ID == plID {
			return a.cmdLoadPlaylistTracks(plID)
		}
	}
	return nil
}

// syncPlaylistRowMarquee updates mqPlRow to match the currently selected
// playlist in the playlist list level.
func (a *App) syncPlaylistRowMarquee() {
	if a.listCursor < 0 || a.listCursor >= len(a.playlists) {
		a.mqPlRow.SetText("")
		return
	}
	pl := a.playlists[a.listCursor]
	name := a.playlistDisplayName(pl)
	a.mqPlRow.SetText(name)
}

// anchorPlaylistTrackCursor moves trackCursor to the currently playing
// track when it exists in the playlist, matching the behaviour of mature
// players (cmus, ncmpcpp) that always jump to the playing song when
// entering or switching to a track list.
func (a *App) anchorPlaylistTrackCursor() {
	if a.currentTrack != nil && len(a.plTracks) > 0 {
		for i, t := range a.plTracks {
			if t.ID == a.currentTrack.ID {
				a.trackCursor = i
				return
			}
		}
	}
}

// syncPlaylistTrackMarquee updates mqRow to match the currently selected
// track in the playlist track level.  This is necessary because the Playlist
// tab's track level shares mqRow with the Local tab; without syncing, the
// selected row may render stale text from a different view.
func (a *App) syncPlaylistTrackMarquee() {
	if a.trackCursor >= 0 && a.trackCursor < len(a.plTracks) {
		a.mqRow.SetText(rowMidText(a.plTracks[a.trackCursor]))
	} else {
		a.mqRow.SetText("")
	}
}

// syncLyricsActive updates activeIdx to the lyric line that should be
// highlighted at the current playback position.
//
// For plain-text (unsynchronised) lyrics where all timestamps are zero,
// activeIdx is kept at -1 so no line is highlighted — the entire list is
// shown at uniform brightness.
//
// When the browse cursor has expired (browseExpired=true) and the playback
// line advances to a new index, the cursor is snapped back to follow mode
// at that natural lyric boundary rather than jumping mid-line.
func (a *App) syncLyricsActive() {
	if len(a.lines) == 0 || !a.synced {
		a.activeIdx = -1
		return
	}
	if a.player == nil {
		return
	}
	pos := a.player.Position()
	idx := lyricIndexAt(a.lines, pos, a.offset)

	// Snap-back on natural line advance when browse has expired.
	if a.browseExpired && idx != a.prevActiveIdx && idx >= 0 {
		a.browseCenterIdx = -1
		a.browseTicks = 0
		a.browseExpired = false
	}

	a.prevActiveIdx = a.activeIdx
	a.activeIdx = idx
}

// buildLyricsProvider constructs the lyrics provider chain from the current
// configuration.  It is called at startup and can be called again whenever
// the user changes provider settings (e.g. enabling or disabling online lookup).
//
// Chain order: local .lrc files → cached lrclib.net.
// If the lyrics cache directory cannot be resolved, the chain falls back to
// local-only so that offline usage is unaffected.
func buildLyricsProvider() lyrics.Provider {
	base := lyrics.LocalLRCProvider{}
	cacheDir, err := store.LyricsCacheDir()
	if err != nil {
		return base
	}
	cachedOnline := lyrics.NewCachedProvider(online.NewLrcLibProvider(), cacheDir)
	return &lyrics.ChainProvider{
		Providers: []lyrics.Provider{base, cachedOnline},
	}
}
