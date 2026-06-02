package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Tab bar ───────────────────────────────────────────────────────────────────

// renderTabBar renders a single-line tab bar (tabBarH = 1).
// The active tab is marked with a ▌ prefix in mauve; inactive tabs are dim.
// Both active and inactive tabs occupy the same total width so the layout
// does not shift when switching tabs.
func (a *App) renderTabBar() string {
	type tabDef struct {
		id    tabID
		icon  string
		label string
	}
	tabs := []tabDef{
		{tabLocal, iconLibrary(), T("tab_local")},
		{tabOnline, iconOnline(), T("tab_online")},
		{tabPlaylist, iconPlaylist(), T("tab_playlist")},
	}

	// activeMark is the left marker rendered on the active tab.
	// A plain space of the same display width is used for inactive tabs so
	// that all tabs remain the same total width regardless of active state.
	const activeMark = "▌ "   // 1 col block + 1 space = 2 display cols
	const inactiveMark = "  " // 2 spaces — same width as activeMark

	var parts []string
	for _, t := range tabs {
		ic := t.icon
		sep := ""
		if ic != "" {
			sep = "  " // gap between icon and label only when icon is present
		}
		body := ic + sep + t.label
		if t.id == a.activeTab {
			mark := styleTabActive.Render(activeMark)
			parts = append(parts, mark+styleTabActive.Render(body)+styleTabActive.Render("  "))
		} else {
			parts = append(parts, styleTabInactive.Render(inactiveMark+body+"  "))
		}
	}

	// Pad remaining width so the tab bar spans the full terminal width.
	return styleTabBar.Width(a.W).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, parts...),
	)
}

// ── Normal body (track list + mini player) ────────────────────────────────────

func (a *App) renderNormalBody() string {
	switch a.activeTab {
	case tabOnline:
		return a.renderOnlinePlaceholder()
	case tabPlaylist:
		return a.renderPlaylistTab()
	default:
	}
	left := a.renderTrackList()
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

// ── Online tab placeholder ────────────────────────────────────────────────────

func (a *App) renderOnlinePlaceholder() string {
	outerW := a.W
	innerW := outerW - borderW
	innerH := a.panelInnerH()

	features := []struct{ icon, text string }{
		{iconLyrics(), "Online streaming & radio"},
		{iconLibrary(), "Netease / Spotify integration"},
		{iconLyricsSync(), "Lyrics sync & search"},
		{iconMusic(), "Playlist discovery"},
	}

	var lines []string
	lines = append(lines,
		gradientText(iconWithSpace(iconOnline())+"Online", true, blue, mauve, pink),
		"",
		styleOverlayMuted.Render(T("online_coming_soon")),
		"",
	)
	for _, f := range features {
		icon := styleModeIcon.Render(f.icon)
		label := styleOverlayValue.Render("  " + f.text)
		lines = append(lines, "  "+icon+label)
	}
	lines = append(lines,
		"",
		styleOverlayMuted.Render(T("online_back_hint")),
	)

	block := lipgloss.JoinVertical(lipgloss.Left, lines...)
	centered := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, block)
	return stylePanelBorder.Width(innerW).Height(innerH).Render(centered)
}

// ── Track list ────────────────────────────────────────────────────────────────

func (a *App) renderTrackList() string {
	innerW := a.trackListInnerW()
	innerH := a.panelInnerH()

	var sb strings.Builder

	// ── Header ──
	if a.activeOvl == overlaySearch {
		prompt := styleSearchPrompt.Render(iconSearch() + " ")
		sb.WriteString(prompt + a.searchInput.View() + "\n")
	} else {
		title := stylePanelTitle.Render(iconWithSpace(iconLibrary()) + T("library_title"))
		// Show "pos / total" when a track from the filtered list is playing.
		var countText string
		if a.currentTrack != nil {
			if pos := a.filteredPos(a.currentTrack.ID); pos >= 0 {
				countText = fmt.Sprintf("  "+T("library_count_playing"), pos+1, a.filteredLen())
			} else {
				countText = fmt.Sprintf("  "+T("library_count"), a.filteredLen())
			}
		} else {
			countText = fmt.Sprintf("  "+T("library_count"), a.filteredLen())
		}
		count := styleTrackMeta.Render(countText)

		// When the right panel is hidden (collapsed or narrow terminal), show
		// a compact playback status on the right side of the header line.
		// The info is dropped progressively as available width shrinks:
		//   full:      ▶ title ████░ 1:23/3:24
		//   no time:   ▶ title ████░
		//   no bar:    ▶ title
		//   icon only: ▶
		//   nothing:   (omitted)
		var playStatus string
		if !a.showMiniPlayer() && a.currentTrack != nil {
			titleW := strWidth(title)
			countW := strWidth(countText)
			// -2 = small gap between count and status block
			remainW := innerW - titleW - countW - 2
			playStatus = a.buildCollapsedPlayStatus(remainW)
		}

		if playStatus != "" {
			// Right-align the playback status within the remaining header space.
			titleAndCount := title + count
			usedW := strWidth(title) + strWidth(countText)
			gap := innerW - usedW - strWidth(playStatus)
			if gap < 1 {
				gap = 1
			}
			sb.WriteString(titleAndCount + strings.Repeat(" ", gap) + playStatus + "\n")
		} else {
			sb.WriteString(title + count + "\n")
		}
	}
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2
	maxRows := innerH - headerLines
	if maxRows < 0 {
		maxRows = 0
	}

	start, end := visibleWindow(a.cursorPos, a.filteredLen(), maxRows)

	for i := start; i < end; i++ {
		t := a.filteredTrack(i)
		isSelected := i == a.cursorPos
		isPlaying := a.currentTrack != nil && a.currentTrack.ID == t.ID

		// ── Left: icon, fixed 2 display columns ───────────────────────────
		// The Nerd Font glyph 󰎆 occupies 2 terminal columns; the plain
		// space pair also occupies exactly 2.  We hard-code the width rather
		// than relying on strWidth which may mis-measure private-use glyphs.
		const leftFixW = 2
		icon := "  " // 2 spaces (no-play state)
		if isPlaying {
			icon = iconPlaying() + " " // glyph(1 col) + 1 space = 2 cols
		}

		// ── Right: format + duration (fixed 10 cols, right-aligned) ──────
		const rightColW = 10
		rightText := t.Format() + " " + formatDuration(t.Duration)
		rightPadded := padLeft(rightText, rightColW)

		// ── Middle: Album · Artist · Title (elastic, marquee on selected) ─
		// Total line width = leftFixW + midAvail + rightColW = innerW
		midAvail := innerW - leftFixW - rightColW
		if midAvail < 4 {
			midAvail = 4
		}

		midText := rowMidText(t)
		var midPadded string
		if isSelected {
			midPadded = a.mqRow.Render(midAvail)
			if isPlaying {
				midPadded = gradientText(strings.TrimRight(midPadded, " "), true, gradientColors...) +
					padRight("", midAvail-strWidth(strings.TrimRight(midPadded, " ")))
			}
		} else {
			if strWidth(midText) <= midAvail {
				if isPlaying {
					midPadded = gradientText(midText, true, gradientColors...) +
						padRight("", midAvail-strWidth(midText))
				} else {
					midPadded = padRight(midText, midAvail)
				}
			} else {
				cut := truncate(midText, midAvail)
				if isPlaying {
					midPadded = gradientText(cut, true, gradientColors...) +
						padRight("", midAvail-strWidth(cut))
				} else {
					midPadded = padRight(cut, midAvail)
				}
			}
		}

		// For playing rows: colour the icon and right column with the playing
		// accent so they match the gradient on the mid column visually.
		var line string
		if isPlaying {
			line = styleTrackRowPlayingAccent.Render(icon) + midPadded + styleTrackRowPlayingAccent.Render(rightPadded)
		} else {
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

	// TrimRight removes the trailing newline that would cause lipgloss to add
	// an extra blank row when padding content up to Height.
	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// buildCollapsedPlayStatus builds a compact playback-status string that fits
// within availW columns.  It is shown on the right side of the Library header
// when the right panel is collapsed or the terminal is too narrow.
//
// Layout (left → right): mode-icon  play-icon  title · artist  time
//
// Drop order strictly right-to-left (rightmost element first):
//  1. mode + title[+artist] + time   (full)
//  2. mode + title[+artist]          (drop time)
//  3. mode + title                   (drop artist)
//  4. mode + title (Marquee)         (title scrolls when too wide)
//  5. mode                           (drop title — mode icon survives last)
//  6. ""                             (nothing fits)
func (a *App) buildCollapsedPlayStatus(availW int) string {
	if availW < 1 || a.currentTrack == nil {
		return ""
	}

	// ── Fixed-width pieces ────────────────────────────────────────────────
	pos := a.player.Position()
	dur := a.player.Duration()

	timeStr := formatDuration(pos) + "/" + formatDuration(dur)
	const timeGap = 1
	timeW := strWidth(timeStr) + timeGap

	// ── Natural text widths ───────────────────────────────────────────────
	titleNatW := strWidth(a.mqTitle.text)
	artistNatW := strWidth(a.mqArtist.text)
	const sep = " · "
	const sepW = 3

	renderNatural := func(mq *Marquee, natW, maxW int) (string, int) {
		if maxW <= 0 {
			return "", 0
		}
		if natW <= maxW {
			return mq.text, natW
		}
		return mq.Render(maxW), maxW
	}

	// ── Play mode icon (lowest priority — dropped first) ──────────────────
	modeRune := iconPlayMode(a.playMode)
	modeStr := styleModeIcon.Render(modeRune)
	modeCols := strWidth(modeRune)
	const modeGap = 1
	modeW := modeCols + modeGap

	textNatW := titleNatW
	if artistNatW > 0 {
		textNatW += sepW + artistNatW
	}

	renderBlock := func(textAvail int) string {
		tStr, tUsed := renderNatural(a.mqTitle, titleNatW, textAvail)
		tRendered := styleTrackRowPlayingAccent.Render(tStr)
		artistAvail := textAvail - tUsed - sepW
		if artistAvail < 1 || artistNatW == 0 {
			return tRendered
		}
		aStr, _ := renderNatural(a.mqArtist, artistNatW, artistAvail)
		return tRendered + styleTrackMeta.Render(sep) + styleTrackMeta.Render(aStr)
	}

	// Variant 1: mode + title[+artist] + time
	if availW-modeW-timeW >= textNatW {
		return modeStr + " " + renderBlock(availW-modeW-timeW) +
			" " + styleTrackMeta.Render(timeStr)
	}

	// Variant 2: mode + title[+artist]  (drop time)
	if availW-modeW >= textNatW {
		return modeStr + " " + renderBlock(availW-modeW)
	}

	// Variant 3: mode + title  (drop artist)
	if availW-modeW >= titleNatW {
		return modeStr + " " + styleTrackRowPlayingAccent.Render(a.mqTitle.text)
	}

	// Variant 4: mode + title  (title too wide → Marquee scroll)
	if availW-modeW >= 1 {
		tStr, _ := renderNatural(a.mqTitle, titleNatW, availW-modeW)
		return modeStr + " " + styleTrackRowPlayingAccent.Render(tStr)
	}

	// Variant 5: mode only
	if availW >= modeCols {
		return modeStr
	}

	return ""
}
