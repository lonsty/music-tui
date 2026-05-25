// Package lyrics provides types and interfaces for fetching and displaying lyrics.
package lyrics

import (
	"context"
	"time"

	"github.com/eilianxiao/music-tui/internal/library"
)

// Line represents a single timed lyric line (LRC format).
type Line struct {
	// Time is the playback offset at which this line should be displayed.
	Time time.Duration
	// Text is the lyric content for this line.
	Text string
}

// Provider is the unified abstraction over lyrics sources.
// Implementations include local .lrc files and online APIs.
type Provider interface {
	// Fetch retrieves lyrics for the given track.
	// Returns nil, nil when no lyrics are available (not an error).
	Fetch(ctx context.Context, track library.Track) ([]Line, error)
}
