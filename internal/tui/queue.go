package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/library"
)

// findTrackByID returns a copy of the track with the given ID, or nil.
func (a *App) findTrackByID(id string) *library.Track {
	pos := a.trackPos(id)
	if pos < 0 {
		return nil
	}
	t := a.tracks[pos]
	return &t
}

func (a *App) queueHas(id string) bool {
	for _, qid := range a.ids {
		if qid == id {
			return true
		}
	}
	return false
}

// enqueue inserts id at the front (play next) or back of the queue.
// It returns false when the track is already queued.
func (a *App) enqueue(id string, front bool) bool {
	if a.queueHas(id) {
		return false
	}
	if front {
		a.ids = append([]string{id}, a.ids...)
	} else {
		a.ids = append(a.ids, id)
	}
	return true
}

// enqueueCursor adds the track under the list cursor to the queue.
func (a *App) enqueueCursor(ctx TrackListContext, front bool) tea.Cmd {
	t := ctx.TrackAt(*ctx.CursorPos)
	if t.ID == "" {
		return nil
	}
	if !a.enqueue(t.ID, front) {
		a.statusMsg = T("queue_duplicate")
		a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
		return nil
	}
	if front {
		a.statusMsg = T("queue_added_next")
	} else {
		a.statusMsg = T("queue_added_back")
	}
	a.statusMsgExpiry = time.Now().Add(statusMsgDuration)
	return nil
}

// popPlayableQueueID removes and returns the first queued track that still
// exists in the library.  Returns "" when the queue is exhausted.
func (a *App) popPlayableQueueID() string {
	for len(a.ids) > 0 {
		id := a.ids[0]
		a.ids = a.ids[1:]
		if a.findTrackByID(id) != nil {
			return id
		}
	}
	a.clampQueueCursor()
	return ""
}

func (a *App) clampQueueCursor() {
	if a.cursor >= len(a.ids) {
		a.cursor = len(a.ids) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// handleQueueKey is the test-facing entry point for the queue overlay.
func (a *App) handleQueueKey(msg tea.KeyMsg) tea.Cmd {
	return a.runAction([]Scope{scopeOverlayQueue, scopeGlobal}, msg, a.handleQueueAction)
}

// handleQueueAction handles queue-overlay actions.
func (a *App) handleQueueAction(action ActionID, msg tea.KeyMsg) tea.Cmd {
	switch action {
	case actionQueueClose, actionCancel:
		a.closeOverlay()
	case actionNavDown:
		if a.cursor < len(a.ids)-1 {
			a.cursor++
			a.keepQueueCursorVisible()
		}
	case actionNavUp:
		if a.cursor > 0 {
			a.cursor--
			a.keepQueueCursorVisible()
		}
	case actionNavTop:
		a.cursor = 0
		a.keepQueueCursorVisible()
	case actionNavBottom:
		if len(a.ids) > 0 {
			a.cursor = len(a.ids) - 1
			a.keepQueueCursorVisible()
		}
	case actionQueueRemove:
		if len(a.ids) > 0 {
			a.clampQueueCursor()
			a.ids = append(a.ids[:a.cursor], a.ids[a.cursor+1:]...)
			a.clampQueueCursor()
			a.keepQueueCursorVisible()
		}
	case actionQueueClear:
		a.ids = nil
		a.cursor = 0
		a.ovlScrollRow = 0
	}
	return nil
}

// keepQueueCursorVisible scrolls the overlay body so the queue cursor stays
// inside the visible rows.
func (a *App) keepQueueCursorVisible() {
	a.scrollOverlayToCursor(a.cursor)
}
