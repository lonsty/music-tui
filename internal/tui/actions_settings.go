package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lonsty/music-tui/internal/library"
)

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
	st := a.st
	return func() tea.Msg {
		defer cancel()
		lines, err := p.Fetch(ctx, track)
		return lyricsLoadedMsg{
			trackID: id,
			lines:   lines,
			offset:  loadLyricsOffset(st, id),
			err:     err,
		}
	}
}
