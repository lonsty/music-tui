package lyrics

import (
	"context"

	"github.com/eilianxiao/music-tui/internal/library"
)

// ChainProvider tries each Provider in order and returns the first non-nil
// result.  If a provider returns an error it is skipped (online failures
// should not block display of locally available lyrics).
type ChainProvider struct {
	Providers []Provider
}

// Fetch implements Provider.
func (c *ChainProvider) Fetch(ctx context.Context, track library.Track) ([]Line, error) {
	for _, p := range c.Providers {
		lines, err := p.Fetch(ctx, track)
		if err != nil {
			// Online provider failed — log silently and try next.
			continue
		}
		if lines != nil {
			return lines, nil
		}
	}
	return nil, nil
}
