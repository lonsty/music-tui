package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// ── Mini lyrics panel ─────────────────────────────────────────────────────────

// renderMiniLyrics renders the right panel in lyrics mode.
// The cover art is replaced by a scrolling lyrics panel identical to the
// fullscreen lyrics panel (minus the "Lyrics" header), and the track info +
// controls are kept below it.
//
// Layout (inside border):
//
//	""                    ← blank line (top padding)
//	[lyrics scroll area]  ← fills available height
//	""                    ← blank line
//	title (Marquee+gradient)
//	artist · album
//	progress bar
//	time
//	""                    ← blank line
//	controls
//	""                    ← blank line (bottom padding)
func (a *App) renderMiniLyrics() string {
	innerW := a.miniPlayerW()
	innerH := a.panelInnerH()

	content := a.buildMiniLyricsContent(innerW, innerH)
	centered := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, content)
	return stylePanelBorder.Width(innerW).Height(innerH).Render(centered)
}

func (a *App) buildMiniLyricsContent(w, h int) string {
	if a.currentTrack == nil {
		// Stopped state: no cover placeholder box — just text prompt.
		idle := lipgloss.JoinVertical(lipgloss.Center,
			"",
			stylePlayerArtist.Width(w).Render(T("no_track_selected")),
			stylePlayerMuted().Width(w).Render(T("press_enter_to_play")),
		)
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, idle)
	}

	// Fixed rows below the lyrics area:
	//   ""(1) + title(1) + meta(1) + ""(1) + bar(1) + time(1) + ""(1) + controls(1) + ""(1) = 9
	// One blank row above the lyrics area = 1 padding row.
	const belowRows = 9
	const topPadRows = 1
	lyricsH := h - belowRows - topPadRows
	if lyricsH < 1 {
		lyricsH = 1
	}

	// Lyrics area — same rendering as fullscreen, no header line.
	var lyricsContent string
	switch {
	case a.lyricsLoading:
		spinner := styleLyricNormal.Render(lyricsLoadingText())
		lyricsContent = lipgloss.Place(w, lyricsH, lipgloss.Center, lipgloss.Center, spinner)
	case len(a.lines) == 0:
		placeholder := styleLyricNormal.Render(T("no_lyrics"))
		lyricsContent = lipgloss.Place(w, lyricsH, lipgloss.Center, lipgloss.Center, placeholder)
	default:
		lyricsContent = a.renderLyricsScroll(w, lyricsH)
	}

	// Track info
	titleAvail := w - 2
	titleText := a.mqTitle.RenderCentered(titleAvail)
	title := centeredGradientText(titleText, titleAvail)

	metaAvail := w - 2
	metaText := a.mqMeta.RenderCentered(metaAvail)
	meta := stylePlayerArtist.Render(metaText)

	// Progress
	pos := a.framePlayback.position
	dur := a.framePlayback.duration
	pct := progressPct(pos, dur)
	a.progressBar.Width = w - 4
	bar := styleCentered.Width(w).Render(a.progressBar.ViewAs(pct))
	timeStr := styleTime.Width(w).Align(lipgloss.Center).
		Render(fmt.Sprintf("%s / %s", formatDuration(pos), formatDuration(dur)))

	// Controls
	controls := a.buildControls(w)

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		lyricsContent,
		"",
		title,
		meta,
		"",
		bar,
		timeStr,
		"",
		controls,
		"",
	)
}
