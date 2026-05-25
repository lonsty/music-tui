// Package local implements the TrackProvider interface for local filesystem music.
package local

import (
	"context"
	"fmt"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
	"github.com/eilianxiao/music-tui/internal/store"
)

// ProviderID is the stable identifier for the local filesystem provider.
const ProviderID = "local"

// Provider implements provider.TrackProvider for local filesystem music.
type Provider struct {
	st       *store.Store
	musicDir string
	coverDir string
}

// New creates a local Provider for the given music directory and store.
// coverDir is the directory where cover art is cached; pass "" to skip cover extraction.
func New(st *store.Store, musicDir, coverDir string) *Provider {
	return &Provider{st: st, musicDir: musicDir, coverDir: coverDir}
}

// ID implements provider.TrackProvider.
func (p *Provider) ID() string { return ProviderID }

// Search implements provider.TrackProvider by performing a case-insensitive
// substring search over the local track database.
// This is a simple in-memory filter; for large libraries a full-text index
// would be preferable.
func (p *Provider) Search(_ context.Context, query string, _, _ int) ([]library.Track, error) {
	tracks, err := p.st.AllTracks()
	if err != nil {
		return nil, fmt.Errorf("local search: %w", err)
	}
	if query == "" {
		return tracks, nil
	}
	var results []library.Track
	for _, t := range tracks {
		if library.MatchesQuery(t, query) {
			results = append(results, t)
		}
	}
	return results, nil
}

// StreamSource implements provider.TrackProvider.
func (p *Provider) StreamSource(_ context.Context, track library.Track) (audio.StreamSource, error) {
	if track.Path == "" {
		return nil, fmt.Errorf("local provider: track has no path")
	}
	return audio.LocalSource{Path: track.Path}, nil
}

// SyncLibrary implements provider.TrackProvider by running an incremental
// sync of the music directory against the database.
func (p *Provider) SyncLibrary(ctx context.Context, progress func(done, total int)) error {
	if p.musicDir == "" {
		return nil
	}
	_, _, _, err := store.SyncDir(ctx, p.musicDir, p.st, p.coverDir, progress)
	return err
}
