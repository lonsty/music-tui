package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Tab bar ───────────────────────────────────────────────────────────────────

// renderTabBar renders a single-line tab bar (tabBarH = 1).
// The active tab is visually distinct via background + underline; no separator
// row is needed, which saves one line of vertical space.
func (a *App) renderTabBar() string {
	type tabDef struct {
		id    tabID
		icon  string
		label string
	}
	tabs := []tabDef{
		{tabLocal, "󰋌", T("tab_local")},
		{tabOnline, "󰖟", T("tab_online")},
	}

	var parts []string
	for _, t := range tabs {
		text := t.icon + "  " + t.label
		if t.id == a.activeTab {
			parts = append(parts, styleTabActive.Render(text))
		} else {
			parts = append(parts, styleTabInactive.Render(text))
		}
	}

	// Fill remaining width with tab bar background.
	return styleTabBar.Width(a.W).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, parts...),
	)
}

// ── Normal body (track list + mini player) ────────────────────────────────────

func (a *App) renderNormalBody() string {
	if a.activeTab == tabOnline {
		return a.renderOnlinePlaceholder()
	}
	left := a.renderTrackList()
	if !a.showMiniPlayer() {
		return left
	}
	right := a.renderMiniPlayer()
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
		title := stylePanelTitle.Render("󰋌  Library")
		count := styleTrackMeta.Render(fmt.Sprintf("  %d tracks", a.filteredLen()))
		sb.WriteString(title + count + "\n")
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
