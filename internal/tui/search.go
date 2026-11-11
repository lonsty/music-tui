package tui

import (
	"strings"

	"github.com/lonsty/music-tui/internal/library"
)

// searchField identifies the field selected by a search prefix.
type searchField int

const (
	searchFieldAny searchField = iota
	searchFieldArtist
	searchFieldAlbum
	searchFieldTitle
	searchFieldFormat
)

// parseSearchQuery splits the raw query into an optional field prefix and the
// lower-cased search term.  Prefixes are: s: artist, a: album, t: title,
// f: format.  Unknown prefixes are treated as plain text.
func parseSearchQuery(raw string) (searchField, string) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(raw, "s:"):
		return searchFieldArtist, strings.TrimSpace(strings.TrimPrefix(raw, "s:"))
	case strings.HasPrefix(raw, "a:"):
		return searchFieldAlbum, strings.TrimSpace(strings.TrimPrefix(raw, "a:"))
	case strings.HasPrefix(raw, "t:"):
		return searchFieldTitle, strings.TrimSpace(strings.TrimPrefix(raw, "t:"))
	case strings.HasPrefix(raw, "f:"):
		return searchFieldFormat, strings.TrimSpace(strings.TrimPrefix(raw, "f:"))
	default:
		return searchFieldAny, raw
	}
}

// trackMatchesQuery reports whether t matches the parsed search query.
func trackMatchesQuery(t library.Track, field searchField, q string) bool {
	if q == "" {
		return true
	}
	switch field {
	case searchFieldArtist:
		return strings.Contains(strings.ToLower(t.Artist), q) ||
			strings.Contains(strings.ToLower(t.AlbumArtist), q)
	case searchFieldAlbum:
		return strings.Contains(strings.ToLower(t.Album), q)
	case searchFieldTitle:
		return strings.Contains(strings.ToLower(t.DisplayTitle()), q)
	case searchFieldFormat:
		return strings.Contains(strings.ToLower(t.Format()), q)
	default:
		return strings.Contains(strings.ToLower(t.DisplayTitle()), q) ||
			strings.Contains(strings.ToLower(t.Artist), q) ||
			strings.Contains(strings.ToLower(t.AlbumArtist), q) ||
			strings.Contains(strings.ToLower(t.Album), q)
	}
}

// selectTrackIndices filters tracks by the raw query and the active format
// preference, returning positions into tracks.  It is shared by the Local tab
// search and the add-to-playlist panel so both use identical semantics.
//
// The scan works on positions only: tracks are never copied, and the format
// preference is applied to the position list rather than to a materialised
// slice of Track values.
func selectTrackIndices(tracks []library.Track, raw string, pref formatPreference) []int {
	field, q := parseSearchQuery(raw)

	positions := make([]int, 0, len(tracks))
	for i := range tracks {
		if trackMatchesQuery(tracks[i], field, q) {
			positions = append(positions, i)
		}
	}
	return applyFormatPreferenceIndices(tracks, positions, pref)
}
