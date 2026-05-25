// Package provider defines the abstraction for music track sources.
//
// A TrackProvider represents a single origin of music tracks, such as the
// local filesystem, Netease Cloud Music, or QQ Music.  Adding a new source
// only requires implementing this interface and registering the provider —
// no changes to the player or TUI layer are needed.
package provider

import (
	"context"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
)

// TrackProvider is the unified abstraction over music track sources.
type TrackProvider interface {
	// ID returns a stable, URL-safe identifier for this provider (e.g. "local", "netease").
	// It is used as the value of Track.ProviderID for tracks from this source.
	ID() string

	// Search queries the provider for tracks matching query.
	// page is 0-indexed; pageSize is the maximum number of results to return.
	Search(ctx context.Context, query string, page, pageSize int) ([]library.Track, error)

	// StreamSource returns a StreamSource that can open an audio stream for track.
	StreamSource(ctx context.Context, track library.Track) (audio.StreamSource, error)

	// SyncLibrary performs an incremental sync of the provider's track catalogue
	// into the local database.  progress is called after each file is processed
	// with (filesProcessed, totalFiles); it may be nil.
	// Returns nil when the provider does not support local sync (e.g. streaming-only).
	SyncLibrary(ctx context.Context, progress func(done, total int)) error
}
