package library

import (
	"sort"
	"strconv"
	"strings"
)

// SortByArtistAlbum sorts tracks in-place by the standard music-library order:
//
//  1. Album Artist  (falls back to Artist when empty)
//  2. Year          (ascending; tracks without a year sort last)
//  3. Album
//  4. Track number  (numeric part of the TRCK tag, e.g. "3/12" → 3)
//  5. Title         (tie-breaker)
func SortByArtistAlbum(tracks []Track) {
	sort.SliceStable(tracks, func(i, j int) bool {
		a, b := &tracks[i], &tracks[j]

		// 1. Album artist
		aa := strings.ToLower(a.DisplayAlbumArtist())
		ba := strings.ToLower(b.DisplayAlbumArtist())
		if aa != ba {
			return aa < ba
		}

		// 2. Year (ascending; empty year sorts after real years)
		ay := normaliseYear(a.Year)
		by := normaliseYear(b.Year)
		if ay != by {
			if ay == 0 {
				return false // a has no year → sort later
			}
			if by == 0 {
				return true // b has no year → sort later
			}
			return ay < by
		}

		// 3. Album
		alb := strings.ToLower(a.Album)
		blb := strings.ToLower(b.Album)
		if alb != blb {
			return alb < blb
		}

		// 4. Track number
		at := trackNum(a.TrackNumber)
		bt := trackNum(b.TrackNumber)
		if at != bt {
			if at == 0 {
				return false
			}
			if bt == 0 {
				return true
			}
			return at < bt
		}

		// 5. Title
		return strings.ToLower(a.DisplayTitle()) < strings.ToLower(b.DisplayTitle())
	})
}

// normaliseYear converts a year string like "2003", "2003-01-15", or "" to an
// integer for comparison.  Returns 0 when the string is empty or unparseable.
func normaliseYear(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// ID3 TDRC can be "YYYY-MM-DD"; take only the year part.
	if len(s) > 4 {
		s = s[:4]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// trackNum extracts the numeric track index from a TRCK value like "3" or
// "3/12".  Returns 0 when the string is empty or unparseable.
func trackNum(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if idx := strings.Index(s, "/"); idx != -1 {
		s = s[:idx]
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
