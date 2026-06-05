package tui

import (
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
		tracks := ctx.Tracks()
		if *ctx.CursorPos < len(tracks)-1 {
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

	case "g":
		*ctx.CursorPos = 0
		if ctx.UseMarquee {
			syncMarquee(a, ctx)
		}

	case "G":
		tracks := ctx.Tracks()
		if len(tracks) > 0 {
			*ctx.CursorPos = len(tracks) - 1
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}

	// ── Playback ─────────────────────────────────────────────────────────────
	case "enter":
		tracks := ctx.Tracks()
		if len(tracks) == 0 {
			return nil
		}
		pos := *ctx.CursorPos
		if pos < 0 || pos >= len(tracks) {
			return nil
		}
		// Second Enter on the currently-playing track → open fullscreen.
		if a.currentTrack != nil && a.currentTrack.ID == tracks[pos].ID {
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
			a.searchInput.SetValue("")
			a.applyFilter()
			a.syncRowMarquee()
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
			if len(a.playlists) == 0 {
				return a.cmdLoadPlaylists()
			}
			// Sync playlist row marquee when switching TO the Playlists tab,
			// so the selected row shows the correct playlist name immediately.
			a.syncPlaylistRowMarquee()
			// When at track level, anchor the cursor to the currently playing
			// track (matching cmus/ncmpcpp behaviour) and sync the track row
			// marquee so the selected row renders the correct text immediately.
			if a.depth == playlistDepthTracks {
				a.anchorPlaylistTrackCursor()
				a.syncPlaylistTrackMarquee()
			}
		}

	// ── Esc ───────────────────────────────────────────────────────────────────
	case "esc":
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

	var hints []statusHint
	switch displayState {
	case audio.StatePlaying:
		hints = []statusHint{
			{"Spc", T("hint_pause"), 1},
			{"q", T("hint_quit"), 1},
			{"n", T("hint_next"), 2},
			{"p", T("hint_prev"), 2},
			{"</>", T("hint_seek"), 3},
			{"+/-", T("hint_vol"), 3},
		}
		if showLyricsHint {
			hints = append(hints, statusHint{"l", lyricsHintLabel, 4})
		}
		hints = append(hints, statusHint{`\`, collapseHintLabel, 4})
		if ctx.ShowSearchBar {
			hints = append(hints, statusHint{"/", T("hint_search"), 5})
		}
		hints = append(hints, statusHint{"?", T("hint_help"), 5})

	case audio.StatePaused:
		hints = []statusHint{
			{"Spc", T("hint_resume"), 1},
			{"q", T("hint_quit"), 1},
			{"n", T("hint_next"), 2},
			{"p", T("hint_prev"), 2},
			{"</>", T("hint_seek"), 3},
			{"+/-", T("hint_vol"), 3},
		}
		if showLyricsHint {
			hints = append(hints, statusHint{"l", lyricsHintLabel, 4})
		}
		hints = append(hints, statusHint{`\`, collapseHintLabel, 4})
		if ctx.ShowSearchBar {
			hints = append(hints, statusHint{"/", T("hint_search"), 5})
		}
		hints = append(hints, statusHint{"?", T("hint_help"), 5})

	default:
		// No track loaded (StateStopped with no currentTrack).
		hints = []statusHint{
			{"Enter", T("hint_play"), 1},
			{"q", T("hint_quit"), 1},
		}
		if ctx.ShowSearchBar {
			hints = append(hints, statusHint{"/", T("hint_search"), 2})
		}
		hints = append(hints,
			statusHint{`\`, collapseHintLabel, 3},
			statusHint{",", T("hint_settings"), 3},
			statusHint{"?", T("hint_help"), 4},
		)
	}

	// Append context-specific extra hints.
	if ctx.ExtraHints != nil {
		hints = append(hints, ctx.ExtraHints(a)...)
	}

	return hints
}
