package tui

import (
	"strings"

	"github.com/lonsty/music-tui/internal/library"
)

// ── Search / filter ──────────────────────────────────────────────────────────

// applyFilter rebuilds a.filtered from a.tracks using the current search query
// and the active format preference.
//
// Search prefix syntax:
//
//	s:KEY  — search artist (singer)
//	a:KEY  — search album
//	t:KEY  — search title
//	f:KEY  — filter by audio format (e.g. f:flac, f:mp3)
//	KEY    — search all text fields
//
// After text filtering, applyFormatPreference is applied to deduplicate or
// hide tracks based on the active formatPreference setting.
func (a *App) applyFilter() {
	raw := strings.TrimSpace(a.searchInput.Value())

	var idxs []int
	if raw == "" && a.formatPref == formatPrefAll && a.drilldown == nil {
		// Unfiltered view: reuse the cached identity table instead of copying
		// every track or remapping through a temporary ID map.
		idxs = a.identityIndices()
	} else {
		source := a.tracks
		if a.drilldown != nil {
			source = make([]library.Track, 0, len(a.tracks))
			for _, t := range a.tracks {
				if a.drilldown.ids[t.ID] {
					source = append(source, t)
				}
			}
		}

		idxs = selectTrackIndices(source, raw, a.formatPref)
		if a.drilldown != nil {
			// selectTrackIndices returns positions into source; remap to a.tracks.
			idToPos := make(map[string]int, len(a.tracks))
			for i := range a.tracks {
				idToPos[a.tracks[i].ID] = i
			}
			remapped := make([]int, 0, len(idxs))
			for _, i := range idxs {
				if pos, ok := idToPos[source[i].ID]; ok {
					remapped = append(remapped, pos)
				}
			}
			idxs = remapped
		}
	}
	a.filteredIdxs = idxs
	a.rebuildFilteredPos()

	// Groups depend only on tracks and the format preference, not on the
	// search query.  Mark them dirty instead of rebuilding on every keystroke.
	if !a.groupsBuilt || a.groupsPref != a.formatPref {
		a.groupsDirty = true
	}
	a.groupsPref = a.formatPref

	// Invalidate shuffle so it is rebuilt from the new filtered list on next use.
	a.shuffleIDs = nil

	// Reset cursor to the first result only when the query actually changes.
	// This allows ↑/↓ navigation within results without the cursor snapping
	// back to row 0 on every applyFilter call (e.g. format-pref toggles).
	queryChanged := raw != a.lastQuery
	a.lastQuery = raw

	if queryChanged && strings.TrimSpace(raw) != "" {
		// New non-empty search: jump to first result.
		a.cursorPos = 0
	} else if len(a.filteredIdxs) == 0 {
		a.cursorPos = 0
	} else if a.cursorPos >= len(a.filteredIdxs) {
		a.cursorPos = len(a.filteredIdxs) - 1
	}
}

// formatDedupKey is the identity used by lossless-first deduplication:
// lowercase (artist · album · title).
func formatDedupKey(t *library.Track) string {
	return strings.ToLower(t.DisplayArtist()) + "\x00" +
		strings.ToLower(t.Album) + "\x00" +
		strings.ToLower(t.DisplayTitle())
}

// applyFormatPreferenceIndices is the position-based counterpart of
// applyFormatPreference.  It preserves input order and never copies tracks.
func applyFormatPreferenceIndices(tracks []library.Track, idxs []int, pref formatPreference) []int {
	switch pref {
	case formatPrefAll:
		return idxs

	case formatPrefLosslessFirst:
		type entry struct {
			pos     int
			quality int
		}
		best := make(map[string]entry, len(idxs))
		for _, i := range idxs {
			t := &tracks[i]
			key := formatDedupKey(t)
			q := library.QualityOf(t.Format())
			if prev, ok := best[key]; !ok || q > prev.quality {
				best[key] = entry{pos: i, quality: q}
			}
		}
		out := make([]int, 0, len(best))
		for _, i := range idxs {
			if e := best[formatDedupKey(&tracks[i])]; e.pos == i {
				out = append(out, i)
			}
		}
		return out

	case formatPrefLosslessOnly:
		const losslessThreshold = 30
		out := make([]int, 0, len(idxs))
		for _, i := range idxs {
			if library.QualityOf(tracks[i].Format()) >= losslessThreshold {
				out = append(out, i)
			}
		}
		return out

	case formatPrefMP3Only:
		out := make([]int, 0, len(idxs))
		for _, i := range idxs {
			if strings.EqualFold(tracks[i].Format(), "MP3") {
				out = append(out, i)
			}
		}
		return out

	default:
		return idxs
	}
}

// applyFormatPreference filters or deduplicates tracks according to pref.
//
//   - formatPrefAll: returns tracks unchanged.
//   - formatPrefLosslessFirst: for each (artist, album, title) group keeps
//     only the copy with the highest format quality score.  Tracks whose
//     group has no other members are always included regardless of format.
//   - formatPrefLosslessOnly: removes any track whose format quality score
//     is below the lowest lossless threshold (WAV/FLAC score ≥ 30).
//   - formatPrefMP3Only: keeps only MP3 tracks.
//
// The original ordering is preserved.
func applyFormatPreference(tracks []library.Track, pref formatPreference) []library.Track {
	switch pref {
	case formatPrefAll:
		return tracks

	case formatPrefLosslessFirst:
		// Group tracks by their dedup key: lowercase (artist · album · title).
		// For each group, remember the index of the highest-quality track seen.
		type entry struct {
			idx     int
			quality int
		}
		best := make(map[string]entry, len(tracks))
		for i, t := range tracks {
			key := formatDedupKey(&tracks[i])
			q := library.QualityOf(t.Format())
			if prev, ok := best[key]; !ok || q > prev.quality {
				best[key] = entry{idx: i, quality: q}
			}
		}
		// Collect winners in original order using an index set.
		keep := make(map[int]bool, len(best))
		for _, e := range best {
			keep[e.idx] = true
		}
		out := make([]library.Track, 0, len(best))
		for i, t := range tracks {
			if keep[i] {
				out = append(out, t)
			}
		}
		return out

	case formatPrefLosslessOnly:
		// lossless threshold: quality score ≥ 30 (WAV=30, FLAC=40).
		const losslessThreshold = 30
		out := make([]library.Track, 0, len(tracks))
		for _, t := range tracks {
			if library.QualityOf(t.Format()) >= losslessThreshold {
				out = append(out, t)
			}
		}
		return out

	case formatPrefMP3Only:
		out := make([]library.Track, 0, len(tracks))
		for _, t := range tracks {
			if strings.EqualFold(t.Format(), "MP3") {
				out = append(out, t)
			}
		}
		return out

	default:
		return tracks
	}
}
