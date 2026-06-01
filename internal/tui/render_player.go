package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lonsty/music-tui/internal/audio"
)

// ── Lyric line decoration constants ──────────────────────────────────────────

// lyricsLoadingText returns the status text shown while a lyrics fetch is in
// flight.  The text is localised via T() so it reflects the active language.
func lyricsLoadingText() string { return iconWithSpace(iconSpinner()) + T("lyrics_loading") }

// lyricsNoneText returns the placeholder shown in the mini player when no
// lyrics are available.  Icon prefix is always kept for visual consistency.
func lyricsNoneText() string { return iconWithSpace(iconLyrics()) + T("no_lyrics") }

// lyricRuleMargin is the number of blank columns between the panel edge and
// the outermost ─ character of the active/cursor line decoration.
const lyricRuleMargin = 4

// lyricRuleGap is the number of space characters between the innermost ─
// and the lyric text on each side.
const lyricRuleGap = 1

// lyricBrowseDecW is the display width of the browse-cursor left decoration:
// space(1) + U+F144 nf-fa-play icon(1) + space(1) + space(1) = 4 cols.
// Assumes the Nerd Font glyph renders as 1 terminal cell wide.
const lyricBrowseDecW = 4

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
	coverLine := styleCentered.Width(w).Render(cover)

	if a.currentTrack == nil {
		idle := lipgloss.JoinVertical(lipgloss.Center,
			coverLine,
			"",
			stylePlayerArtist.Width(w).Render(T("no_track_selected")),
			stylePlayerMuted().Width(w).Render(T("press_enter_to_play")),
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
	pos := a.player.Position()
	dur := a.player.Duration()
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
	state := a.player.State()
	playIcon := iconPlay()
	if state == audio.StatePlaying {
		playIcon = iconPause()
	}

	modeIcon := styleModeIcon.Render(iconPlayMode(a.playMode))
	volIcon := iconVolumeOn()
	if a.volume == 0 {
		volIcon = iconVolumeMute()
	}
	volPct := int(a.volume / maxVolume * 100)

	ctrl := iconPrev() + "  " + playIcon + "  " + iconNext() + "    " + modeIcon + "  " + volIcon + " " + fmt.Sprintf("%d%%", volPct)
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
			styleModeIcon.Render(iconMusic()+"\n\nNo track selected"))
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
	pos := a.player.Position()
	dur := a.player.Duration()
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
func (a *App) renderLyricsScroll(w, h int) string {
	lines := a.lines
	active := a.activeIdx
	total := len(lines)

	// Plain-text (unsynchronised) lyrics: no highlight, top-to-bottom display.
	if !a.synced {
		return a.renderLyricsPlain(w, h)
	}

	// Pre-compute the maximum displayed text width for consistent rule lengths.
	maxTextW := 0
	for _, l := range lines {
		if tw := strWidth(l.Text); tw > maxTextW {
			maxTextW = tw
		}
	}

	// Determine the line that sits at the centre of the visible window.
	//   browseCenterIdx >= 0  → browse mode: cursor is pinned to that line
	//   browseCenterIdx == -1 → follow mode: centre follows activeIdx
	isBrowsing := a.browseCenterIdx >= 0
	var centerIdx int
	if isBrowsing {
		centerIdx = a.browseCenterIdx // absolute, never moves on its own
	} else {
		// Follow-playback: use -1 before the first timed line so lyrics
		// wait below the centre line.
		if active < 0 {
			centerIdx = -1
		} else {
			centerIdx = active
		}
	}

	centerRow := h / 2
	var sb strings.Builder

	for row := 0; row < h; row++ {
		idx := centerIdx + (row - centerRow)
		dist := absInt(row - centerRow)

		if idx < 0 || idx >= total {
			sb.WriteString(lyricStyleForDistance(dist, false).Width(w).Render("") + "\n")
			continue
		}

		text := truncate(lines[idx].Text, w)

		var rendered string
		switch {
		case idx == centerIdx && !isBrowsing && active >= 0:
			// Follow mode, centre == playing line: full decoration.
			rendered = renderActiveLyricLine(text, w, maxTextW)

		case idx == centerIdx && isBrowsing && idx == active:
			// Browse mode, cursor happens to be on the playing line: same deco.
			rendered = renderActiveLyricLine(text, w, maxTextW)

		case idx == centerIdx && isBrowsing:
			// Browse mode, cursor on a non-playing line: neutral style + deco.
			rendered = renderBrowseCursorLine(text, w, maxTextW)

		case isBrowsing && active >= 0 && idx == active:
			// Browse mode, playing line is off-centre: mauve highlight, no deco.
			rendered = lyricStyleForDistance(0, true).Width(w).Render(text)

		default:
			rendered = lyricStyleForDistance(dist, false).Width(w).Render(text)
		}

		sb.WriteString(rendered + "\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderLyricsPlain renders unsynchronised (plain-text) lyrics with automatic
// progress-based scrolling.
//
// The scroll is driven by the playback position such that the centre row of
// the panel tracks the current line index proportionally to progress:
//
//	middleIdx = int(progress × (total - 1))
//	offset    = clamp(middleIdx - h/2, 0, max(0, total - h))
//
// This means:
//   - At progress=0 the first line is near the top.
//   - At progress=1 the last line is at the centre of the panel, with the
//     rows below it already visible — there is no "last line arrives late"
//     problem because the tail content appears before the track ends.
//
// When total ≤ h the content fits without scrolling (offset=0).
// All lines are rendered in subtext0 (medium brightness).
func (a *App) renderLyricsPlain(w, h int) string {
	lines := a.lines
	total := len(lines)

	offset := 0
	if total > h {
		pos := a.player.Position()
		dur := a.player.Duration()
		if dur > 0 {
			progress := float64(pos) / float64(dur)
			if progress > 1 {
				progress = 1
			}
			// Middle row of the panel tracks the current line.
			middleIdx := int(progress * float64(total-1))
			offset = middleIdx - h/2
			// Clamp to valid window.
			maxOffset := total - h
			if offset > maxOffset {
				offset = maxOffset
			}
			if offset < 0 {
				offset = 0
			}
		}
	}

	var sb strings.Builder
	for row := 0; row < h; row++ {
		idx := offset + row
		if idx < total {
			text := truncate(lines[idx].Text, w)
			sb.WriteString(styleLyricPlain.Width(w).Render(text) + "\n")
		} else {
			sb.WriteString(styleLyricPlain.Width(w).Render("") + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// lyricDistanceStyles holds pre-computed lipgloss styles for lyric lines at
// each distance from the active line, avoiding per-frame allocations.
//
//	[0]  dist==0, not active — mauve, centred (used for blank/empty rows)
//	[1]  dist==0, active     — mauve + Bold, centred
//	[2]  dist==1             — subtext1
//	[3]  dist==2             — subtext1
//	[4]  dist==3             — subtext0
//	[5]  dist==4             — subtext0
//	[6]  dist==5             — overlay2
//	[7]  dist==6             — overlay2
//	[8]  dist==7             — overlay1
//	[9]  dist==8             — overlay1
//	[10] dist>=9             — overlay0 (clamped)
//
// Gradient: subtext1(1-2) → subtext0(3-4) → overlay2(5-6) → overlay1(7-8) → overlay0(9+).
var lyricDistanceStyles [11]lipgloss.Style

func init() {
	mkStyle := func(colour string, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(lipgloss.Color(colour)).Align(lipgloss.Center)
		if bold {
			s = s.Bold(true)
		}
		return s
	}
	lyricDistanceStyles[0] = mkStyle(mauve, false)
	lyricDistanceStyles[1] = mkStyle(mauve, true)
	lyricDistanceStyles[2] = mkStyle(subtext1, false)
	lyricDistanceStyles[3] = mkStyle(subtext1, false)
	lyricDistanceStyles[4] = mkStyle(subtext0, false)
	lyricDistanceStyles[5] = mkStyle(subtext0, false)
	lyricDistanceStyles[6] = mkStyle(overlay2, false)
	lyricDistanceStyles[7] = mkStyle(overlay2, false)
	lyricDistanceStyles[8] = mkStyle(overlay1, false)
	lyricDistanceStyles[9] = mkStyle(overlay1, false)
	lyricDistanceStyles[10] = mkStyle(overlay0, false)
}

// lyricStyleForDistance returns a pre-computed lipgloss.Style for a lyric line
// that is dist rows away from the active line.
//   - dist == 0, isActive == true:  mauve + Bold (playing line)
//   - dist == 0, isActive == false: mauve (blank/padding row at centre)
//   - dist >= 1: fading gradient, clamped at dist==9
func lyricStyleForDistance(dist int, isActive bool) lipgloss.Style {
	if dist == 0 {
		if isActive {
			return lyricDistanceStyles[1]
		}
		return lyricDistanceStyles[0]
	}
	idx := dist + 1 // dist=1 → index 2, dist=2 → index 3, …
	if idx >= len(lyricDistanceStyles) {
		idx = len(lyricDistanceStyles) - 1
	}
	return lyricDistanceStyles[idx]
}

// renderActiveLyricLine renders the currently playing lyric line framed by a
// pair of horizontal rules with a smooth per-character colour gradient:
//
//	overlay0 ─ overlay1 ─ overlay2 ─ subtext0  text  subtext0 ─ overlay2 ─ overlay1 ─ overlay0
//	(dim outer)                       (bright)        (bright)                        (dim outer)
//
// The gradient is computed by gradientText so every ─ character gets its own
// interpolated colour — no visible colour bands.
//
// Layout:
//   - margin: lyricRuleMargin (4) columns between panel edge and outermost ─.
//   - fixed rule width: derived from maxTextW so the rule length never
//     changes as different (shorter) lines become active.
//   - gap: lyricRuleGap (1) space between innermost ─ and text on each side.
//   - short text is centred within the maxTextW slot via equal padding.
func renderActiveLyricLine(text string, w, maxTextW int) string {
	textW := strWidth(text)

	ruleLen := (w - 2*lyricRuleMargin - 2*lyricRuleGap - maxTextW) / 2
	if ruleLen < 2 {
		// Not enough room for the decorative rules — centre the text only.
		return styleLyricActiveText.Width(w).Align(lipgloss.Center).Render(text)
	}

	rule := strings.Repeat("─", ruleLen)

	// Left rule: outer (dim) → inner (bright), approaching the text.
	leftRule := gradientText(rule, false, overlay0, overlay1, overlay2, subtext0)
	// Right rule: inner (bright) → outer (dim), leaving the text.
	rightRule := gradientText(rule, false, subtext0, overlay2, overlay1, overlay0)

	mid := styleLyricActiveText.Render(text)

	// Centre short text within maxTextW.
	padTotal := maxTextW - textW
	padL := padTotal / 2
	padR := padTotal - padL

	// Use a Builder to avoid creating multiple intermediate strings.
	contentW := 2*lyricRuleMargin + 2*ruleLen + 2*lyricRuleGap + maxTextW
	outerPad := 0
	if contentW < w {
		outerPad = w - contentW
	}
	var sb strings.Builder
	sb.Grow(w)
	if outerPad > 0 {
		sb.WriteString(strings.Repeat(" ", outerPad/2))
	}
	sb.WriteString(strings.Repeat(" ", lyricRuleMargin))
	sb.WriteString(leftRule)
	sb.WriteString(strings.Repeat(" ", lyricRuleGap+padL))
	sb.WriteString(mid)
	sb.WriteString(strings.Repeat(" ", padR+lyricRuleGap))
	sb.WriteString(rightRule)
	sb.WriteString(strings.Repeat(" ", lyricRuleMargin))
	if outerPad > 0 {
		sb.WriteString(strings.Repeat(" ", outerPad-outerPad/2))
	}
	// contentW > w cannot occur here: ruleLen = (w - overhead - maxTextW)/2 uses
	// integer floor division, so 2*ruleLen ≤ w - overhead - maxTextW, giving
	// contentW ≤ w.  The case is therefore unreachable and needs no fallback.
	return sb.String()
}

// renderBrowseCursorLine renders the lyric line at the browse cursor position
// when it is NOT the currently playing line.
//
//	 (U+F144 nf-fa-play) — rendered in overlay1 via styleBorder.
//	                        The whole left decoration is lyricBrowseDecW (4) cols:
//	                        space(1) + icon(1) + space(1) + space(1).
//	──  — gradient rule with same colour stops as renderActiveLyricLine
//	      (overlay0→overlay1→overlay2→subtext0), but no mauve endpoint.
//	text — text (#CDD6F4, bright white), bold — clearly readable but not "playing".
//
// Note: styleIcon is defined for colour but its Render argument is empty ("");
// the icon character (U+F144) is embedded in styleLyricBrowseBorder.Render(" <icon>").
// Nerd Font glyph width is assumed to be 1 terminal cell.
//
// maxTextW fixes the rule width to the widest lyric line.
func renderBrowseCursorLine(lyric string, w, maxTextW int) string {
	lyricW := strWidth(lyric)

	// Available width after the left decoration for the rule+text block.
	// We reuse the same rule-length formula as renderActiveLyricLine so
	// the horizontal rules are the same width as the playback line.
	ruleLen := (w - 2*lyricRuleMargin - 2*lyricRuleGap - maxTextW) / 2

	if ruleLen < 2 || w-lyricBrowseDecW < 4 {
		// Not enough room for the decorative rules — centre the text only.
		return styleLyricBrowseText.Width(w).Align(lipgloss.Center).Render(lyric)
	}

	rule := strings.Repeat("─", ruleLen)
	// Rules gradient: overlay0→overlay1→overlay2→subtext0 (same stops,
	// colour stays neutral — no mauve so it doesn't look "playing").
	leftRule := gradientText(rule, false, overlay0, overlay1, overlay2, subtext0)
	rightRule := gradientText(rule, false, subtext0, overlay2, overlay1, overlay0)

	sp := strings.Repeat(" ", lyricRuleGap)
	midText := styleLyricBrowseText.Render(lyric)

	padTotal := maxTextW - lyricW
	padL := padTotal / 2
	padR := padTotal - padL

	// Build the rule+text block (same structure as renderActiveLyricLine
	// but without the outer margin — left decoration takes that space).
	block := leftRule +
		sp + strings.Repeat(" ", padL) + midText + strings.Repeat(" ", padR) + sp +
		rightRule

	// Left decoration replaces the left margin.
	leftDec := styleLyricBrowseBorder.Render(" ") + " " + styleLyricBrowseIcon.Render("") + " "

	// Right margin.
	rightMargin := strings.Repeat(" ", lyricRuleMargin)

	line := leftDec + block + rightMargin

	// lineW = leftDec(lyricBrowseDecW) + leftRule(ruleLen) + gap(lyricRuleGap)
	//       + maxTextW(text+padding) + gap(lyricRuleGap) + rightRule(ruleLen)
	//       + rightMargin(lyricRuleMargin).
	// padL+padR = maxTextW - lyricW is already folded into maxTextW.
	lineW := lyricBrowseDecW + 2*ruleLen + 2*lyricRuleGap + maxTextW + lyricRuleMargin
	if lineW < w {
		pad := w - lineW
		line = strings.Repeat(" ", pad/2) + line + strings.Repeat(" ", pad-pad/2)
	}
	// lineW > w is unreachable for the same reason as renderActiveLyricLine.
	return line
}

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
		idle := stylePlayerArtist.Width(w).Render(T("no_track_selected"))
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
	pos := a.player.Position()
	dur := a.player.Duration()
	pct := progressPct(pos, dur)
	a.progressBar.Width = w - 4
	bar := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).
		Render(a.progressBar.ViewAs(pct))
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
