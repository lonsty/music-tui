package lyrics

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lonsty/music-tui/internal/library"
)

const (
	// cacheTTL is how long a cached lyrics result is considered fresh.
	cacheTTL = 30 * 24 * time.Hour
)

// cacheEntry is the on-disk JSON representation of a cached lyrics result.
type cacheEntry struct {
	// FetchedAt is when the result was stored (Unix seconds).
	FetchedAt int64 `json:"fetched_at"`
	// Lines holds the serialised lyrics.  nil means "no lyrics available"
	// (we cache the negative result to avoid hammering the API).
	Lines []cachedLine `json:"lines"`
}

// cachedLine is a JSON-serialisable version of Line.
type cachedLine struct {
	TimeMS int64  `json:"t"` // milliseconds
	Text   string `json:"s"`
}

// CachedProvider wraps another Provider and caches results to cacheDir.
// A nil result (no lyrics) is also cached so the API is not queried again
// for the same track within the TTL window.
type CachedProvider struct {
	inner    Provider
	cacheDir string
}

// NewCachedProvider wraps inner, storing cache files under cacheDir.
// Call store.LyricsCacheDir() to get the standard cache directory.
func NewCachedProvider(inner Provider, cacheDir string) *CachedProvider {
	return &CachedProvider{inner: inner, cacheDir: cacheDir}
}

// Fetch implements Provider.
func (c *CachedProvider) Fetch(ctx context.Context, track library.Track) ([]Line, error) {
	key := cacheKey(track)
	path := filepath.Join(c.cacheDir, key+".json")

	// Try to load from cache.
	if lines, ok := c.load(path); ok {
		return lines, nil
	}

	// Cache miss — fetch from the wrapped provider.
	lines, err := c.inner.Fetch(ctx, track)
	if err != nil {
		// Don't cache errors; let the caller retry next time.
		return nil, err
	}

	// Persist the result (including nil → "no lyrics").
	c.save(path, lines)
	return lines, nil
}

// load reads and validates a cache entry.
// Returns (lines, true) on a valid, non-expired hit.
// Returns (nil, false) on miss, expiry, or any read/parse error.
func (c *CachedProvider) load(path string) ([]Line, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false // file missing or unreadable
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false // corrupted cache
	}

	age := time.Since(time.Unix(entry.FetchedAt, 0))
	if age > cacheTTL {
		_ = os.Remove(path) // proactively evict stale entry
		return nil, false
	}

	// Reconstruct Lines.  entry.Lines == nil means "no lyrics" was cached.
	if entry.Lines == nil {
		return nil, true // valid negative cache
	}
	out := make([]Line, len(entry.Lines))
	for i, cl := range entry.Lines {
		out[i] = Line{
			Time: time.Duration(cl.TimeMS) * time.Millisecond,
			Text: cl.Text,
		}
	}
	return out, true
}

// save writes a cache entry to path, creating parent directories as needed.
// Errors are silently ignored — cache write failures are non-fatal.
func (c *CachedProvider) save(path string, lines []Line) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	entry := cacheEntry{FetchedAt: time.Now().Unix()}
	if lines != nil {
		entry.Lines = make([]cachedLine, len(lines))
		for i, l := range lines {
			entry.Lines[i] = cachedLine{
				TimeMS: l.Time.Milliseconds(),
				Text:   l.Text,
			}
		}
	}
	// entry.Lines stays nil when lines == nil (negative cache).

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Write to a temp file then rename for atomicity.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// cacheKey derives a stable filename from track metadata.
// We use sha256(trackID) when a stable ID exists; otherwise we hash the
// title+artist string so searches for the same song always share a cache entry.
func cacheKey(track library.Track) string {
	s := track.ID
	if s == "" {
		s = track.DisplayTitle() + "\x00" + track.DisplayArtist()
	}
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:16]) // 32-char hex, collision-resistant
}
