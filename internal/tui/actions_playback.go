package tui

import (
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
)

// ── Play commands ─────────────────────────────────────────────────────────────

// cmdRestoreSession loads the last-played track at the saved position and
// leaves the player paused.  Called once from Init().
func (a *App) cmdRestoreSession() tea.Cmd {
	if a.session == nil {
		return nil
	}
	sess := a.session
	a.session = nil // consume

	// Search the full library (a.tracks), not the filtered list, because the
	// last-played track might currently be hidden by the format preference.
	// Prefer the stable ID (stored since v0.4); fall back to path for older
	// session files that pre-date the ID field.
	var found *library.Track
	for i := range a.tracks {
		t := &a.tracks[i]
		if t.ID == sess.LastTrackID {
			tc := *t
			found = &tc
			break
		}
	}
	if found == nil {
		return nil
	}

	track := *found
	offsetDur := time.Duration(sess.LastPositionMs) * time.Millisecond

	return func() tea.Msg {
		src := audio.LocalSource{Path: track.Path}
		if err := a.player.PlaySourceAt(src, offsetDur); err != nil {
			return playResultMsg{err: err}
		}
		a.player.Pause()
		t := track
		return playResultMsg{track: &t}
	}
}

// cmdPlayTrack returns a Cmd that plays the track at position pos in the
// current filtered list.
// All App-state mutations happen via playResultMsg in Update — never in the Cmd.
func (a *App) cmdPlayTrack(pos int) tea.Cmd {
	if pos < 0 || pos >= a.filteredLen() {
		return nil
	}
	return a.cmdPlayTrackValue(a.filteredTrack(pos))
}

// cmdPlayTrackValue plays an explicit track value.  It is shared by list
// selection and queue playback; neither touches filteredIdxs.
func (a *App) cmdPlayTrackValue(track library.Track) tea.Cmd {
	return func() tea.Msg {
		src := audio.LocalSource{Path: track.Path}
		if err := a.player.PlaySource(src); err != nil {
			return playResultMsg{err: err}
		}
		t := track
		return playResultMsg{track: &t}
	}
}

// cmdPlayTrackByID plays a track by ID without changing the visible list.
func (a *App) cmdPlayTrackByID(id string) tea.Cmd {
	t := a.findTrackByID(id)
	if t == nil {
		return nil
	}
	return a.cmdPlayTrackValue(*t)
}

// cmdAdvance chooses the next track: queue first, then the current play mode.
func (a *App) cmdAdvance() tea.Cmd {
	if id := a.popPlayableQueueID(); id != "" {
		return a.cmdPlayTrackByID(id)
	}
	return a.cmdPlayNext()
}

// cmdPlayNext picks the next track according to the current playMode.
func (a *App) cmdPlayNext() tea.Cmd {
	n := a.filteredLen()
	if n == 0 {
		return nil
	}

	// Resolve the current playing position.  If the playing track is not in the
	// filtered list (e.g. a search hid it), fall back to position 0.
	basePos := 0
	if a.currentTrack != nil {
		if p := a.filteredPos(a.currentTrack.ID); p >= 0 {
			basePos = p
		}
	}

	var nextPos int
	switch a.playMode {
	case playModeSingle:
		nextPos = basePos

	case playModeRandom:
		if len(a.shuffleIDs) == 0 {
			a.rebuildShuffleIDs()
		}
		// Find the current track's position in shuffleIDs and advance.
		// Skip IDs that are no longer in the filtered list.
		curID := ""
		if a.currentTrack != nil {
			curID = a.currentTrack.ID
		}
		shufflePos := 0
		for i, id := range a.shuffleIDs {
			if id == curID {
				shufflePos = i
				break
			}
		}
		// Advance, wrapping with a reshuffle.
		shufflePos = (shufflePos + 1) % len(a.shuffleIDs)
		if shufflePos == 0 {
			rand.Shuffle(len(a.shuffleIDs), func(i, j int) {
				a.shuffleIDs[i], a.shuffleIDs[j] = a.shuffleIDs[j], a.shuffleIDs[i]
			})
		}
		// Walk forward until we find an ID present in the filtered list.
		for i := 0; i < len(a.shuffleIDs); i++ {
			id := a.shuffleIDs[(shufflePos+i)%len(a.shuffleIDs)]
			if p := a.filteredPos(id); p >= 0 {
				nextPos = p
				break
			}
		}

	case playModeLoop:
		nextPos = (basePos + 1) % n

	default: // playModeSequential
		nextPos = basePos + 1
		if nextPos >= n {
			// Reached the end of the list — stop playback and clear state.
			return func() tea.Msg { return playbackEndedMsg{} }
		}
	}

	return a.cmdPlayTrack(nextPos)
}

// cmdPlayPrev picks the previous track (ignores playModeRandom — always linear).
func (a *App) cmdPlayPrev() tea.Cmd {
	n := a.filteredLen()
	if n == 0 {
		return nil
	}
	basePos := 0
	if a.currentTrack != nil {
		if p := a.filteredPos(a.currentTrack.ID); p >= 0 {
			basePos = p
		}
	}
	prev := (basePos - 1 + n) % n
	return a.cmdPlayTrack(prev)
}

// ── Seek ──────────────────────────────────────────────────────────────────────

// seekStep is the seek distance applied by < and >.
const seekStep = 5 * time.Second

// browseFadeOutTicks is the number of tickInterval ticks after the last manual lyric
// scroll before the browse cursor automatically resets to follow playback.
// 10 ticks × tickInterval = 5 seconds.
const browseFadeOutTicks = 10

// cmdSeek moves the playback position by delta relative to the current
// position.  The result is clamped to [0, Duration].
func (a *App) cmdSeek(delta time.Duration) tea.Cmd {
	return func() tea.Msg {
		pos := a.player.Position()
		dur := a.player.Duration()
		target := pos + delta
		if target < 0 {
			target = 0
		}
		if dur > 0 && target > dur {
			target = dur
		}
		_ = a.player.Seek(target)
		return noopMsg{}
	}
}

// cmdSeekAndResume seeks to the given absolute position and ensures the player
// is running.  Used when the user selects a lyric line in browse mode.
func (a *App) cmdSeekAndResume(pos time.Duration) tea.Cmd {
	return func() tea.Msg {
		_ = a.player.Seek(pos)
		if a.player.State() != audio.StatePlaying {
			a.player.Resume()
		}
		return noopMsg{}
	}
}

// ── Pause toggle ──────────────────────────────────────────────────────────────

// cmdTogglePause runs TogglePause in a background goroutine.
func (a *App) cmdTogglePause() tea.Cmd {
	return func() tea.Msg {
		a.player.TogglePause()
		return noopMsg{}
	}
}

// ── Play-mode cycling ────────────────────────────────────────────────────────

// playModeChangedMsg carries the next play mode back to Update, which owns all
// App state writes.
type playModeChangedMsg struct{ mode playMode }

// retroChangedMsg carries the next Lo-Fi preset index back to Update.
type retroChangedMsg struct{ idx int }

// cmdNextPlayMode advances playMode to the next value.  The target is computed
// synchronously in Update; the returned Cmd carries it back as a message.
func (a *App) cmdNextPlayMode() tea.Cmd {
	next := (a.playMode + 1) % playModeCount
	return func() tea.Msg {
		return playModeChangedMsg{mode: next}
	}
}

// cmdRetroUp increases the retro effect preset by one step (lower sample rate).
func (a *App) cmdRetroUp() tea.Cmd {
	next := a.retroIdx
	if next < audio.RetroPresetCount-1 {
		next++
	}
	return func() tea.Msg {
		return retroChangedMsg{idx: next}
	}
}

// cmdRetroDown decreases the retro effect preset by one step (higher sample rate / off).
func (a *App) cmdRetroDown() tea.Cmd {
	next := a.retroIdx
	if next > 0 {
		next--
	}
	return func() tea.Msg {
		return retroChangedMsg{idx: next}
	}
}
