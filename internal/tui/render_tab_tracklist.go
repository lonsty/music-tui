package tui

import (
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
		{tabPlaylist, iconPlaylist(), T("tab_playlist")},
		{tabLocal, iconLibrary(), T("tab_local")},
		{tabOnline, iconOnline(), T("tab_online")},
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
			sep = " " // gap between icon and label only when icon is present
		}
		body := ic + sep + t.label
		if t.id == a.activeTab {
			mark := styleTabActive.Render(activeMark)
			parts = append(parts, mark+styleTabActive.Render(body)+styleTabActive.Render(" "))
		} else {
			parts = append(parts, styleTabInactive.Render(inactiveMark+body+" "))
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
	left := a.renderSharedTrackList(a.localTrackListCtx(), a.trackListInnerW(), a.panelInnerH())
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
		label := styleOverlayValue.Render(" " + f.text)
		lines = append(lines, " "+icon+label)
	}
	lines = append(lines,
		"",
		styleOverlayMuted.Render(T("online_back_hint")),
	)

	block := lipgloss.JoinVertical(lipgloss.Left, lines...)
	centered := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, block)
	return stylePanelBorder.Width(innerW).Height(innerH).Render(centered)
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

	// renderBlock renders title[+artist] tightly (no trailing padding).
	// The returned string width equals the actual content width, not textAvail.
	// This lets the caller's gap calculation produce correct right-alignment
	// without the title slot being artificially padded with spaces.
	//
	// Title goes through mqTitle.Render() when it overflows textAvail so the
	// marquee scrolling offset is visible.  When it fits, mq.text is used
	// directly (no padding) — the offset is irrelevant when text fits anyway.
	renderBlock := func(textAvail int) string {
		if textAvail <= 0 {
			return ""
		}
		if titleNatW+sepW+artistNatW <= textAvail {
			// Both title and artist fit — show both, no truncation.
			if artistNatW > 0 {
				return styleTrackRowPlayingAccent.Render(a.mqTitle.text) +
					styleTrackMeta.Render(sep+a.mqArtist.text)
			}
			return styleTrackRowPlayingAccent.Render(a.mqTitle.text)
		}
		if titleNatW <= textAvail {
			// Title fits but artist doesn't — show title only, no padding.
			return styleTrackRowPlayingAccent.Render(a.mqTitle.text)
		}
		// Title overflows — use marquee window (scrolling, no padding).
		tStr := a.mqTitle.Render(textAvail)
		return styleTrackRowPlayingAccent.Render(strings.TrimRight(tStr, " "))
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

	// Variant 3: mode + title (title may overflow → renderBlock handles marquee)
	if availW-modeW >= 1 {
		return modeStr + " " + renderBlock(availW-modeW)
	}

	// Variant 5: mode only
	if availW >= modeCols {
		return modeStr
	}

	return ""
}
