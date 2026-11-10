package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderGroupList renders the Albums or Artists view for the Library tab.
func (a *App) renderGroupList(innerW, innerH int) string {
	groups := a.activeGroups()
	viewLabel := T("library_view_albums")
	icon := iconAlbum()
	if a.view == libraryViewArtists {
		viewLabel = T("library_view_artists")
		icon = iconArtist()
	}

	title := stylePanelTitle.Render(iconWithSpace(icon) + T("library_title") + " · " + viewLabel)
	count := styleTrackMeta.Render(fmt.Sprintf("  "+T("library_count"), len(groups)))
	header := title + count

	var sb strings.Builder
	sb.WriteString(header + "\n")
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2
	maxRows := innerH - headerLines
	if maxRows < 0 {
		maxRows = 0
	}

	if len(groups) == 0 {
		empty := styleOverlayMuted.Render(T("no_tracks_hint"))
		sb.WriteString(lipgloss.Place(innerW, maxRows, lipgloss.Center, lipgloss.Center, empty))
		return stylePanelBorder.Width(innerW).Height(innerH).Render(strings.TrimRight(sb.String(), "\n"))
	}

	const leftFixW = 2
	const rightColW = 10
	midAvail := innerW - leftFixW - rightColW
	if midAvail < 4 {
		midAvail = 4
	}

	start, end := visibleWindow(a.groupCursor, len(groups), maxRows)
	for i := start; i < end; i++ {
		g := groups[i]
		isSelected := i == a.groupCursor

		iconCol := "  "
		ic := iconAlbum()
		if a.view == libraryViewArtists {
			ic = iconArtist()
		}
		if ic != "" {
			iconCol = ic + " "
		}

		mid := a.groupTitle(g)
		if sub := a.groupSubtitle(g); sub != "" && a.view == libraryViewAlbums {
			mid += " · " + sub
		}
		mid = padRight(truncate(mid, midAvail), midAvail)

		countStr := padLeft(fmt.Sprintf(T("library_count"), len(g.IDs)), rightColW)

		line := iconCol + mid + countStr
		style := styleTrackRowDefault
		if isSelected {
			style = styleTrackRowSelected
		}
		sb.WriteString(style.Render(line) + "\n")
	}

	return stylePanelBorder.Width(innerW).Height(innerH).Render(strings.TrimRight(sb.String(), "\n"))
}
