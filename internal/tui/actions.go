package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/store"
)

// chip8 command constants.
const (
	chip8Command       = "p2chip"
	chip8DefaultSF2    = "nes"
	chip8DefaultFormat = "mp3"
	chip8Timeout       = 10 * time.Minute
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

	idx := -1
	for i, t := range a.filtered {
		if t.Path == sess.LastTrackPath {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}

	track := a.filtered[idx]
	offsetDur := time.Duration(sess.LastPositionMs) * time.Millisecond

	return func() tea.Msg {
		// PlayAt opens the file and seeks before handing to the speaker,
		// so the position is accurate from the very first frame.
		if err := a.player.PlayAt(track, offsetDur); err != nil {
			return playResultMsg{err: err, idx: idx}
		}
		// Start paused — user presses Space to resume.
		a.player.Pause()
		t := track
		return playResultMsg{track: &t, idx: idx}
	}
}

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

	// If the current track is not in filtered (e.g. a search hid it),
	// clamp the base index so next/prev stays within bounds.
	baseIdx := a.currentIdx
	if baseIdx < 0 || baseIdx >= len(a.filtered) {
		baseIdx = 0
	}
	var next int
	switch a.playMode {
	case playModeSingle:
		next = baseIdx

	case playModeRandom:
		if len(a.shuffleOrder) == 0 {
			a.rebuildShuffle()
		}
		// Find baseIdx position in shuffle order and advance.
		pos := 0
		for i, v := range a.shuffleOrder {
			if v == baseIdx {
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
		next = (baseIdx + 1) % len(a.filtered)

	default: // playModeSequential
		next = baseIdx + 1
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
	baseIdx := a.currentIdx
	if baseIdx < 0 || baseIdx >= len(a.filtered) {
		baseIdx = 0
	}
	prev := (baseIdx - 1 + len(a.filtered)) % len(a.filtered)
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

// cmdNextPlayMode advances playMode to the next value and rebuilds shuffle
// order if switching into random mode.
func (a *App) cmdNextPlayMode() tea.Cmd {
	return func() tea.Msg {
		a.playMode = (a.playMode + 1) % playModeCount
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

	// Re-anchor currentIdx so next/prev navigation stays correct after the
	// filtered list changes.  The cursor clamp is a separate concern.
	if a.currentTrack != nil {
		newIdx := -1
		for i, t := range a.filtered {
			if t.ID == a.currentTrack.ID {
				newIdx = i
				break
			}
		}
		if newIdx >= 0 {
			a.currentIdx = newIdx
		}
		// If the playing track is not in the filtered list, currentIdx is left
		// unchanged (it points outside filtered); cmdPlayNext/Prev will handle
		// the boundary naturally since they operate on len(a.filtered).
	}

	if a.cursor >= len(a.filtered) {
		a.cursor = max(0, len(a.filtered)-1)
	}
}

// ── Volume ────────────────────────────────────────────────────────────────────

// volumeStep is the amount by which volume changes on each key press.
const volumeStep = 0.1

// maxVolume is the upper limit of the volume knob (2.0 = +6 dB boost above unity gain).
const maxVolume = 2.0

// clampVolume keeps v in [0.0, maxVolume].
func clampVolume(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > maxVolume {
		return maxVolume
	}
	return v
}

// adjustVolume changes the volume by delta, clamps it, and applies it to the player.
func (a *App) adjustVolume(delta float64) {
	a.volume = clampVolume(a.volume + delta)
	a.player.SetVolume(a.volume)
}

// ── Lyrics ────────────────────────────────────────────────────────────────────

// cmdLoadLyrics asynchronously loads lyrics for track using the configured
// provider chain (local files → cached lrclib.net).
// The result is delivered via lyricsLoadedMsg; lines is nil when nothing found.
func (a *App) cmdLoadLyrics(track library.Track) tea.Cmd {
	id := track.ID
	p := a.provider // capture; safe — provider is read-only after NewApp
	return func() tea.Msg {
		lines, _ := p.Fetch(context.Background(), track)
		return lyricsLoadedMsg{trackID: id, lines: lines}
	}
}

// ── Library sync ──────────────────────────────────────────────────────────────

// cmdSyncLibrary runs an incremental sync of a.musicDir against the database,
// then reloads the full track list and sends a scanDoneMsg.
// Any sync error is surfaced in the status bar via scanDoneMsg.err.
func (a *App) cmdSyncLibrary() tea.Cmd {
	if a.st == nil || a.musicDir == "" {
		return nil
	}
	dir := a.musicDir
	st := a.st
	return func() tea.Msg {
		coverDir, _ := store.CoverCacheDir()
		_, _, _, syncErr := store.SyncDir(context.Background(), dir, st, coverDir, nil)
		tracks, dbErr := st.AllTracks()
		if dbErr != nil {
			return scanDoneMsg{err: dbErr}
		}
		// Return tracks even when syncErr is non-nil: we show whatever is in
		// the DB and surface the sync error in the status bar.
		if len(tracks) == 0 {
			tracks = []library.Track{} // always non-nil slice
		}
		return scanDoneMsg{tracks: tracks, err: syncErr}
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
			// State mutations happen in Update via chipCrossfadeDoneMsg,
			// not here — tea.Cmd goroutines must not write App fields directly.
			return chipCrossfadeDoneMsg{chipMode: false}
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
			return chipCrossfadeDoneMsg{chipMode: true}
		}
	}

	// No cache — run p2chip in the background.
	a.chipConverting = true
	outPath := filepath.Join(a.tmpDir, chip8CacheKey(track.Path)+"."+chip8DefaultFormat)
	extraOpts := a.chip8Options // capture before goroutine
	return func() tea.Msg {
		args := []string{track.Path, outPath, "--format", chip8DefaultFormat}
		if extraOpts != "" {
			parsed := shellSplit(extraOpts)
			args = append(args, parsed...)
		} else {
			args = append(args, "--sf2", chip8DefaultSF2)
		}
		// Use chip8Timeout so a stalled p2chip process doesn't block
		// the chip state forever.
		ctx, cancel := context.WithTimeout(context.Background(), chip8Timeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, chip8Command, args...)
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

// shellSplit splits a string into tokens the way a POSIX shell would,
// respecting single-quoted and double-quoted sub-strings.
// Malformed quoting (unclosed quotes) is handled leniently.
func shellSplit(s string) []string {
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
	return tokens
}
