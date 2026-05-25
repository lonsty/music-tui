// Package online provides lyrics providers that fetch from remote APIs.
package online

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lonsty/music-tui/internal/library"
	"github.com/lonsty/music-tui/internal/lyrics"
)

const (
	lrcLibBaseURL = "https://lrclib.net/api"
	// lrcLibTimeout is the per-request HTTP timeout.
	lrcLibTimeout = 10 * time.Second
	// lrcLibUserAgent identifies the client to lrclib.net.
	lrcLibUserAgent = "music-tui/0.1 (https://github.com/eilianxiao/music-tui)"
)

// LrcLibProvider fetches lyrics from lrclib.net.
//
// Strategy:
//  1. Try GET /api/get with exact metadata (title + artist + album + duration).
//     lrclib.net uses duration to disambiguate live/studio versions.
//  2. If the exact match returns no synced lyrics (instrumental or not found),
//     fall back to GET /api/search?q=<artist title> and pick the best hit.
//
// Both endpoints return syncedLyrics (LRC format) and plainLyrics (plain text).
// syncedLyrics is preferred; plainLyrics is used as a static fallback.
type LrcLibProvider struct {
	client *http.Client
}

// NewLrcLibProvider creates a LrcLibProvider with a default HTTP client.
func NewLrcLibProvider() *LrcLibProvider {
	return &LrcLibProvider{
		client: &http.Client{Timeout: lrcLibTimeout},
	}
}

// Fetch implements lyrics.Provider.
func (p *LrcLibProvider) Fetch(ctx context.Context, track library.Track) ([]lyrics.Line, error) {
	title := track.DisplayTitle()
	artist := track.DisplayArtist()
	if title == "" || artist == "Unknown Artist" {
		return nil, nil // not enough metadata to search
	}

	// 1. Exact match.
	resp, err := p.getExact(ctx, track)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		lines := lrcLibRespToLines(resp)
		if lines != nil {
			return lines, nil
		}
		// resp found but has no usable lyrics (instrumental) — skip search.
		if resp.Instrumental {
			return nil, nil
		}
	}

	// 2. Search fallback.
	return p.search(ctx, track)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// lrcLibResponse is the JSON shape returned by lrclib.net /api/get and each
// element of /api/search.
type lrcLibResponse struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	SyncedLyrics string  `json:"syncedLyrics"`
	PlainLyrics  string  `json:"plainLyrics"`
}

// getExact calls GET /api/get with full metadata.
// Returns (nil, nil) when the track is not found (404).
func (p *LrcLibProvider) getExact(ctx context.Context, track library.Track) (*lrcLibResponse, error) {
	q := url.Values{}
	q.Set("track_name", track.DisplayTitle())
	q.Set("artist_name", track.DisplayArtist())
	if track.Album != "" {
		q.Set("album_name", track.Album)
	}
	if track.Duration > 0 {
		secs := track.Duration.Seconds()
		q.Set("duration", strconv.FormatFloat(secs, 'f', 0, 64))
	}

	reqURL := lrcLibBaseURL + "/get?" + q.Encode()
	resp, err := p.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil // 404
	}

	var result lrcLibResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("lrclib get: decode response: %w", err)
	}
	return &result, nil
}

// search calls GET /api/search and returns lines from the best-matching hit.
// "Best" is the first result whose artist name matches the track's artist.
func (p *LrcLibProvider) search(ctx context.Context, track library.Track) ([]lyrics.Line, error) {
	q := url.Values{}
	q.Set("q", track.DisplayArtist()+" "+track.DisplayTitle())

	reqURL := lrcLibBaseURL + "/search?" + q.Encode()
	resp, err := p.doGet(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}

	var results []lrcLibResponse
	if err := json.Unmarshal(resp, &results); err != nil {
		return nil, fmt.Errorf("lrclib search: decode response: %w", err)
	}

	// Pick the first result that matches the artist; fall back to the first
	// result overall if nothing matches exactly.
	want := strings.ToLower(track.DisplayArtist())
	var best *lrcLibResponse
	for i := range results {
		r := &results[i]
		if r.Instrumental {
			continue
		}
		if strings.Contains(strings.ToLower(r.ArtistName), want) {
			best = r
			break
		}
		if best == nil {
			best = r
		}
	}
	if best == nil {
		return nil, nil
	}
	return lrcLibRespToLines(best), nil
}

// doGet performs an HTTP GET and returns the response body.
// Returns (nil, nil) for 404 responses (track not found).
// Returns (nil, err) for other non-2xx responses or network errors.
func (p *LrcLibProvider) doGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("lrclib: build request: %w", err)
	}
	req.Header.Set("User-Agent", lrcLibUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lrclib: http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lrclib: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lrclib: read body: %w", err)
	}
	return body, nil
}

// lrcLibRespToLines converts a lrclib.net response to []lyrics.Line.
// syncedLyrics is parsed as LRC; plainLyrics is the static fallback.
// Returns nil when neither field has usable content.
func lrcLibRespToLines(r *lrcLibResponse) []lyrics.Line {
	if r.SyncedLyrics != "" {
		lines := lyrics.ParseLRCString(r.SyncedLyrics)
		if len(lines) > 0 {
			return lines
		}
	}
	if r.PlainLyrics != "" {
		return lyrics.PlainTextToLines(r.PlainLyrics)
	}
	return nil
}
