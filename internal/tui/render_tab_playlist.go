package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lonsty/music-tui/internal/library"
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

	// Header — show only the section title; no track/count number here.
	title := stylePanelTitle.Render(iconWithSpace(iconPlaylist()) + T("playlist_header"))
	header := title
	if !a.showMiniPlayer() && a.currentTrack != nil {
		remainW := innerW - strWidth(title) - 2
		if ps := a.buildCollapsedPlayStatus(remainW); ps != "" {
			gap := innerW - strWidth(title) - strWidth(ps)
			if gap < 1 {
				gap = 1
			}
			header = title + strings.Repeat(" ", gap) + ps
		}
	}
	sb.WriteString(header + "\n")
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

	maxRows := innerH - headerLines - inputLines
	if maxRows < 0 {
		maxRows = 0
	}

	if len(a.playlists) == 0 {
		empty := styleOverlayMuted.Render(T("playlist_empty"))
		centered := lipgloss.Place(innerW, maxRows, lipgloss.Center, lipgloss.Center, empty)
		sb.WriteString(centered)
	} else {
		// Use the cached right-column width (computed once when playlists
		// are loaded / modified) to avoid N DB queries per render frame.
		rightColW := a.plListMaxColW
		if rightColW < 4 {
			rightColW = 4
		}
		const leftFixW = 2

		start, end := visibleWindow(a.listCursor, len(a.playlists), maxRows)
		for i := start; i < end; i++ {
			pl := a.playlists[i]
			isSelected := i == a.listCursor
			isFav := pl.ID == store.FavoritesPlaylistID

			// Left icon column (2 cols).
			leftIcon := "  "
			if isFav {
				leftIcon = fgColorBold(iconHeartFilled(), mauve) + " "
			}

			// Right: track count (dynamic width).
			var count int
			if a.st != nil {
				count, _ = a.st.PlaylistTrackCount(pl.ID)
			}
			countStr := fmt.Sprintf(T("playlist_track_count"), count)
			rightStr := fgColor(padLeft(countStr, rightColW), overlay0)

			// Middle: playlist name (elastic, marquee on selected row).
			midAvail := innerW - leftFixW - rightColW
			if midAvail < 4 {
				midAvail = 4
			}
			var namePadded string
			if isSelected {
				// Use the marquee scroller for the selected row.
				namePadded = a.mqPlRow.Render(midAvail)
			} else {
				namePadded = padRight(truncate(a.playlistDisplayName(pl), midAvail), midAvail)
			}

			line := leftIcon + namePadded + rightStr

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
	if a.addingTracks {
		return a.renderPlaylistAddPanel(innerW, innerH)
	}
	return a.renderSharedTrackList(a.playlistTrackCtx(), innerW, innerH)
}

// renderPlaylistAddPanel renders the inline "add tracks" search panel that
// appears when a.addingTracks is true.  It is separate from the main track
// list so that the shared engine does not need to know about this mode.
func (a *App) renderPlaylistAddPanel(innerW, innerH int) string {
	var sb strings.Builder

	// Header: breadcrumb navigation — Playlists > playlist name + back hint.
	pl := a.currentPlaylist()
	title := stylePanelTitle.Render(
		iconWithSpace(iconPlaylist()) + T("playlist_header") +
			breadcrumbSep() + a.playlistDisplayName(pl))
	backHint := styleTrackMeta.Render("  " + T("playlist_back_hint"))
	sb.WriteString(title + backHint + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	// Search input row + second divider.
	sb.WriteString(styleSearchPrompt.Render(iconSearch()+" ") + a.addInput.View() + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const fixedRows = 4
	maxRows := innerH - fixedRows
	if maxRows < 0 {
		maxRows = 0
	}

	a.renderAddTrackResults(&sb, innerW, maxRows)

	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// renderAddTrackResults renders search results inside the add-tracks panel.
func (a *App) renderAddTrackResults(sb *strings.Builder, innerW, maxRows int) {
	results := a.addResults
	if len(results) == 0 {
		sb.WriteString(styleOverlayMuted.Render("  "+T("playlist_tracks_empty")) + "\n")
		return
	}

	// Build a set of track IDs already in this playlist for ✓ marking.
	// Merge delta changes from the current add-tracks session so that
	// recently toggled tracks show the correct icon immediately.
	inPlaylist := make(map[string]bool, len(a.plTracks))
	for _, t := range a.plTracks {
		inPlaylist[t.ID] = true
	}
	for id, v := range a.addDelta {
		if v > 0 {
			inPlaylist[id] = true
		} else if v < 0 {
			delete(inPlaylist, id)
		}
	}

	start, end := visibleWindow(a.addCursor, len(results), maxRows)
	for i := start; i < end; i++ {
		idx := results[i]
		t := a.tracks[idx]
		isSelected := i == a.addCursor

		const leftFixW = 2
		const rightColW = 10

		midAvail := innerW - leftFixW - rightColW
		if midAvail < 4 {
			midAvail = 4
		}
		midText := rowMidText(t)
		midPadded := padRight(truncate(midText, midAvail), midAvail)
		rightText := t.Format() + " " + formatDuration(t.Duration)
		rightPadded := padLeft(rightText, rightColW)

		var line string
		if isSelected {
			// Use fgColorBold (no trailing reset) so the selection background
			// fills the entire row, including the checkmark icon column.
			var leftIcon string
			if inPlaylist[t.ID] {
				leftIcon = fgColorBoldBg("✓", mauve, surface0) + " "
			} else {
				leftIcon = "  "
			}
			line = leftIcon + midPadded + rightPadded
		} else {
			var leftIcon string
			if inPlaylist[t.ID] {
				leftIcon = fgColorBold("✓", mauve) + " "
			} else {
				leftIcon = "  "
			}
			line = leftIcon + midPadded + rightPadded
		}

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
	var src []library.Track
	if q == "" {
		src = a.tracks
	} else {
		for _, t := range a.tracks {
			haystack := strings.ToLower(t.Title + " " + t.Artist + " " + t.Album)
			if strings.Contains(haystack, q) {
				src = append(src, t)
			}
		}
	}
	// Apply format preference so add results stay consistent with
	// the Local tab's filtered view.
	filtered := applyFormatPreference(src, a.formatPref)
	var results []int
	for _, t := range filtered {
		for i := range a.tracks {
			if a.tracks[i].ID == t.ID {
				results = append(results, i)
				break
			}
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
