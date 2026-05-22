package tui

import (
	"math/rand"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
)

// ── Play commands ─────────────────────────────────────────────────────────────

// cmdPlayTrack returns a Cmd that plays filtered[idx].
// All App-state mutations happen via playResultMsg in Update — never in the Cmd.
func (a *App) cmdPlayTrack(idx int) tea.Cmd {
	if idx < 0 || idx >= len(a.filtered) {
		return nil
	}
	track := a.filtered[idx] // value copy — safe across goroutine boundary
	return func() tea.Msg {
		if err := a.player.Play(track); err != nil {
			return playResultMsg{err: err, idx: idx}
		}
		t := track
		return playResultMsg{track: &t, idx: idx}
	}
}

// cmdPlayNext picks the next track according to the current playMode.
func (a *App) cmdPlayNext() tea.Cmd {
	if len(a.filtered) == 0 {
		return nil
	}

	var next int
	switch a.playMode {
	case playModeSingle:
		next = a.currentIdx

	case playModeRandom:
		if len(a.shuffleOrder) == 0 {
			a.rebuildShuffle()
		}
		// Find currentIdx position in shuffle order and advance.
		pos := 0
		for i, v := range a.shuffleOrder {
			if v == a.currentIdx {
				pos = i
				break
			}
		}
		pos = (pos + 1) % len(a.shuffleOrder)
		if pos == 0 {
			// Reshuffle when we wrap around so consecutive plays differ.
			rand.Shuffle(len(a.shuffleOrder), func(i, j int) {
				a.shuffleOrder[i], a.shuffleOrder[j] = a.shuffleOrder[j], a.shuffleOrder[i]
			})
		}
		next = a.shuffleOrder[pos]

	case playModeLoop:
		next = (a.currentIdx + 1) % len(a.filtered)

	default: // playModeSequential
		next = a.currentIdx + 1
		if next >= len(a.filtered) {
			return nil // reached end, stop
		}
	}

	return a.cmdPlayTrack(next)
}

// cmdPlayPrev picks the previous track (ignores playModeRandom — always linear).
func (a *App) cmdPlayPrev() tea.Cmd {
	if len(a.filtered) == 0 {
		return nil
	}
	prev := (a.currentIdx - 1 + len(a.filtered)) % len(a.filtered)
	return a.cmdPlayTrack(prev)
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

// cmdNextPlayMode advances playMode to the next value and rebuilds shuffle
// order if switching into random mode.
func (a *App) cmdNextPlayMode() tea.Cmd {
	return func() tea.Msg {
		a.playMode = (a.playMode + 1) % 4
		if a.playMode == playModeRandom {
			a.rebuildShuffle()
		}
		return noopMsg{}
	}
}

// cmdRetroUp increases the retro effect preset by one step (lower sample rate).
func (a *App) cmdRetroUp() tea.Cmd {
	return func() tea.Msg {
		if a.retroIdx < audio.RetroPresetCount-1 {
			a.retroIdx++
		}
		a.player.SetRetroPreset(a.retroIdx)
		return noopMsg{}
	}
}

// cmdRetroDown decreases the retro effect preset by one step (higher sample rate / off).
func (a *App) cmdRetroDown() tea.Cmd {
	return func() tea.Msg {
		if a.retroIdx > 0 {
			a.retroIdx--
		}
		a.player.SetRetroPreset(a.retroIdx)
		return noopMsg{}
	}
}

// ── Search / filter ──────────────────────────────────────────────────────────

// applyFilter rebuilds a.filtered from a.tracks using the current search query.
//
// Prefix syntax:
//
//	s:KEY  — search artist (singer)
//	a:KEY  — search album
//	t:KEY  — search title
//	KEY    — search all fields
func (a *App) applyFilter() {
	raw := strings.TrimSpace(a.searchInput.Value())

	var field, q string
	switch {
	case strings.HasPrefix(raw, "s:"):
		field = "artist"
		q = strings.ToLower(strings.TrimPrefix(raw, "s:"))
	case strings.HasPrefix(raw, "a:"):
		field = "album"
		q = strings.ToLower(strings.TrimPrefix(raw, "a:"))
	case strings.HasPrefix(raw, "t:"):
		field = "title"
		q = strings.ToLower(strings.TrimPrefix(raw, "t:"))
	default:
		q = strings.ToLower(raw)
	}

	if q == "" {
		a.filtered = a.tracks
	} else {
		var results []library.Track
		for _, t := range a.tracks {
			var match bool
			switch field {
			case "artist":
				match = strings.Contains(strings.ToLower(t.DisplayArtist()), q)
			case "album":
				match = strings.Contains(strings.ToLower(t.Album), q)
			case "title":
				match = strings.Contains(strings.ToLower(t.DisplayTitle()), q)
			default:
				match = strings.Contains(strings.ToLower(t.DisplayTitle()), q) ||
					strings.Contains(strings.ToLower(t.DisplayArtist()), q) ||
					strings.Contains(strings.ToLower(t.Album), q)
			}
			if match {
				results = append(results, t)
			}
		}
		a.filtered = results
	}

	a.rebuildShuffle()
	if a.cursor >= len(a.filtered) {
		a.cursor = max(0, len(a.filtered)-1)
	}
}

// ── Volume ────────────────────────────────────────────────────────────────────

// clampVolume keeps v in [0.0, 2.0].
func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 2.0 {
		return 2.0
	}
	return v
}
