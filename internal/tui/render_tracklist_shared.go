package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderSharedTrackList renders a track-list panel using the supplied
// TrackListContext.  It produces the full bordered panel (header + divider +
// rows) at the given dimensions and is the single source of truth for track-
// list rendering shared between the Local and Playlist tabs.
func (a *App) renderSharedTrackList(ctx TrackListContext, innerW, innerH int) string {
	var sb strings.Builder

	// ── Header ───────────────────────────────────────────────────────────────
	if ctx.ShowSearchBar && a.activeOvl == overlaySearch {
		// Search mode: replace the title line with the search input.
		prompt := styleSearchPrompt.Render(iconSearch() + " ")
		sb.WriteString(prompt + a.searchInput.View() + "\n")
	} else {
		title := ctx.HeaderTitle()

		// Track count annotation.
		total := ctx.Count()
		var countText string
		if a.currentTrack != nil && ctx.PlayingPos(a.currentTrack.ID) >= 0 {
			countText = fmt.Sprintf("  "+T("library_count_playing"), ctx.PlayingPos(a.currentTrack.ID)+1, total)
		} else {
			countText = fmt.Sprintf("  "+T("library_count"), total)
		}
		count := styleTrackMeta.Render(countText)

		// Active search filter: keep it visible after the input closes so the
		// shortened list is never unexplained.
		var filterChip string
		if ctx.ShowSearchBar && a.activeOvl != overlaySearch {
			if q := strings.TrimSpace(a.searchInput.Value()); q != "" {
				filterChip = styleSearchPrompt.Render("  " + fmt.Sprintf(T("search_filter_active"), q))
			}
		}

		// Right annotation (e.g. "← Playlists" for the playlist track level).
		rightAnnotation := ctx.HeaderRight()

		// When collapsed, replace annotation with compact play status.
		var playStatus string
		if !a.showMiniPlayer() && a.currentTrack != nil {
			titleW := strWidth(title)
			countW := strWidth(countText)
			filterW := strWidth(filterChip)
			rightAnnotationW := strWidth(rightAnnotation)
			remainW := innerW - titleW - countW - filterW - rightAnnotationW - 2
			playStatus = a.buildCollapsedPlayStatus(remainW)
		}

		if playStatus != "" {
			usedW := strWidth(title) + strWidth(countText) + strWidth(filterChip) + strWidth(rightAnnotation)
			gap := innerW - usedW - strWidth(playStatus)
			if gap < 1 {
				gap = 1
			}
			sb.WriteString(title + count + filterChip + rightAnnotation +
				strings.Repeat(" ", gap) + playStatus + "\n")
		} else {
			sb.WriteString(title + count + filterChip + rightAnnotation + "\n")
		}
	}
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2
	maxRows := innerH - headerLines
	if maxRows < 0 {
		maxRows = 0
	}

	// ── Row rendering ────────────────────────────────────────────────────────
	total := ctx.Count()
	cursor := *ctx.CursorPos

	if total == 0 {
		emptyKey := ctx.EmptyMsg
		if emptyKey == "" {
			emptyKey = "no_tracks_hint"
		}
		emptyText := T(emptyKey)
		if q := strings.TrimSpace(a.searchInput.Value()); q != "" && ctx.ShowSearchBar {
			emptyText = fmt.Sprintf(T("no_search_results"), q) + "\n" + T("search_clear_hint")
		}
		empty := styleOverlayMuted.Render(emptyText)
		centered := lipgloss.Place(innerW, maxRows, lipgloss.Center, lipgloss.Center, empty)
		sb.WriteString(centered)
		content := strings.TrimRight(sb.String(), "\n")
		return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
	}

	start, end := visibleWindow(cursor, total, maxRows)

	for i := start; i < end; i++ {
		t := ctx.TrackAt(i)
		isSelected := i == cursor
		isPlaying := a.currentTrack != nil && a.currentTrack.ID == t.ID

		// ── Left: playing icon (fixed 2 cols) ─────────────────────────────
		const leftFixW = 2
		icon := "  "
		if isPlaying {
			icon = iconPlaying() + " "
		}

		// ── Right: format + duration (fixed 10 cols) ──────────────────────
		const rightColW = 10
		rightText := t.Format() + " " + formatDuration(t.Duration)
		rightPadded := padLeft(rightText, rightColW)

		// ── Middle: Album · Artist · Title (elastic) ──────────────────────
		midAvail := innerW - leftFixW - rightColW
		if midAvail < 4 {
			midAvail = 4
		}

		midText := rowMidText(t)
		var midPadded string

		if ctx.UseMarquee {
			// Local tab: use marquee for the selected row.
			switch {
			case isPlaying && isSelected:
				core := strings.TrimRight(a.mqRow.Render(midAvail), " ")
				coreW := strWidth(core)
				midPadded = gradientTextBg(core, true, surface0, gradientColors...) +
					padRight("", midAvail-coreW)
			case isSelected:
				midPadded = a.mqRow.Render(midAvail)
			case isPlaying:
				cut := truncate(midText, midAvail)
				midPadded = gradientText(cut, true, gradientColors...) +
					padRight("", midAvail-strWidth(cut))
			default:
				midPadded = padRight(truncate(midText, midAvail), midAvail)
			}
		} else {
			// Playlist / Online tab: static truncation.
			cut := truncate(midText, midAvail)
			switch {
			case isPlaying && isSelected:
				midPadded = gradientTextBg(cut, true, surface0, gradientColors...) +
					padRight("", midAvail-strWidth(cut))
			case isPlaying:
				midPadded = gradientText(cut, true, gradientColors...) +
					padRight("", midAvail-strWidth(cut))
			default:
				midPadded = padRight(cut, midAvail)
			}
		}

		// ── Assemble line ─────────────────────────────────────────────────
		var line string
		switch {
		case isPlaying && isSelected:
			line = fgColorBoldBg(icon, blue, surface0) +
				midPadded +
				fgColorBoldBg(rightPadded, blue, surface0)
		case isPlaying:
			line = fgColorBold(icon, blue) + midPadded + fgColorBold(rightPadded, blue)
		default:
			line = icon + midPadded + rightPadded
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

	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}
