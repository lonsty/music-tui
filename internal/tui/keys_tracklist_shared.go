package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
)

// handleSharedTrackListKey is the unified key handler for any tab whose main
// panel is a scrollable track list.  It operates on the supplied
// TrackListContext so that both Local and Playlist (track level) share the
// same full feature set without code duplication.
//
// Key responsibilities handled here:
//   - Cursor navigation (j/k/g/G)
//   - Playback control (Enter/Space/n/p/m/b/r/R/+/-/</>)
//   - Panel control (f/l/\/,)
//   - Overlays (?/i) — using ctx.LikeTarget for the info overlay
//   - Like/Unlike (L) — using ctx.LikeTarget
//   - Search (/) — only when ctx.ShowSearchBar is true
//   - Esc — delegated to ctx.OnEsc
//   - Tab cycling
//   - Any remaining keys — delegated to ctx.OnExtraKeys
func (a *App) handleSharedTrackListKey(ctx TrackListContext, msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return a.cmdQuit()

	// ── Navigation ───────────────────────────────────────────────────────────
	case "j", "down":
		if *ctx.CursorPos < ctx.Count()-1 {
			*ctx.CursorPos++
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}

	case "k", "up":
		if *ctx.CursorPos > 0 {
			*ctx.CursorPos--
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}

	case "pgdown", "ctrl+d":
		moveCursor(a, ctx, ctx.PageSize())

	case "pgup", "ctrl+u":
		moveCursor(a, ctx, -ctx.PageSize())

	case "g":
		*ctx.CursorPos = 0
		if ctx.UseMarquee {
			syncMarquee(a, ctx)
		}

	case "G":
		if n := ctx.Count(); n > 0 {
			*ctx.CursorPos = n - 1
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}

	case "c":
		if a.currentTrack == nil {
			a.statusMsg = T("no_track_selected")
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return nil
		}
		pos := ctx.PlayingPos(a.currentTrack.ID)
		if pos < 0 {
			a.statusMsg = T("track_not_in_list")
			a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
			return nil
		}
		*ctx.CursorPos = pos
		if ctx.UseMarquee {
			syncMarquee(a, ctx)
		}

	// ── Playback ─────────────────────────────────────────────────────────────
	case "enter":
		n := ctx.Count()
		if n == 0 {
			return nil
		}
		pos := *ctx.CursorPos
		if pos < 0 || pos >= n {
			return nil
		}
		// Second Enter on the currently-playing track → open fullscreen.
		if a.currentTrack != nil && a.currentTrack.ID == ctx.TrackAt(pos).ID {
			a.currentView = viewFullscreen
			return nil
		}
		return ctx.OnEnter(a, pos)

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

	// ── Retro lo-fi effect ────────────────────────────────────────────────────
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

	// ── Views / panel control ─────────────────────────────────────────────────
	case "v":
		return a.cycleLibraryView()

	case "f":
		a.currentView = viewFullscreen

	case "l":
		if a.showMiniPlayer() {
			if a.rightMode == rightPanelLyrics {
				a.rightMode = rightPanelPlayer
			} else {
				a.rightMode = rightPanelLyrics
			}
		}

	case `\`:
		a.rightCollapsed = !a.rightCollapsed

	// ── Overlays ──────────────────────────────────────────────────────────────
	case "i":
		a.activeOvl = overlayInfo
		a.ovlScrollRow = 0

	case "?":
		a.activeOvl = overlayHelp
		a.ovlScrollRow = 0

	case ",":
		a.openSettings()

	// ── Search ────────────────────────────────────────────────────────────────
	case "/":
		if ctx.ShowSearchBar {
			a.activeOvl = overlaySearch
			a.searchInput.Focus()
			a.searchInput.CursorEnd()
		}

	// ── Like / Unlike ─────────────────────────────────────────────────────────
	case "L":
		if ctx.LikeTarget != nil {
			if target := ctx.LikeTarget(a); target != nil {
				return a.cmdToggleFavorite(*target)
			}
		}

	// ── Tab cycling ───────────────────────────────────────────────────────────
	case "tab":
		a.activeTab = (a.activeTab + 1) % tabCount
		switch a.activeTab {
		case tabLocal:
			a.restoreLocalFilter()
		case tabPlaylist:
			// Refresh the cached playlist list (names and track counts)
			// whenever the user returns to this tab.
			if len(a.playlists) == 0 {
				return a.cmdLoadPlaylists()
			}
			// Sync playlist row marquee when switching TO the Playlists tab,
			// so the selected row shows the correct playlist name immediately.
			a.syncPlaylistRowMarquee()
			// When at track level, anchor the cursor to the currently playing
			// track (matching cmus/ncmpcpp behaviour) and sync the track row
			// marquee so the selected row renders the correct text immediately.
			// Also re-load tracks from DB so that changes made in other tabs
			// (e.g. like/unlike in Local tab) are reflected.
			if a.depth == playlistDepthTracks {
				a.anchorPlaylistTrackCursor()
				a.syncPlaylistTrackMarquee()
				if pl := a.currentPlaylist(); pl.ID != "" {
					return a.cmdLoadPlaylistTracks(pl.ID)
				}
			}
			return a.cmdLoadPlaylists()
		}

	// ── Esc ───────────────────────────────────────────────────────────────────
	case "esc":
		// Drilled-in album/artist returns to its group list first.
		if a.drilldown != nil {
			a.exitDrilldown()
			return nil
		}
		// A visible search filter is cleared before any context-level Esc
		// action, so the list never stays silently filtered.
		if ctx.ShowSearchBar && strings.TrimSpace(a.searchInput.Value()) != "" {
			a.searchInput.SetValue("")
			a.searchInput.Blur()
			a.lastQuery = ""
			a.applyFilter()
			a.syncSearchLibraryView(false)
			if a.currentTrack != nil {
				if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
					*ctx.CursorPos = pos
				}
			}
			syncMarquee(a, ctx)
			return nil
		}
		if ctx.OnEsc != nil {
			ctx.OnEsc(a)
		}
		return nil

	// ── Tab-specific keys ─────────────────────────────────────────────────────
	default:
		if ctx.OnExtraKeys != nil {
			if handled, cmd := ctx.OnExtraKeys(a, msg); handled {
				return cmd
			}
		}
	}

	return nil
}

// moveCursor moves the cursor by delta rows, clamped to the list bounds.
func moveCursor(a *App, ctx TrackListContext, delta int) {
	if delta == 0 {
		return
	}
	n := ctx.Count()
	if n == 0 {
		*ctx.CursorPos = 0
		return
	}
	pos := *ctx.CursorPos + delta
	if pos < 0 {
		pos = 0
	}
	if pos >= n {
		pos = n - 1
	}
	if pos == *ctx.CursorPos {
		return
	}
	*ctx.CursorPos = pos
	if ctx.UseMarquee {
		syncMarquee(a, ctx)
	}
}

// syncMarquee calls ctx.SyncMarquee if set, otherwise falls back to the
// default syncRowMarquee which reads from filteredIdxs[cursorPos].
func syncMarquee(a *App, ctx TrackListContext) {
	if ctx.SyncMarquee != nil {
		ctx.SyncMarquee(a)
	} else {
		a.syncRowMarquee()
	}
}

// buildTrackListHints constructs the status-bar hint slice for a track-list
// context.  It combines the shared hints (playback, panel) with any
// context-specific extra hints returned by ctx.ExtraHints.
func buildTrackListHints(a *App, ctx TrackListContext, displayState audio.State) []statusHint {
	lyricsHintLabel := T("hint_lyrics")
	if a.rightMode == rightPanelLyrics {
		lyricsHintLabel = T("hint_player")
	}
	showLyricsHint := a.showMiniPlayer()

	collapseHintLabel := T("hint_collapse")
	if a.rightCollapsed {
		collapseHintLabel = T("hint_expand")
	}

	isLocal := ctx.ShowSearchBar

	var hints []statusHint
	switch displayState {
	case audio.StatePlaying:
		if isLocal {
			// Local: ? Spc n/p </> m l f \ L / , Tab q
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Spc", T("hint_pause")},
				{"n/p", T("hint_next_prev")},
				{"</>", T("hint_seek")},
				{"m", T("hint_mode")},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel})
			}
			hints = append(hints,
				statusHint{"f", T("hint_fullscreen")},
				statusHint{`\`, collapseHintLabel},
				statusHint{"L", T("hint_like_toggle")},
				statusHint{"/", T("hint_search")},
				statusHint{",", T("hint_settings")},
				statusHint{"Tab", T("hint_switch_tab")},
				statusHint{"q", T("hint_quit")},
			)
		} else {
			// Playlist: ? Spc n/p </> m l f \ a x L , Esc Tab q
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Spc", T("hint_pause")},
				{"n/p", T("hint_next_prev")},
				{"</>", T("hint_seek")},
				{"m", T("hint_mode")},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel})
			}
			hints = append(hints,
				statusHint{"f", T("hint_fullscreen")},
				statusHint{`\`, collapseHintLabel},
				statusHint{"a", T("hint_playlist_add_track")},
				statusHint{"x", T("hint_playlist_remove_track")},
				statusHint{"L", T("hint_like_toggle")},
				statusHint{",", T("hint_settings")},
				statusHint{"Esc", T("hint_back")},
				statusHint{"Tab", T("hint_switch_tab")},
				statusHint{"q", T("hint_quit")},
			)
		}

	case audio.StatePaused:
		if isLocal {
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Spc", T("hint_resume")},
				{"n/p", T("hint_next_prev")},
				{"</>", T("hint_seek")},
				{"m", T("hint_mode")},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel})
			}
			hints = append(hints,
				statusHint{"f", T("hint_fullscreen")},
				statusHint{`\`, collapseHintLabel},
				statusHint{"L", T("hint_like_toggle")},
				statusHint{"/", T("hint_search")},
				statusHint{",", T("hint_settings")},
				statusHint{"Tab", T("hint_switch_tab")},
				statusHint{"q", T("hint_quit")},
			)
		} else {
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Spc", T("hint_resume")},
				{"n/p", T("hint_next_prev")},
				{"</>", T("hint_seek")},
				{"m", T("hint_mode")},
			}
			if showLyricsHint {
				hints = append(hints, statusHint{"l", lyricsHintLabel})
			}
			hints = append(hints,
				statusHint{"f", T("hint_fullscreen")},
				statusHint{`\`, collapseHintLabel},
				statusHint{"a", T("hint_playlist_add_track")},
				statusHint{"x", T("hint_playlist_remove_track")},
				statusHint{"L", T("hint_like_toggle")},
				statusHint{",", T("hint_settings")},
				statusHint{"Esc", T("hint_back")},
				statusHint{"Tab", T("hint_switch_tab")},
				statusHint{"q", T("hint_quit")},
			)
		}

	default:
		if isLocal {
			// Local: ? Enter L / , Tab q
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Enter", T("hint_play")},
				{"L", T("hint_like_toggle")},
				{"/", T("hint_search")},
				{",", T("hint_settings")},
				{"Tab", T("hint_switch_tab")},
				{"q", T("hint_quit")},
			}
		} else {
			// Playlist: ? Enter a x L , Esc Tab q
			hints = []statusHint{
				{"?", T("hint_help")},
				{"Enter", T("hint_play")},
				{"a", T("hint_playlist_add_track")},
				{"x", T("hint_playlist_remove_track")},
				{"L", T("hint_like_toggle")},
				{",", T("hint_settings")},
				{"Esc", T("hint_back")},
				{"Tab", T("hint_switch_tab")},
				{"q", T("hint_quit")},
			}
		}
	}

	return hints
}
