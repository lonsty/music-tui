package library

import (
	"sort"
	"strings"
)

// SortByArtistAlbum sorts tracks in-place by Artist → Album → Title,
// matching the default ordering used by most music players.
func SortByArtistAlbum(tracks []Track) {
	sort.SliceStable(tracks, func(i, j int) bool {
		a, b := &tracks[i], &tracks[j]
		ai := strings.ToLower(a.DisplayArtist())
		bi := strings.ToLower(b.DisplayArtist())
		if ai != bi {
			return ai < bi
		}
		alb := strings.ToLower(a.Album)
		blb := strings.ToLower(b.Album)
		if alb != blb {
			return alb < blb
		}
		return strings.ToLower(a.DisplayTitle()) < strings.ToLower(b.DisplayTitle())
	})
}
