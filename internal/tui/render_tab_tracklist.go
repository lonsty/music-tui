package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lonsty/music-tui/internal/audio"
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
	// Only show tabs with a working implementation.
	// tabPlaylist is declared but its UI is not yet built;
	// add it back here once the playlist panel is implemented.
	tabs := []tabDef{
		{tabLocal, "󰋌", T("tab_local")},
		{tabOnline, "󰖟", T("tab_online")},
	}

	// activeMark is the left marker rendered on the active tab.
	// A plain space of the same display width is used for inactive tabs so
	// that all tabs remain the same total width regardless of active state.
	const activeMark = "▌ "   // 1 col block + 1 space = 2 display cols
	const inactiveMark = "  " // 2 spaces — same width as activeMark

	var parts []string
	for _, t := range tabs {
		body := t.icon + "  " + t.label
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
		// Playlist UI is not yet implemented; fall through to the local list
		// so an accidental Tab keypress does not show a blank screen.
		fallthrough
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
		{"󰝚", "Online streaming & radio"},
		{"󰋌", "Netease / Spotify integration"},
		{"󰍋", "Lyrics sync & search"},
		{"󰒝", "Playlist discovery"},
	}

	var lines []string
	lines = append(lines,
		gradientText("󰖟  Online", true, blue, mauve, pink),
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
		prompt := styleSearchPrompt.Render("󰍉 ")
		sb.WriteString(prompt + a.searchInput.View() + "\n")
	} else {
		title := stylePanelTitle.Render("󰋌  " + T("library_title"))
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
			icon = "󰎆 " // glyph(1 col) + 1 space = 2 cols
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
//  1. mode + icon + title[+artist] + time   (full)
//  2. mode + icon + title[+artist]          (drop time)
//  3. mode + icon + title                   (drop artist)
//  4. mode + icon + title (Marquee)         (title scrolls when too wide)
//  5. mode + icon                           (drop title)
//  6. mode                                  (drop play icon — mode icon survives last)
//  7. ""                                    (nothing fits)
func (a *App) buildCollapsedPlayStatus(availW int) string {
	if availW < 2 || a.currentTrack == nil {
		return ""
	}

	// State icon: nf-fa-play or nf-fa-pause.
	iconRune := "󰐊" // nf-fa-play
	if a.player.State() == audio.StatePaused {
		iconRune = "󰏤" // nf-fa-pause
	}
	icon := styleTrackRowPlayingAccent.Render(iconRune)
	const iconCols = 1 // Nerd Font glyph renders as 1 terminal cell
	const iconGap = 1  // space after icon
	iconW := iconCols + iconGap

	if availW <= iconW {
		if availW >= iconCols {
			return icon
		}
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
	const sep = " · " // connector between title and artist — signals they form one unit
	const sepW = 3

	// renderNatural returns the text at its natural width when it fits within
	// maxW, otherwise returns a Marquee scroll window of exactly maxW columns.
	// Returns (rendered string, actual columns used).
	renderNatural := func(mq *Marquee, natW, maxW int) (string, int) {
		if maxW <= 0 {
			return "", 0
		}
		if natW <= maxW {
			return mq.text, natW
		}
		return mq.Render(maxW), maxW
	}

	// ── Try variants from widest to narrowest ────────────────────────────
	// Each variant is selected only when the available space can fit the
	// natural (unscrolled) widths of all its components.  Once a variant is
	// chosen, text that is still too wide gets a Marquee scroll window rather
	// than being truncated.  This ensures that elements are dropped (right to
	// left) before any scrolling is introduced.
	//
	// Progress bar is intentionally omitted: at header-row scale (8 cols) it
	// adds visual noise without conveying more information than the time string.

	// ── Play mode icon (lowest priority — shown only when all other components fit) ──
	modeRune := playModeIcon(a.playMode)
	modeStr := styleModeIcon.Render(modeRune)
	const modeCols = 1 // Nerd Font glyph renders as 1 terminal cell
	const modeGap = 1  // space after mode icon
	modeW := modeCols + modeGap

	// Helper: minimum cols needed for title[+artist] given fixed overhead.
	// "natural" means both title and artist at their raw display widths.
	textNatW := titleNatW // title alone
	if artistNatW > 0 {
		textNatW += sepW + artistNatW // title + sep + artist
	}

	// renderBlock renders "title · artist" within textAvail cols, using natural
	// widths when they fit; only falls back to Marquee when the text itself
	// is wider than the slot.
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

	// Variants: strictly right-to-left drop order.
	// Each step removes the rightmost remaining element.

	// Variant 1: mode + icon + title[+artist] + time
	if availW-modeW-iconW-timeW >= textNatW {
		return modeStr + " " + icon + " " + renderBlock(availW-modeW-iconW-timeW) +
			" " + styleTrackMeta.Render(timeStr)
	}

	// Variant 2: mode + icon + title[+artist]  (drop time)
	if availW-modeW-iconW >= textNatW {
		return modeStr + " " + icon + " " + renderBlock(availW-modeW-iconW)
	}

	// Variant 3: mode + icon + title  (drop artist)
	if availW-modeW-iconW >= titleNatW {
		return modeStr + " " + icon + " " + styleTrackRowPlayingAccent.Render(a.mqTitle.text)
	}

	// Variant 4: mode + icon + title  (title too wide → Marquee scroll)
	if availW-modeW-iconW >= 1 {
		tStr, _ := renderNatural(a.mqTitle, titleNatW, availW-modeW-iconW)
		return modeStr + " " + icon + " " + styleTrackRowPlayingAccent.Render(tStr)
	}

	// Variant 5: mode + icon  (drop title)
	if availW >= modeW+iconCols {
		return modeStr + " " + icon
	}

	// Variant 6: mode only  (drop play icon — mode is the last surviving element)
	if availW >= modeCols {
		return modeStr
	}

	return ""
}
