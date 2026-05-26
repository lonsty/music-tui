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

	// Pre-compute the maximum displayed text width across all lines so that
	// the active-line rules always have a consistent length regardless of
	// which line is currently active.
	maxTextW := 0
	for _, l := range lines {
		if tw := strWidth(l.Text); tw > maxTextW {
			maxTextW = tw
		}
	}

	// ── Determine the centre line index ────────────────────────────────────
	// In browse mode (browseOffset != 0) the centre follows the browse cursor.
	// In follow mode (browseOffset == 0) the centre follows activeIdx.
	isBrowsing := a.browseOffset != 0

	var centerIdx int
	if !isBrowsing {
		// Follow-playback mode: same as before.
		// Use virtualActive=-1 before the first timed line so lyrics wait below.
		if active < 0 {
			// Synchronised lyrics before first line: show lines below centre.
			centerIdx = -1
		} else {
			centerIdx = active
		}
	} else {
		// Browse mode: clamp the browse cursor to valid range.
		centerIdx = active + a.browseOffset
		if centerIdx < 0 {
			centerIdx = 0
		}
		if total > 0 && centerIdx >= total {
			centerIdx = total - 1
		}
	}

	centerRow := h / 2
	var sb strings.Builder

	for row := 0; row < h; row++ {
		idx := centerIdx + (row - centerRow)
		dist := row - centerRow
		if dist < 0 {
			dist = -dist
		}

		if idx < 0 || idx >= total {
			sb.WriteString(lyricStyleForDistance(dist, false).Width(w).Render("") + "\n")
			continue
		}

		text := truncate(lines[idx].Text, w)

		var rendered string
		switch {
		case idx == centerIdx && !isBrowsing && active >= 0:
			// Follow mode, centre = active line: use the full decorated style.
			rendered = renderActiveLyricLine(text, w, maxTextW)

		case idx == centerIdx && isBrowsing:
			// Browse mode, centre line: bright colour + play icon, no rule deco.
			rendered = renderBrowseCursorLine(text, w, maxTextW)

		case isBrowsing && active >= 0 && idx == active:
			// Browse mode, the actual playing line (not at centre):
			// keep the mauve highlight so the user can still see where playback
			// is, but without any icon or rule decoration.
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

	stylePlain := lipgloss.NewStyle().
		Foreground(lipgloss.Color(subtext0)).
		Align(lipgloss.Center)

	var sb strings.Builder
	for row := 0; row < h; row++ {
		idx := offset + row
		if idx < total {
			text := truncate(lines[idx].Text, w)
			sb.WriteString(stylePlain.Width(w).Render(text) + "\n")
		} else {
			sb.WriteString(stylePlain.Width(w).Render("") + "\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// lyricDistanceColors defines the colour palette for non-active lines.
// Index 0 corresponds to distance=1 (immediately adjacent to active),
// since the active line itself is always rendered in mauve+bold and does
// not use this table.  Each subsequent index is one step dimmer.
// The gradient is stretched over 10 steps for a gradual transition.
var lyricDistanceColors = []string{
	subtext1, // dist 1 — immediately adjacent (NOT mauve — that's active only)
	subtext1, // dist 2
	subtext0, // dist 3
	subtext0, // dist 4
	overlay2, // dist 5
	overlay2, // dist 6
	overlay1, // dist 7
	overlay1, // dist 8
	overlay0, // dist 9
	overlay0, // dist 10+ — most distant / out-of-range
}

// lyricStyleForDistance returns a lipgloss.Style for a lyric line that is dist
// rows away from the active line.
//   - dist == 0 (active line): mauve + Bold — the active style is handled by
//     renderActiveLyricLine, but this function is kept consistent.
//   - dist >= 1: look up lyricDistanceColors[dist-1], clamped to the last entry.
func lyricStyleForDistance(dist int, isActive bool) lipgloss.Style {
	var colour string
	if dist == 0 || isActive {
		colour = mauve
	} else {
		idx := dist - 1
		if idx >= len(lyricDistanceColors) {
			idx = len(lyricDistanceColors) - 1
		}
		colour = lyricDistanceColors[idx]
	}
	s := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colour)).
		Align(lipgloss.Center)
	if isActive {
		s = s.Bold(true)
	}
	return s
}

// renderActiveLyricLine renders the currently playing lyric line framed by a
// pair of horizontal rules with a smooth per-character colour gradient:
//
//	overlay0 ·········· subtext0  text  subtext0 ·········· overlay0
//	(dim outer)         (bright inner)  (bright inner)      (dim outer)
//
// The gradient is computed by gradientText so every ─ character gets its own
// interpolated colour — no visible colour bands.
//
// Layout:
//   - margin: 4-column gap between the panel edge and the outermost ─.
//   - fixed rule width: derived from maxTextW so the rule length never
//     changes as different (shorter) lines become active.
//   - 1-space gap between the innermost ─ and the text on each side.
//   - short text is centred within the maxTextW slot via equal padding.
func renderActiveLyricLine(text string, w, maxTextW int) string {
	const margin = 4 // columns reserved on each side of the whole decoration
	const gap = 1    // spaces between rule end and text

	styleText := lipgloss.NewStyle().Foreground(lipgloss.Color(mauve)).Bold(true)

	textW := strWidth(text)

	ruleLen := (w - 2*margin - 2*gap - maxTextW) / 2
	if ruleLen < 2 {
		return styleText.Width(w).Render(text)
	}

	rule := strings.Repeat("─", ruleLen)

	// Left rule: outer (dim) → inner (bright), approaching the text.
	leftRule := gradientText(rule, false, overlay0, overlay1, overlay2, subtext0)
	// Right rule: inner (bright) → outer (dim), leaving the text.
	rightRule := gradientText(rule, false, subtext0, overlay2, overlay1, overlay0)

	sp := strings.Repeat(" ", gap)
	mid := styleText.Render(text)

	// Centre short text within maxTextW.
	padTotal := maxTextW - textW
	padL := padTotal / 2
	padR := padTotal - padL

	content := strings.Repeat(" ", margin) +
		leftRule +
		sp + strings.Repeat(" ", padL) + mid + strings.Repeat(" ", padR) + sp +
		rightRule +
		strings.Repeat(" ", margin)

	contentW := 2*margin + 2*ruleLen + 2*gap + maxTextW
	if contentW < w {
		pad := w - contentW
		content = strings.Repeat(" ", pad/2) + content + strings.Repeat(" ", pad-pad/2)
	}
	return content
}


// renderBrowseCursorLine renders the lyric line at the browse cursor position
// (centre of the panel in browse mode).  It uses the same gradient-rule
// decoration as the active playback line so the cursor position is visually
// consistent — the decoration indicates "press Enter to seek here".
//
// maxTextW is the widest lyric line in the set and is passed through to keep
// rule widths consistent with the rest of the panel.
func renderBrowseCursorLine(lyric string, w, maxTextW int) string {
	return renderActiveLyricLine(lyric, w, maxTextW)
}
