package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lonsty/music-tui/internal/store"
)

// renderPlaylistTab renders the full body for the Playlist tab.
// It dispatches to the list view or the track view depending on depth.
func (a *App) renderPlaylistTab() string {
	left := a.renderPlaylistList()
	if !a.showMiniPlayer() {
		return left
	}
	var right string
	if a.rightMode == rightPanelLyrics {
		right = a.renderMiniLyrics()
	} else {
		right = a.renderMiniPlayer()
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderPlaylistList renders the left panel of the Playlist tab.
// When depth == list it shows all playlists; when depth == tracks it shows
// the tracks inside the selected playlist (with an optional inline add-panel).
func (a *App) renderPlaylistList() string {
	innerW := a.trackListInnerW()
	innerH := a.panelInnerH()

	if a.depth == playlistDepthTracks {
		return a.renderPlaylistTracks(innerW, innerH)
	}
	return a.renderPlaylistListLevel(innerW, innerH)
}

// ── Playlist list level ───────────────────────────────────────────────────────

func (a *App) renderPlaylistListLevel(innerW, innerH int) string {
	var sb strings.Builder

	// Header.
	title := stylePanelTitle.Render(iconWithSpace(iconPlaylist()) + T("playlist_header"))
	var countText string
	if n := len(a.playlists); n > 0 {
		countText = "  " + fmt.Sprintf(T("playlist_track_count"), n)
	}
	sb.WriteString(title + styleTrackMeta.Render(countText) + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2

	// Inline input (new / rename).
	inputLines := 0
	if a.inputMode != playlistInputNone {
		prompt := T("playlist_new_prompt")
		if a.inputMode == playlistInputRename {
			prompt = T("playlist_rename_prompt")
		}
		sb.WriteString(styleSearchPrompt.Render(prompt+": ") + a.nameInput.View() + "\n")
		inputLines = 1
	}

	// Delete-confirm banner.
	if a.confirmDel && a.inputMode == playlistInputNone {
		sb.WriteString(styleOverlayMuted.Render("  "+T("playlist_delete_confirm")) + "\n")
		inputLines = 1
	}

	maxRows := innerH - headerLines - inputLines
	if maxRows < 0 {
		maxRows = 0
	}

	if len(a.playlists) == 0 {
		empty := styleOverlayMuted.Render("  " + T("playlist_empty"))
		centered := lipgloss.Place(innerW, maxRows, lipgloss.Left, lipgloss.Center, empty)
		sb.WriteString(centered)
	} else {
		start, end := visibleWindow(a.listCursor, len(a.playlists), maxRows)
		for i := start; i < end; i++ {
			pl := a.playlists[i]
			isSelected := i == a.listCursor
			isFav := pl.ID == store.FavoritesPlaylistID

			// Left icon column (2 cols).
			const leftFixW = 2
			leftIcon := "  "
			if isFav {
				leftIcon = styleModeIcon.Render(iconHeartFilled()) + " "
			}

			// Right: track count (right-aligned, fixed 8 cols).
			const rightColW = 8
			var count int
			if a.st != nil {
				count, _ = a.st.PlaylistTrackCount(pl.ID)
			}
			countStr := fmt.Sprintf(T("playlist_track_count"), count)
			rightStr := padLeft(countStr, rightColW)

			// Middle: playlist name (elastic).
			midAvail := innerW - leftFixW - rightColW
			if midAvail < 4 {
				midAvail = 4
			}
			name := pl.Name
			if isFav {
				// Show the localised name for Favorites.
				name = T("playlist_favorites_name")
			}
			if strWidth(name) > midAvail {
				name = truncate(name, midAvail)
			}
			namePadded := padRight(name, midAvail)

			line := leftIcon + namePadded + styleTrackMeta.Render(rightStr)

			var style lipgloss.Style
			if isSelected {
				style = styleTrackRowSelected
			} else {
				style = styleTrackRowDefault
			}
			sb.WriteString(style.Render(line) + "\n")
		}
	}

	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// ── Playlist track level ──────────────────────────────────────────────────────

func (a *App) renderPlaylistTracks(innerW, innerH int) string {
	pl := a.currentPlaylist()
	var sb strings.Builder

	// Header: playlist name + back hint.
	backHint := styleTrackMeta.Render("  " + T("playlist_back_hint"))
	plName := pl.Name
	if pl.ID == store.FavoritesPlaylistID {
		plName = T("playlist_favorites_name")
	}
	title := stylePanelTitle.Render(iconWithSpace(iconPlaylist()) + plName)
	var count int
	if a.st != nil {
		count, _ = a.st.PlaylistTrackCount(pl.ID)
	}
	countStr := fmt.Sprintf("  "+T("playlist_track_count"), count)
	sb.WriteString(title + styleTrackMeta.Render(countStr) + backHint + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2

	// Add-tracks inline panel.
	addLines := 0
	if a.addingTracks {
		sb.WriteString(styleSearchPrompt.Render(iconSearch()+" ") + a.addInput.View() + "\n")
		sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")
		addLines = 2
	}

	maxRows := innerH - headerLines - addLines
	if maxRows < 0 {
		maxRows = 0
	}

	if a.addingTracks {
		a.renderAddTrackResults(&sb, innerW, maxRows)
	} else {
		a.renderPlaylistTrackRows(&sb, innerW, maxRows)
	}

	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// renderPlaylistTrackRows renders the track rows for the current playlist.
func (a *App) renderPlaylistTrackRows(sb *strings.Builder, innerW, maxRows int) {
	tracks := a.plTracks
	if len(tracks) == 0 {
		empty := styleOverlayMuted.Render("  " + T("playlist_tracks_empty"))
		sb.WriteString(lipgloss.Place(innerW, maxRows, lipgloss.Left, lipgloss.Center, empty))
		return
	}

	start, end := visibleWindow(a.trackCursor, len(tracks), maxRows)
	for i := start; i < end; i++ {
		t := tracks[i]
		isSelected := i == a.trackCursor
		isPlaying := a.currentTrack != nil && a.currentTrack.ID == t.ID

		const leftFixW = 2
		leftIcon := "  "
		if isPlaying {
			leftIcon = iconPlaying() + " "
		}

		const rightColW = 10
		rightText := t.Format() + " " + formatDuration(t.Duration)
		rightPadded := padLeft(rightText, rightColW)

		midAvail := innerW - leftFixW - rightColW
		if midAvail < 4 {
			midAvail = 4
		}
		midText := rowMidText(t)
		var midPadded string
		if isPlaying {
			cut := midText
			if strWidth(cut) > midAvail {
				cut = truncate(cut, midAvail)
			}
			midPadded = gradientText(cut, true, gradientColors...) +
				padRight("", midAvail-strWidth(cut))
		} else {
			midPadded = padRight(truncate(midText, midAvail), midAvail)
		}

		var line string
		if isPlaying {
			line = styleTrackRowPlayingAccent.Render(leftIcon) + midPadded + styleTrackRowPlayingAccent.Render(rightPadded)
		} else {
			line = leftIcon + midPadded + rightPadded
		}

		var style lipgloss.Style
		switch {
		case isPlaying && isSelected:
			style = styleTrackRowPlayingSelected
		case isPlaying:
			style = styleTrackRowPlaying
		case isSelected:
			style = styleTrackRowSelected
		default:
			style = styleTrackRowDefault
		}
		sb.WriteString(style.Render(line) + "\n")
	}
}

// renderAddTrackResults renders search results inside the add-tracks panel.
func (a *App) renderAddTrackResults(sb *strings.Builder, innerW, maxRows int) {
	results := a.addResults
	if len(results) == 0 {
		sb.WriteString(styleOverlayMuted.Render("  "+T("playlist_tracks_empty")) + "\n")
		return
	}

	// Build a set of track IDs already in this playlist for ✓ marking.
	inPlaylist := make(map[string]bool, len(a.plTracks))
	for _, t := range a.plTracks {
		inPlaylist[t.ID] = true
	}

	start, end := visibleWindow(a.addCursor, len(results), maxRows)
	for i := start; i < end; i++ {
		idx := results[i]
		t := a.tracks[idx]
		isSelected := i == a.addCursor

		const leftFixW = 2
		var leftIcon string
		if inPlaylist[t.ID] {
			leftIcon = styleModeIcon.Render("✓") + " "
		} else {
			leftIcon = "  "
		}

		const rightColW = 10
		rightText := t.Format() + " " + formatDuration(t.Duration)
		rightPadded := padLeft(rightText, rightColW)

		midAvail := innerW - leftFixW - rightColW
		if midAvail < 4 {
			midAvail = 4
		}
		midText := rowMidText(t)
		midPadded := padRight(truncate(midText, midAvail), midAvail)

		line := leftIcon + midPadded + rightPadded

		var style lipgloss.Style
		if isSelected {
			style = styleTrackRowSelected
		} else {
			style = styleTrackRowDefault
		}
		sb.WriteString(style.Render(line) + "\n")
	}
}

// currentPlaylist returns the playlist currently open in depth == tracks.
// Returns a zero Playlist if the cursor is out of range.
func (a *App) currentPlaylist() store.Playlist {
	if a.listCursor < 0 || a.listCursor >= len(a.playlists) {
		return store.Playlist{}
	}
	return a.playlists[a.listCursor]
}

// applyAddFilter rebuilds a.addResults from a.tracks using the addInput query.
func (a *App) applyAddFilter() {
	q := strings.ToLower(strings.TrimSpace(a.addInput.Value()))
	if q == "" {
		// Show all tracks when query is empty.
		idxs := make([]int, len(a.tracks))
		for i := range a.tracks {
			idxs[i] = i
		}
		a.addResults = idxs
		return
	}
	var results []int
	for i, t := range a.tracks {
		haystack := strings.ToLower(t.Title + " " + t.Artist + " " + t.Album)
		if strings.Contains(haystack, q) {
			results = append(results, i)
		}
	}
	a.addResults = results
	if a.addCursor >= len(results) {
		a.addCursor = len(results) - 1
	}
	if a.addCursor < 0 {
		a.addCursor = 0
	}
}
