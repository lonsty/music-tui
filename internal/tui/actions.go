package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
	"github.com/eilianxiao/music-tui/internal/store"
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

// ── Library sync ──────────────────────────────────────────────────────────────

// cmdSyncLibrary runs an incremental sync of a.musicDir against the database,
// then reloads the full track list and sends a scanDoneMsg.
func (a *App) cmdSyncLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
		return nil
	}
	dir := a.musicDir
	st := a.st
	return func() tea.Msg {
		coverDir, _ := store.CoverCacheDir()
		_, _, _, err := store.SyncDir(context.Background(), dir, st, coverDir, nil)
		if err != nil {
			// Non-fatal: continue and return whatever is in the DB.
			_ = err
		}
		tracks, dbErr := st.AllTracks()
		if dbErr != nil {
			return scanDoneMsg{err: dbErr}
		}
		return scanDoneMsg{tracks: tracks}
	}
}

// ── 8-bit chip mode ───────────────────────────────────────────────────────────

// cmdToggleChip toggles the 8-bit conversion mode for the current track.
//
// State machine:
//   - chipBusy == true  → no-op (conversion or crossfade already in progress)
//   - chipMode == true  → crossfade back to the original track, clear chipMode
//   - chipMode == false → convert (if needed) then crossfade to 8-bit version
func (a *App) cmdToggleChip() tea.Cmd {
	// Locked — ignore repeat presses.
	if a.chipBusy {
		return nil
	}
	if a.currentTrack == nil {
		return nil
	}

	a.chipBusy = true
	// chipConverting is set only when we actually invoke p2chip.
	// Cache-hit and crossfade-back paths leave it false.

	if a.chipMode {
		// ── Turn off: crossfade back to original ──────────────────────────
		originPath := a.currentTrack.Path
		return func() tea.Msg {
			pos := a.player.Position()
			_ = a.player.CrossfadeTo(originPath, pos)
			a.chipMode = false
			a.chipBusy = false
			return noopMsg{}
		}
	}

	// ── Turn on ───────────────────────────────────────────────────────────
	track := *a.currentTrack

	// Cache hit — crossfade immediately without re-converting.
	if a.chipOrigin == track.Path && a.chipPath != "" {
		cachedPath := a.chipPath
		return func() tea.Msg {
			pos := a.player.Position()
			_ = a.player.CrossfadeTo(cachedPath, pos)
			a.chipMode = true
			a.chipBusy = false
			return noopMsg{}
		}
	}

	// No cache — run p2chip in the background.
	a.chipConverting = true
	outPath := filepath.Join(a.tmpDir, chip8CacheKey(track.Path)+".mp3")
	extraOpts := a.chip8Options // capture before goroutine
	return func() tea.Msg {
		args := []string{track.Path, outPath, "--format", "mp3"}
		if extraOpts != "" {
			parsed, _ := shellSplit(extraOpts)
			args = append(args, parsed...)
		} else {
			// Default preset when no custom options are given.
			args = append(args, "--sf2", "nes")
		}
		cmd := exec.Command("p2chip", args...)
		if err := cmd.Run(); err != nil {
			return chip8DoneMsg{err: fmt.Errorf("p2chip: %w", err)}
		}
		return chip8DoneMsg{originPath: track.Path, chipPath: outPath}
	}
}

// chip8CacheKey returns a short hex string derived from path and options,
// used as the cache file name for the converted 8-bit mp3.
func chip8CacheKey(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
}

// shellSplit splits a string into tokens the same way a POSIX shell would,
// respecting single-quoted and double-quoted sub-strings.
//
// It handles the common cases needed for p2chip option strings like:
//
//	--sf2 nes --onset 0.6 --trim "0:30"
//
// Returns a nil error; malformed quoting is handled leniently (unclosed
// quotes are treated as if closed at end-of-string).
func shellSplit(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			} else {
				cur.WriteByte(c)
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			} else {
				cur.WriteByte(c)
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == ' ' || c == '\t':
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}
