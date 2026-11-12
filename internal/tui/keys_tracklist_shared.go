package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

)

// handleSharedTrackListKey is the test-facing entry point. It resolves the key
// through the registry and delegates to the action-based engine, preserving the
// supplied TrackListContext.
func (a *App) handleSharedTrackListKey(ctx TrackListContext, msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeLocalTracks, scopeTracklist, scopeGlobal}, msg, func(action ActionID, msg tea.KeyMsg) tea.Cmd {
		return a.handleSharedTrackListAction(ctx, action, msg)
	})
}

// handleSharedTrackListAction is the unified action handler for any tab whose
// main panel is a scrollable track list. Navigation, playback and panel actions
// come from the registry; context-specific actions go to ctx.OnExtraKeys.
func (a *App) handleSharedTrackListAction(ctx TrackListContext, action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionPlay:
		n := ctx.Count()
		if n == 0 {
			return nil
		}
		pos := *ctx.CursorPos
		if pos < 0 || pos >= n {
			return nil
		}
		// A second play action on the current track opens the fullscreen player.
		if a.currentTrack != nil && a.currentTrack.ID == ctx.TrackAt(pos).ID {
			a.currentView = viewFullscreen
			return nil
		}
		return ctx.OnEnter(a, pos)

	case actionPause:
		return a.cmdTogglePause()

	case actionNavDown:
		if *ctx.CursorPos < ctx.Count()-1 {
			*ctx.CursorPos++
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}
	case actionNavUp:
		if *ctx.CursorPos > 0 {
			*ctx.CursorPos--
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}
	case actionNavPageDown:
		moveCursor(a, ctx, ctx.PageSize())
	case actionNavPageUp:
		moveCursor(a, ctx, -ctx.PageSize())
	case actionNavTop:
		*ctx.CursorPos = 0
		if ctx.UseMarquee {
			syncMarquee(a, ctx)
		}
	case actionNavBottom:
		if n := ctx.Count(); n > 0 {
			*ctx.CursorPos = n - 1
			if ctx.UseMarquee {
				syncMarquee(a, ctx)
			}
		}
	case actionJumpPlaying:
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

	case actionQueueAppend:
		return a.enqueueCursor(ctx, false)
	case actionQueueInsert:
		return a.enqueueCursor(ctx, true)

	case actionClearFilter:
		a.clearSearchFilter(ctx)
	case actionPlaylistBack, actionGroupBack, actionCancel:
		// Cancel cascades: drilled-in group first, then a visible filter, then
		// the context-specific escape action.
		if action != actionPlaylistBack && a.drilldown != nil {
			a.exitDrilldown()
			return nil
		}
		if action != actionPlaylistBack && ctx.ShowSearchBar && strings.TrimSpace(a.searchInput.Value()) != "" {
			a.clearSearchFilter(ctx)
			return nil
		}
		if ctx.OnEsc != nil {
			ctx.OnEsc(a)
		}
		return nil

	default:
		if ctx.OnExtraKeys != nil {
			if handled, cmd := ctx.OnExtraKeys(a, action, msg); handled {
				return cmd
			}
		}
	}
	return nil
}

// clearSearchFilter resets the inline search and restores the full list.
func (a *App) clearSearchFilter(ctx TrackListContext) {
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
}

// moveCursor moves the cursor by delta rows, clamped to the list bounds.
func moveCursor(a *App, ctx TrackListContext, delta int) {
	n := ctx.Count()
	if n == 0 {
		return
	}
	pos := *ctx.CursorPos + delta
	if pos < 0 {
		pos = 0
	}
	if pos >= n {
		pos = n - 1
	}
	*ctx.CursorPos = pos
	if ctx.UseMarquee {
		syncMarquee(a, ctx)
	}
}

func syncMarquee(a *App, ctx TrackListContext) {
	if ctx.SyncMarquee != nil {
		ctx.SyncMarquee(a)
		return
	}
	a.syncRowMarquee()
}
