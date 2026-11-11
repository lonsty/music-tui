package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lonsty/music-tui/internal/audio"
)

// ── Mini player ───────────────────────────────────────────────────────────────

func (a *App) renderMiniPlayer() string {
	innerW := a.miniPlayerW()
	innerH := a.panelInnerH()

	content := a.buildMiniPlayerContent(innerW, innerH)
	centered := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, content)
	return stylePanelBorder.Width(innerW).Height(innerH).Render(centered)
}

func (a *App) buildMiniPlayerContent(w, h int) string {
	// Stopped state: do NOT show the cover art placeholder box.
	// Just show the text prompt so the UI is clean and consistent.
	if a.currentTrack == nil {
		idle := lipgloss.JoinVertical(lipgloss.Center,
			"",
			stylePlayerArtist.Width(w).Render(T("no_track_selected")),
			stylePlayerMuted().Width(w).Render(T("press_enter_to_play")),
		)
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, idle)
	}

	// Fixed rows consumed by everything below the cover art:
	//   ""(1) + title(1) + meta(1) + ""(1) + div(1) + lyric(1) + div(1) +
	//   ""(1) + bar(1) + timeStr(1) + ""(1) + controls(1) = 12
	// One blank row above and below the cover box → +2 padding rows.
	const belowRows = 12
	const coverPad = 2
	coverAvailRows := h - belowRows - coverPad
	// Largest visual square: outerCols = outerRows*2, both dimensions constrained.
	// maxOuterCols is panel inner width minus 2 for a small horizontal margin.
	coverOuterRows := coverAvailRows
	if maxCols := (w - 2); coverOuterRows*2 > maxCols {
		coverOuterRows = maxCols / 2
	}
	if coverOuterRows < coverMinSize {
		coverOuterRows = coverMinSize
	}
	coverOuterCols := coverOuterRows * 2

	cover := a.getCoverArt(coverOuterCols, coverOuterRows)
	coverLine := styleCentered.Width(w).Render(cover)

	div := styleDivider.Render(strings.Repeat("─", w-2))

	// Track info — use Marquee for scrolling when text overflows.
	titleAvail := w - 2
	titleText := a.mqTitle.RenderCentered(titleAvail)
	title := centeredGradientText(titleText, titleAvail)

	metaAvail := w - 2
	metaText := a.mqMeta.RenderCentered(metaAvail)
	meta := stylePlayerArtist.Render(metaText)

	// Current lyric line — three states:
	//   loading:    provider fetch in-flight → show spinner text
	//   no lyrics:  provider returned nil   → show placeholder
	//   has lyrics: show active line or "…" before first timed line
	var lyricText string
	switch {
	case a.lyricsLoading:
		lyricText = lyricsLoadingText()
	case a.activeIdx >= 0 && a.activeIdx < len(a.lines):
		lyricText = a.lines[a.activeIdx].Text
	case len(a.lines) > 0:
		// Has lyrics but no active line yet (plain-text or before first stamp).
		lyricText = a.lines[0].Text
	default:
		lyricText = lyricsNoneText()
	}
	lyric := styleLyricNormal.Align(lipgloss.Center).Width(w).Render(lyricText)

	// Progress
	pos := a.framePlayback.position
	dur := a.framePlayback.duration
	pct := progressPct(pos, dur)
	// -4 = charmbracelet/progress adds 1-col padding each side (-2)
	//      + lipgloss.Center wrapper adds 1 col each side (-2).
	a.progressBar.Width = w - 4
	bar := styleCentered.Width(w).Render(a.progressBar.ViewAs(pct))
	timeStr := styleTime.Width(w).Align(lipgloss.Center).
		Render(fmt.Sprintf("%s / %s", formatDuration(pos), formatDuration(dur)))

	// Controls
	controls := a.buildControls(w)

	return lipgloss.JoinVertical(lipgloss.Center,
		coverLine,
		"",
		title,
		meta,
		"",
		div,
		lyric,
		div,
		"",
		bar,
		timeStr,
		"",
		controls,
	)
}

func (a *App) buildControls(w int) string {
	state := a.framePlayback.state
	playIcon := iconPlay()
	if state == audio.StatePlaying {
		playIcon = iconPause()
	}

	modeIcon := styleModeIcon.Render(iconPlayMode(a.playMode))
	volIcon := iconVolumeOn()
	if a.volume == 0 {
		volIcon = iconVolumeMute()
	}
	volPct := volumePercent(a.volume)
	volText := fmt.Sprintf("%d%%", volPct)
	if a.volume > 1.0 {
		// Above unity gain the slider stays at 100%; the plus marks boost.
		volText = "100%+"
	}

	// Heart icon: filled when the current track is in Favorites, empty otherwise.
	// Only show when an icon is available (not empty in Plain mode).
	heartIcon := iconHeartEmpty()
	if a.isFavorite {
		heartIcon = styleModeIcon.Render(iconHeartFilled())
	}
	var heartPart string
	if heartIcon != "" {
		heartPart = "    " + heartIcon
	}

	ctrl := iconPrev() + "  " + playIcon + "  " + iconNext() + "    " + modeIcon + "  " + volIcon + " " + volText + heartPart
	return styleControls.Width(w).Render(ctrl)
}

// volumePercent maps the volume knob to the displayed 0-100 scale.
// 1.0 (unity gain) is 100%; the boost range above it is reported separately
// as a "+" marker rather than as a percentage over 100.
func volumePercent(v float64) int {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 100
	}
	return int(math.Round(v * 100))
}
