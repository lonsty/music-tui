package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Fullscreen player ─────────────────────────────────────────────────────────

func (a *App) renderFullscreen() string {
	left := a.renderFullPlayer()
	right := a.renderFullLyrics()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	status := a.renderStatusBar()
	return strings.Repeat("\n", topPad) +
		lipgloss.JoinVertical(lipgloss.Left, body, status)
}

func (a *App) renderFullPlayer() string {
	innerW := a.fullPlayerInnerW()
	innerH := a.fullBodyH()

	var content string
	if a.currentTrack == nil {
		content = lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center,
			stylePlayerArtist.Align(lipgloss.Center).Width(innerW).Render(T("no_track_selected")))
	} else {
		content = a.buildFullPlayerContent(innerW, innerH)
	}

	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

func (a *App) buildFullPlayerContent(w, h int) string {
	// Fixed rows consumed by everything below the cover art:
	//   ""(1) + title(1) + artist(1) + album(1) + ""(1) +
	//   barLine(1) + timeLine(1) + ""(1) + controls(1) = 9
	// One blank row above and below the cover box → +2 padding rows.
	const belowRows = 9
	const coverPad = 2
	coverAvailRows := h - belowRows - coverPad
	// Largest visual square: outerCols = outerRows*2, both dimensions constrained.
	coverOuterRows := coverAvailRows
	if maxCols := (w - 4); coverOuterRows*2 > maxCols {
		coverOuterRows = maxCols / 2
	}
	if coverOuterRows < coverMinSize {
		coverOuterRows = coverMinSize
	}
	coverOuterCols := coverOuterRows * 2

	cover := a.getCoverArt(coverOuterCols, coverOuterRows)
	coverLine := styleCentered.Width(w).Render(cover)

	// Track info lines — Marquee scrolling for overflow.
	avail := w - 2
	// Title with gradient.
	title := centeredGradientText(a.mqTitle.RenderCentered(avail), avail)
	artist := stylePlayerArtist.Width(w).Align(lipgloss.Center).
		Render(a.mqArtist.RenderCentered(avail))
	album := stylePlayerAlbum.Width(w).Align(lipgloss.Center).
		Render(a.mqAlbum.RenderCentered(avail))

	// Progress bar + time (single line each)
	pos := a.framePlayback.position
	dur := a.framePlayback.duration
	pct := progressPct(pos, dur)
	// -6 = same as mini player (-4) plus the fullscreen player panel has 1 extra col
	//      of interior padding on each side compared to the mini panel (-2 more).
	a.progressBar.Width = w - 6
	barLine := styleCentered.Width(w).Render(a.progressBar.ViewAs(pct))
	timeLine := styleTime.Width(w).Align(lipgloss.Center).
		Render(fmt.Sprintf("%s / %s", formatDuration(pos), formatDuration(dur)))

	// Controls
	controls := a.buildControls(w)

	// Stack without any hardcoded blank lines — Place will centre the block.
	block := lipgloss.JoinVertical(lipgloss.Center,
		coverLine,
		"",
		title,
		artist,
		album,
		"",
		barLine,
		timeLine,
		"",
		controls,
	)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, block)
}

func (a *App) renderFullLyrics() string {
	innerW := a.fullLyricsW()
	innerH := a.fullBodyH()

	const headerLines = 2
	lyricsH := innerH - headerLines
	if lyricsH < 1 {
		lyricsH = 1
	}

	header := stylePanelTitle.Render(iconWithSpace(iconLyrics())+T("lyrics_panel_title")) + "\n" +
		styleDivider.Render(strings.Repeat("─", innerW))

	var lyricsContent string
	switch {
	case a.lyricsLoading:
		spinner := styleLyricNormal.Render(lyricsLoadingText())
		lyricsContent = lipgloss.Place(innerW, lyricsH,
			lipgloss.Center, lipgloss.Center, spinner)
	case len(a.lines) == 0:
		placeholder := lipgloss.JoinVertical(lipgloss.Center,
			// lyricsNoneText() includes the icon prefix; strip it for the fullscreen
			// placeholder since the panel header already shows the lyrics icon.
			styleLyricNormal.Render(T("no_lyrics")),
			"",
			styleOverlayMuted.Render(T("lyrics_hint_lrc")),
		)
		lyricsContent = lipgloss.Place(innerW, lyricsH,
			lipgloss.Center, lipgloss.Center, placeholder)
	default:
		lyricsContent = a.renderLyricsScroll(innerW, lyricsH)
	}

	content := header + "\n" + lyricsContent
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// renderLyricsScroll renders the lyric lines with the active line fixed at the
// vertical centre of the panel.  Lines above and below fade out as they move
// away from the centre, creating a "spotlight" effect.
//
// Layout (h rows, centerRow = h/2 integer division):
// For odd h, there is one more row below centre than above.
//
//	row 0          → lines[active - centerRow]   (dimmed)
//	…
//	row centerRow  → lines[active]               (highlighted, mauve+bold)
//	…
//	row h-1        → lines[active + (h-1-centerRow)] (dimmed)
//
// Rows that map to out-of-range indices are rendered as blank lines in the
// most dimmed colour so the panel always fills its allocated height.
//
// When active < 0 (plain-text / unsynchronised lyrics) the function delegates
// to renderLyricsPlain which shows lines top-to-bottom without any highlight.
// maxLineWidth returns the widest line in display columns.
