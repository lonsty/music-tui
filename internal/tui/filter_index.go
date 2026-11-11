package tui

import (
	"math/rand"

	"github.com/lonsty/music-tui/internal/library"
)

// ── Filter / index helpers ────────────────────────────────────────────────────

// filteredLen returns the number of tracks that pass the current filter.
func (a *App) filteredLen() int { return len(a.filteredIdxs) }

// filteredTrack returns the track at position pos in the filtered list.
// Returns a zero Track if pos is out of range.
func (a *App) filteredTrack(pos int) library.Track {
	if pos < 0 || pos >= len(a.filteredIdxs) {
		return library.Track{}
	}
	return a.tracks[a.filteredIdxs[pos]]
}

// rebuildFilteredPos rebuilds the ID → filtered-position cache.
func (a *App) rebuildFilteredPos() {
	a.posByID = make(map[string]int, len(a.filteredIdxs))
	for pos, idx := range a.filteredIdxs {
		a.posByID[a.tracks[idx].ID] = pos
	}
}

// identityIndices returns the cached 0..len(tracks)-1 table used when no
// filter is active.  Callers must treat the result as read-only.
func (a *App) identityIndices() []int {
	n := len(a.tracks)
	if cap(a.identityIdxs) < n {
		a.identityIdxs = make([]int, n)
	} else {
		a.identityIdxs = a.identityIdxs[:n]
	}
	for i := range a.identityIdxs {
		a.identityIdxs[i] = i
	}
	return a.identityIdxs
}

// rebuildTrackPos rebuilds the ID → full-library-position cache used by queue
// and overlay lookups.  Call it whenever a.tracks is replaced.
func (a *App) rebuildTrackPos() {
	a.trackPosByID = make(map[string]int, len(a.tracks))
	for i := range a.tracks {
		a.trackPosByID[a.tracks[i].ID] = i
	}
}

// trackPos returns the position of id in a.tracks, or -1 when absent.
func (a *App) trackPos(id string) int {
	if len(a.trackPosByID) != len(a.tracks) {
		a.rebuildTrackPos()
	}
	if pos, ok := a.trackPosByID[id]; ok {
		return pos
	}
	return -1
}

// filteredPos returns the position of the track with the given ID in the
// filtered list, or -1 if it is not present.
func (a *App) filteredPos(id string) int {
	if a.posByID != nil {
		if pos, ok := a.posByID[id]; ok {
			return pos
		}
		return -1
	}
	for i, idx := range a.filteredIdxs {
		if a.tracks[idx].ID == id {
			return i
		}
	}
	return -1
}

// cursorTrack returns the track currently under the cursor.
// Returns nil when the filtered list is empty.
func (a *App) cursorTrack() *library.Track {
	if len(a.filteredIdxs) == 0 {
		return nil
	}
	pos := a.cursorPos
	if pos < 0 {
		pos = 0
	}
	if pos >= len(a.filteredIdxs) {
		pos = len(a.filteredIdxs) - 1
	}
	t := a.tracks[a.filteredIdxs[pos]]
	return &t
}

// ── Shuffle helpers ───────────────────────────────────────────────────────────

// rebuildShuffleIDs creates a fresh shuffled ID order from the current
// filtered list.  IDs are stable across subsequent filter changes; the
// cmdPlayNext logic skips any ID no longer present in filteredIdxs.
func (a *App) rebuildShuffleIDs() {
	ids := make([]string, len(a.filteredIdxs))
	for i, idx := range a.filteredIdxs {
		ids[i] = a.tracks[idx].ID
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	a.shuffleIDs = ids
}
