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

// resolveSource returns the StreamSource for track by consulting providerMap.
// If the track's ProviderID has a registered TrackProvider, that provider's
// StreamSource() is called.  Otherwise a LocalSource is returned as the default.
func (a *App) resolveSource(ctx context.Context, track library.Track) (audio.StreamSource, error) {
	if p, ok := a.providerMap[track.ProviderID]; ok && p != nil {
		return p.StreamSource(ctx, track)
	}
	return audio.LocalSource{Path: track.Path}, nil
}

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
		src, err := a.resolveSource(context.Background(), track)
		if err != nil {
			return playResultMsg{err: err}
		}
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
	track := a.filteredTrack(pos) // value copy — safe across goroutine boundary
	return func() tea.Msg {
		src, err := a.resolveSource(context.Background(), track)
		if err != nil {
			return playResultMsg{err: err}
		}
		if err := a.player.PlaySource(src); err != nil {
			return playResultMsg{err: err}
		}
		t := track
		return playResultMsg{track: &t}
	}
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
			return nil // reached end, stop
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

// applyFilter rebuilds a.filtered from a.tracks using the current search query
// and the active format preference.
//
// Search prefix syntax:
//
//	s:KEY  — search artist (singer)
//	a:KEY  — search album
//	t:KEY  — search title
//	f:KEY  — filter by audio format (e.g. f:flac, f:mp3)
//	KEY    — search all text fields
//
// After text filtering, applyFormatPreference is applied to deduplicate or
// hide tracks based on the active formatPreference setting.
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
	case strings.HasPrefix(raw, "f:"):
		field = "format"
		q = strings.ToLower(strings.TrimPrefix(raw, "f:"))
	default:
		q = strings.ToLower(raw)
	}

	var candidate []library.Track
	if q == "" {
		candidate = a.tracks
	} else {
		for _, t := range a.tracks {
			var match bool
			switch field {
			case "artist":
				match = strings.Contains(strings.ToLower(t.DisplayArtist()), q)
			case "album":
				match = strings.Contains(strings.ToLower(t.Album), q)
			case "title":
				match = strings.Contains(strings.ToLower(t.DisplayTitle()), q)
			case "format":
				match = strings.Contains(strings.ToLower(t.Format()), q)
			default:
				match = strings.Contains(strings.ToLower(t.DisplayTitle()), q) ||
					strings.Contains(strings.ToLower(t.DisplayArtist()), q) ||
					strings.Contains(strings.ToLower(t.Album), q)
			}
			if match {
				candidate = append(candidate, t)
			}
		}
	}

	// Build the filtered index table from the candidate slice.
	// candidate is already a subset of a.tracks (same Track values), so we
	// look up each candidate's position in a.tracks by ID.
	idToTrackPos := make(map[string]int, len(a.tracks))
	for i := range a.tracks {
		idToTrackPos[a.tracks[i].ID] = i
	}

	preferred := applyFormatPreference(candidate, a.formatPref)
	newIdxs := make([]int, 0, len(preferred))
	for _, t := range preferred {
		if pos, ok := idToTrackPos[t.ID]; ok {
			newIdxs = append(newIdxs, pos)
		}
	}
	a.filteredIdxs = newIdxs

	// Invalidate shuffle so it is rebuilt from the new filtered list on next use.
	a.shuffleIDs = nil

	// Clamp cursor to the new filtered length.
	if len(a.filteredIdxs) == 0 {
		a.cursorPos = 0
	} else if a.cursorPos >= len(a.filteredIdxs) {
		a.cursorPos = len(a.filteredIdxs) - 1
	}
}

// applyFormatPreference filters or deduplicates tracks according to pref.
//
//   - formatPrefAll: returns tracks unchanged.
//   - formatPrefLosslessFirst: for each (artist, album, title) group keeps
//     only the copy with the highest format quality score.  Tracks whose
//     group has no other members are always included regardless of format.
//   - formatPrefLosslessOnly: removes any track whose format quality score
//     is below the lowest lossless threshold (WAV/FLAC score ≥ 30).
//   - formatPrefMP3Only: keeps only MP3 tracks.
//
// The original ordering is preserved.
func applyFormatPreference(tracks []library.Track, pref formatPreference) []library.Track {
	switch pref {
	case formatPrefAll:
		return tracks

	case formatPrefLosslessFirst:
		// Group tracks by their dedup key: lowercase (artist · album · title).
		// For each group, remember the index of the highest-quality track seen.
		type entry struct {
			idx     int
			quality int
		}
		best := make(map[string]entry, len(tracks))
		for i, t := range tracks {
			key := strings.ToLower(t.DisplayArtist()) + "\x00" +
				strings.ToLower(t.Album) + "\x00" +
				strings.ToLower(t.DisplayTitle())
			q := library.QualityOf(t.Format())
			if prev, ok := best[key]; !ok || q > prev.quality {
				best[key] = entry{idx: i, quality: q}
			}
		}
		// Collect winners in original order using an index set.
		keep := make(map[int]bool, len(best))
		for _, e := range best {
			keep[e.idx] = true
		}
		out := make([]library.Track, 0, len(best))
		for i, t := range tracks {
			if keep[i] {
				out = append(out, t)
			}
		}
		return out

	case formatPrefLosslessOnly:
		// lossless threshold: quality score ≥ 30 (WAV=30, FLAC=40).
		const losslessThreshold = 30
		out := tracks[:0]
		for _, t := range tracks {
			if library.QualityOf(t.Format()) >= losslessThreshold {
				out = append(out, t)
			}
		}
		return out

	case formatPrefMP3Only:
		out := tracks[:0]
		for _, t := range tracks {
			if strings.EqualFold(t.Format(), "MP3") {
				out = append(out, t)
			}
		}
		return out

	default:
		return tracks
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

// lyricsNetTimeout is the maximum time allowed for an online lyrics fetch.
// Local file lookups complete immediately; this guard applies to HTTP calls.
const lyricsNetTimeout = 10 * time.Second

// cmdLoadLyrics asynchronously loads lyrics for track using the configured
// provider chain (local files → cached lrclib.net).
//
// Any in-flight request started by a previous call is cancelled before the new
// one begins, preventing stale responses from polluting the current track state.
// The result is delivered via lyricsLoadedMsg; lines is nil when nothing found.
func (a *App) cmdLoadLyrics(track library.Track) tea.Cmd {
	// Cancel the previous in-flight request before starting a new one.
	a.netCancel()

	ctx, cancel := context.WithTimeout(context.Background(), lyricsNetTimeout)
	a.netCancel = cancel

	id := track.ID
	p := a.provider // capture; safe — provider is read-only after NewApp
	return func() tea.Msg {
		defer cancel()
		lines, err := p.Fetch(ctx, track)
		return lyricsLoadedMsg{trackID: id, lines: lines, err: err}
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
