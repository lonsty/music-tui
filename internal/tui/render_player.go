package tui

import (
	"fmt"
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
	coverLine := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(cover)

	if a.currentTrack == nil {
		idle := lipgloss.JoinVertical(lipgloss.Center,
			coverLine,
			"",
			stylePlayerArtist.Width(w).Render("No track selected"),
			stylePlayerMuted().Width(w).Render("Press Enter to play"),
		)
		return idle
	}

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
	case a.loading:
		lyricText = "󰔟  loading lyrics…"
	case a.activeIdx >= 0 && a.activeIdx < len(a.lines):
		lyricText = a.lines[a.activeIdx].Text
	case len(a.lines) > 0:
		// Has lyrics but no active line yet (plain-text or before first stamp).
		lyricText = a.lines[0].Text
	default:
		lyricText = "󰝚  No lyrics"
	}
	lyric := styleLyricNormal.Align(lipgloss.Center).Width(w).Render(lyricText)

	// Progress
	pos := a.player.Position()
	dur := a.player.Duration()
	pct := progressPct(pos, dur)
	a.progressBar.Width = w - 4 // -4: progress bar horizontal padding compensation
	bar := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).
		Render(a.progressBar.ViewAs(pct))
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
	state := a.player.State()
	playIcon := "󰐊" // play
	if state == audio.StatePlaying {
		playIcon = "󰏤" // pause
	}

	modeIcon := styleModeIcon.Render(playModeIcon(a.playMode))
	volIcon := "󰕾"
	if a.volume == 0 {
		volIcon = "󰖁"
	}
	volPct := int(a.volume / 2.0 * 100)

	ctrl := fmt.Sprintf("󰒮  %s  󰒭    %s  %s %d%%",
		playIcon, modeIcon, volIcon, volPct)
	return styleControls.Width(w).Render(ctrl)
}

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
			styleModeIcon.Render("󰎄\n\nNo track selected"))
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
	coverLine := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(cover)

	// Track info lines — Marquee scrolling for overflow.
	avail := w - 2
	// Title with gradient.
	title := centeredGradientText(a.mqTitle.RenderCentered(avail), avail)
	artist := stylePlayerArtist.Width(w).Align(lipgloss.Center).
		Render(a.mqArtist.RenderCentered(avail))
	album := stylePlayerAlbum.Width(w).Align(lipgloss.Center).
		Render(a.mqAlbum.RenderCentered(avail))

	// Progress bar + time (single line each)
	pos := a.player.Position()
	dur := a.player.Duration()
	pct := progressPct(pos, dur)
	a.progressBar.Width = w - 6 // -6: fullscreen player horizontal padding compensation
	barLine := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).
		Render(a.progressBar.ViewAs(pct))
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

	header := stylePanelTitle.Render("󰝚  Lyrics") + "\n" +
		styleDivider.Render(strings.Repeat("─", innerW))

	var lyricsContent string
	switch {
	case a.loading:
		spinner := styleLyricNormal.Render("󰔟  loading lyrics…")
		lyricsContent = lipgloss.Place(innerW, lyricsH,
			lipgloss.Center, lipgloss.Center, spinner)
	case len(a.lines) == 0:
		placeholder := lipgloss.JoinVertical(lipgloss.Center,
			styleLyricNormal.Render("No lyrics"),
			"",
			styleOverlayMuted.Render("Place a .lrc file next to the audio file"),
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
// Layout (h rows, centerRow = h/2):
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
func (a *App) renderLyricsScroll(w, h int) string {
	lines := a.lines
	active := a.activeIdx
	total := len(lines)

	// Plain-text (unsynchronised) lyrics: no highlight, top-to-bottom display.
	if !a.synced {
		return a.renderLyricsPlain(w, h)
	}

	// Synchronised lyrics before the first timestamp has been reached
	// (active == -1): treat as if active were at index -1 so that all lyrics
	// sit below the centre line, waiting to scroll up into view.
	// We achieve this by using a virtual active position of -1: the first
	// lyric line (index 0) will appear one row below centre.
	virtualActive := active
	if virtualActive < 0 {
		virtualActive = -1
	}

	centerRow := h / 2
	var sb strings.Builder

	for row := 0; row < h; row++ {
		idx := virtualActive + (row - centerRow)
		dist := row - centerRow
		if dist < 0 {
			dist = -dist
		}

		if idx < 0 || idx >= total {
			// Out-of-range (above first line or below last): blank line.
			sb.WriteString(lyricStyleForDistance(dist, false).Width(w).Render("") + "\n")
			continue
		}

		text := truncate(lines[idx].Text, w)
		isActive := active >= 0 && idx == active
		var rendered string
		if isActive {
			rendered = renderActiveLyricLine(text, w)
		} else {
			rendered = lyricStyleForDistance(dist, false).Width(w).Render(text)
		}
		sb.WriteString(rendered + "\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderLyricsPlain renders unsynchronised (plain-text) lyrics top-to-bottom
// without any active-line highlight.  All lines use the same dimmed colour.
func (a *App) renderLyricsPlain(w, h int) string {
	lines := a.lines
	total := len(lines)

	var sb strings.Builder
	plain := lyricStyleForDistance(len(lyricDistanceColors)-1, false)

	for row := 0; row < h; row++ {
		if row < total {
			text := truncate(lines[row].Text, w)
			sb.WriteString(plain.Width(w).Render(text) + "\n")
		} else {
			sb.WriteString(plain.Width(w).Render("") + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// lyricDistanceColors defines the colour palette used for the distance-based
// fade effect.  Index 0 is the active line (brightest); each subsequent index
// is one step dimmer.  The gradient is stretched over 12 steps so the colour
// transition is gradual rather than abrupt.  All colours are from the
// Catppuccin Mocha palette.
var lyricDistanceColors = []string{
	mauve,    // 0 — active line
	mauve,    // 1 — still very close
	subtext1, // 2
	subtext1, // 3
	subtext0, // 4
	subtext0, // 5
	overlay2, // 6
	overlay2, // 7
	overlay1, // 8
	overlay1, // 9
	overlay0, // 10
	overlay0, // 11+ — most distant / out-of-range
}

// lyricStyleForDistance returns a lipgloss.Style for a lyric line that is dist
// rows away from the active line.  The active line (dist=0) is rendered bold;
// all others are normal weight.
func lyricStyleForDistance(dist int, isActive bool) lipgloss.Style {
	maxIdx := len(lyricDistanceColors) - 1
	if dist > maxIdx {
		dist = maxIdx
	}
	s := lipgloss.NewStyle().
		Foreground(lipgloss.Color(lyricDistanceColors[dist])).
		Align(lipgloss.Center)
	if isActive {
		s = s.Bold(true)
	}
	return s
}

// renderActiveLyricLine renders the currently playing lyric line with a pair
// of dim horizontal rules on each side to visually anchor the centre position:
//
//	──── text ────
//
// The rules are coloured overlay0 (same as the most-distant lyric colour) so
// they frame the bright active line without competing with it.  The text
// itself is rendered in mauve+bold.  Total display width equals w.
func renderActiveLyricLine(text string, w int) string {
	const minRuleLen = 2  // minimum dashes on each side
	const rulePad    = 1  // spaces between rule and text

	styleText := lipgloss.NewStyle().
		Foreground(lipgloss.Color(mauve)).
		Bold(true)
	styleRule := lipgloss.NewStyle().
		Foreground(lipgloss.Color(overlay0))

	textW := strWidth(text)

	// Available width for both rules = total - text - 2*padding.
	available := w - textW - 2*rulePad
	if available < 2*minRuleLen {
		// Not enough space for rules; fall back to plain centred text.
		return styleText.Width(w).Render(text)
	}

	// Split available space evenly; left rule gets any remainder.
	ruleR := available / 2
	ruleL := available - ruleR

	left  := styleRule.Render(strings.Repeat("─", ruleL))
	right := styleRule.Render(strings.Repeat("─", ruleR))
	mid   := styleText.Render(text)
	space := strings.Repeat(" ", rulePad)

	return left + space + mid + space + right
}