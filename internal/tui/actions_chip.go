package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// chip8 command constants.
const (
	chip8Command       = "p2chip"
	chip8DefaultSF2    = "nes"
	chip8DefaultFormat = "mp3"
	chip8Timeout       = 10 * time.Minute
)

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
// playlistUndoWindow is how long a removed playlist track can be restored.
const playlistUndoWindow = 5 * time.Second
