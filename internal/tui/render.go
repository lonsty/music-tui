package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/library"
)

// ── Catppuccin Mocha palette ─────────────────────────────────────────────────

const (
	crust   = "#11111B"
	mantle  = "#181825"
	base    = "#1E1E2E"
	surface0 = "#313244"
	surface1 = "#45475A"
	surface2 = "#585B70"
	overlay0 = "#6C7086"
	overlay1 = "#7F849C"
	overlay2 = "#9399B2"
	subtext0 = "#A6ADC8"
	subtext1 = "#BAC2DE"
	text     = "#CDD6F4"
	lavender = "#B4BEFE"
	blue     = "#89B4FA"
	sapphire = "#74C7EC"
	sky      = "#89DCEB"
	teal     = "#94E2D5"
	green    = "#A6E3A1"
	yellow   = "#F9E2AF"
	peach    = "#FAB387"
	maroon   = "#EBA0AC"
	red      = "#F38BA8"
	mauve    = "#CBA6F7"
	pink     = "#F5C2E7"
	flamingo = "#F2CDCD"
	rosewater = "#F5E0DC"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	// ── Tab bar ──────────────────────────────────────────────────────────────
	// Single-line tabs: active tab has a distinct background + underline;
	// inactive tabs are dim. No separator row — tabBarH = 1.
	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mauve)).
			Background(lipgloss.Color(surface0)).
			Underline(true).
			PaddingLeft(2).PaddingRight(2)

	styleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0)).
				PaddingLeft(2).PaddingRight(2)

	styleTabBar = lipgloss.NewStyle().
			Background(lipgloss.Color(mantle))

	// ── Panels ───────────────────────────────────────────────────────────────
	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(surface1))

	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(lavender))

	// ── Track list ───────────────────────────────────────────────────────────
	styleTrackNormal = lipgloss.NewStyle().
				Foreground(lipgloss.Color(subtext0))

	styleTrackSelected = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(text)).
				Bold(true)

	styleTrackPlaying = lipgloss.NewStyle().
				Foreground(lipgloss.Color(blue)).
				Bold(true)

	styleTrackPlayingSelected = lipgloss.NewStyle().
					Background(lipgloss.Color(surface0)).
					Foreground(lipgloss.Color(blue)).
					Bold(true)

	styleTrackMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color(overlay0))

	// ── Mini / fullscreen player ─────────────────────────────────────────────
	stylePlayerTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(text)).
				Align(lipgloss.Center)

	stylePlayerArtist = lipgloss.NewStyle().
				Foreground(lipgloss.Color(subtext0)).
				Align(lipgloss.Center)

	stylePlayerAlbum = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay1)).
				Align(lipgloss.Center)

	styleTime = lipgloss.NewStyle().
			Foreground(lipgloss.Color(overlay0))

	styleControls = lipgloss.NewStyle().
			Foreground(lipgloss.Color(subtext1)).
			Align(lipgloss.Center)

	styleModeIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color(mauve)).
			Bold(true)

	styleDivider = lipgloss.NewStyle().
			Foreground(lipgloss.Color(surface1))

	styleLyricNormal = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay1)).
				Align(lipgloss.Center)

	styleLyricActive = lipgloss.NewStyle().
				Foreground(lipgloss.Color(mauve)).
				Bold(true).
				Align(lipgloss.Center)

	// ── Cover art placeholder ─────────────────────────────────────────────────
	styleCoverBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(blue)).
				Foreground(lipgloss.Color(blue)).
				Align(lipgloss.Center, lipgloss.Center)

	// ── Status bar ───────────────────────────────────────────────────────────
	// No background on the line itself — only chips have backgrounds.
	styleStatusLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color(overlay0))

	styleStatusKey = lipgloss.NewStyle().
			Background(lipgloss.Color(surface0)).
			Foreground(lipgloss.Color(subtext1)).
			Bold(true)

	styleStatusHintLabel = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0))

	styleStatusState = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(blue)).
				Bold(true)

	// ── Overlays ─────────────────────────────────────────────────────────────
	styleOverlayBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(mauve)).
			Background(lipgloss.Color(base)).
			Padding(1, 2)

	styleOverlayTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(mauve))

	styleOverlayKey = lipgloss.NewStyle().
			Background(lipgloss.Color(surface0)).
			Foreground(lipgloss.Color(lavender)).
			PaddingLeft(1).PaddingRight(1)

	styleOverlayValue = lipgloss.NewStyle().
				Foreground(lipgloss.Color(text))

	styleOverlayMuted = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0))

	// ── Search ───────────────────────────────────────────────────────────────
	styleSearchPrompt = lipgloss.NewStyle().
				Foreground(lipgloss.Color(yellow)).
				Bold(true)
)

// ── Top-level render ──────────────────────────────────────────────────────────

func (a *App) render() string {
	// Fullscreen player replaces everything except status bar.
	if a.currentView == viewFullscreen {
		return a.renderFullscreen()
	}

	// Overlays are rendered on top of the normal layout.
	switch a.activeOvl {
	case overlayHelp:
		return a.renderHelpOverlay()
	case overlayInfo:
		return a.renderInfoOverlay()
	case overlaySettings:
		return a.renderSettingsOverlay()
	}

	tab := a.renderTabBar()
	body := a.renderNormalBody()
	status := a.renderStatusBar()

	return strings.Repeat("\n", topPad) +
		lipgloss.JoinVertical(lipgloss.Left, tab, body, status)
}

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
		{tabLocal, "󰋌", "Local"},
		{tabOnline, "󰖟", "Online"},
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
	left := a.renderTrackList()
	if !a.showMiniPlayer() {
		return left
	}
	right := a.renderMiniPlayer()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
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
		count := styleTrackMeta.Render(fmt.Sprintf("  %d tracks", len(a.filtered)))
		sb.WriteString(title + count + "\n")
	}
	sb.WriteString(styleDivider.Render(strings.Repeat("─", innerW)) + "\n")

	const headerLines = 2
	maxRows := innerH - headerLines
	if maxRows < 0 {
		maxRows = 0
	}

	start, end := visibleWindow(a.cursor, len(a.filtered), maxRows)

	for i := start; i < end; i++ {
		t := a.filtered[i]
		isSelected := i == a.cursor
		isPlaying := a.currentTrack != nil && a.currentTrack.ID == t.ID

		// ── Left: icon, fixed 2 display columns ───────────────────────────
		// The Nerd Font glyph 󰎆 occupies 2 terminal columns; the plain
		// space pair also occupies exactly 2.  We hard-code the width rather
		// than relying on strWidth which may mis-measure private-use glyphs.
		const leftFixW = 2
		icon := "  "   // 2 spaces (no-play state)
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
				// Re-render the marquee output with gradient; the marquee
				// already truncated/padded to midAvail columns.
				midPadded = gradientText(strings.TrimRight(midPadded, " "), true, gradientColors...) +
					padRight("", midAvail-strWidth(strings.TrimRight(midPadded, " ")))
			}
		} else {
			// Use strWidth(midText) ≤ midAvail to avoid appending "…" when
			// the text fits exactly.
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

		line := icon + midPadded + rightPadded

		var style lipgloss.Style
		switch {
		case isPlaying && isSelected:
			// Gradient handles foreground; keep background + bold only.
			style = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0)).Bold(true)
		case isPlaying:
			// Gradient handles foreground; bold only.
			style = lipgloss.NewStyle().Bold(true)
		case isSelected:
			style = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(text)).Bold(true)
		default:
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(subtext0))
		}

		sb.WriteString(style.Render(line) + "\n")
	}

	// TrimRight removes the trailing newline that would cause lipgloss to add
	// an extra blank row when padding content up to Height.
	content := strings.TrimRight(sb.String(), "\n")
	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

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
	if coverOuterRows < 4 {
		coverOuterRows = 4
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
	// Build gradient title: strip padding, apply gradient, re-centre.
	titleCore := strings.TrimSpace(titleText)
	titleGrad := gradientText(titleCore, true, gradientColors...)
	pad := titleAvail - strWidth(titleCore)
	lpad, rpad := pad/2, pad-pad/2
	title := strings.Repeat(" ", lpad) + titleGrad + strings.Repeat(" ", rpad)

	metaAvail := w - 2
	metaText := a.mqMeta.RenderCentered(metaAvail)
	meta := stylePlayerArtist.Render(metaText)

	// Lyric placeholder
	lyric := styleLyricNormal.Align(lipgloss.Center).Width(w).Render("󰝚  暂无歌词")

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
	if coverOuterRows < 4 {
		coverOuterRows = 4
	}
	coverOuterCols := coverOuterRows * 2

	cover := a.getCoverArt(coverOuterCols, coverOuterRows)
	coverLine := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(cover)

	// Track info lines — Marquee scrolling for overflow.
	avail := w - 2
	// Title with gradient.
	titleCore := strings.TrimSpace(a.mqTitle.RenderCentered(avail))
	titleGrad := gradientText(titleCore, true, gradientColors...)
	tpad := avail - strWidth(titleCore)
	title := strings.Repeat(" ", tpad/2) + titleGrad + strings.Repeat(" ", tpad-tpad/2)
	artist := stylePlayerArtist.Width(w).Align(lipgloss.Center).
		Render(a.mqArtist.RenderCentered(avail))
	album := stylePlayerAlbum.Width(w).Align(lipgloss.Center).
		Render(a.mqAlbum.RenderCentered(avail))

	// Progress bar + time (single line each)
	pos := a.player.Position()
	dur := a.player.Duration()
	pct := progressPct(pos, dur)
	a.progressBar.Width = w - 6
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

	// Header takes 2 lines inside the border.
	headerLines := 2
	lyricsH := innerH - headerLines
	if lyricsH < 1 {
		lyricsH = 1
	}

	placeholder := lipgloss.JoinVertical(lipgloss.Center,
		styleLyricNormal.Render("暂无歌词"),
		"",
		styleOverlayMuted.Render("在线歌词将在后续版本提供"),
	)
	lyricsContent := lipgloss.Place(innerW, lyricsH,
		lipgloss.Center, lipgloss.Center, placeholder)

	header := stylePanelTitle.Render("󰝚  Lyrics") + "\n" +
		styleDivider.Render(strings.Repeat("─", innerW))
	content := header + "\n" + lyricsContent

	return stylePanelBorder.Width(innerW).Height(innerH).Render(content)
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (a *App) renderStatusBar() string {
	if a.loading {
		return styleStatusLine.Render("  󰔟  Scanning library…")
	}
	if a.scanErr != nil {
		return styleStatusLine.Render("  󰅚  " + a.scanErr.Error())
	}

	state := a.player.State()
	stateLabel := map[audio.State]string{
		audio.StateStopped: "  Stopped ",
		audio.StatePlaying: "  Playing ",
		audio.StatePaused:  "  Paused  ",
	}[state]
	stateChip := styleStatusState.Render(stateLabel)

	// Build hint chips: [key] label
	hint := func(key, label string) string {
		return styleStatusKey.Render(" "+key+" ") +
			styleStatusHintLabel.Render(" "+label+"  ")
	}

	var hints string
	if a.currentView == viewFullscreen {
		hints = hint("Esc", "Back") + hint("Spc", "Pause") +
			hint("n", "Next") + hint("p", "Prev") + hint("m", "Mode")
	} else {
		hints = hint("/", "Search") + hint("i", "Info") +
			hint("f", "Full") + hint("m", "Mode") +
			hint("?", "Help") + hint("q", "Quit")
	}

	if a.statusMsg != "" {
		hints = styleStatusHintLabel.Render("  " + a.statusMsg)
	}

	modeChip := styleModeIcon.Render(" " + playModeIcon(a.playMode) + " ")

	var retroChip string
	if a.retroIdx > 0 {
		retroChip = "  " + styleStatusState.Render(" "+retroLabel(a.retroIdx)+" ")
	}

	var chipChip string
	switch {
	case a.chipConverting:
		chipChip = "  " + styleStatusState.Render(" 8-bit Converting… ")
	case a.chipBusy && a.chipMode:
		chipChip = "  " + styleStatusState.Render(" 8-bit Switching… ")
	case a.chipBusy:
		chipChip = "  " + styleStatusState.Render(" 8-bit… ")
	case a.chipMode:
		chipChip = "  " + styleStatusState.Render(" 8-bit ")
	}

	line := " " + stateChip + "  " + modeChip + retroChip + chipChip + "  " + hints
	// No Width — don't pad with background colour to the right edge.
	return styleStatusLine.Render(line)
}

// ── Help overlay ──────────────────────────────────────────────────────────────

func (a *App) renderHelpOverlay() string {
	type binding struct{ key, action string }
	bindings := []binding{
		{"j / ↓", "Move down"},
		{"k / ↑", "Move up"},
		{"g / G", "Top / Bottom"},
		{"Enter", "Play  (2nd Enter → Fullscreen)"},
		{"Space", "Pause / Resume"},
		{"n / p", "Next / Previous"},
		{"m", "Cycle play mode"},
		{"b", "Toggle 8-bit chip mode  (converts + crossfades)"},
		{"r / R", "Lo-fi effect  lower / raise sample rate"},
		{",", "Settings  (music dir · p2chip options · Ctrl+R reload)"},
		{"/", "Search  (s: artist  a: album  t: title)"},
		{"i", "Track info"},
		{"f", "Toggle fullscreen"},
		{"+ / -", "Volume up / down"},
		{"Tab", "Switch tab"},
		{"?", "This help"},
		{"q", "Quit"},
	}

	title := styleOverlayTitle.Render("󰋼  Keyboard shortcuts")
	div := styleOverlayMuted.Render(strings.Repeat("─", 44))

	var rows []string
	rows = append(rows, title, div, "")
	for _, b := range bindings {
		// Use lipgloss Width for the key chip so ANSI codes don't skew %-padding.
		k := styleOverlayKey.Width(14).Render(b.key)
		v := styleOverlayValue.Render(b.action)
		rows = append(rows, "  "+k+"  "+v)
	}
	rows = append(rows, "", styleOverlayMuted.Render("  Any key to close"))

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Info overlay ──────────────────────────────────────────────────────────────

func (a *App) renderInfoOverlay() string {
	t := a.currentTrack
	if t == nil && a.cursor < len(a.filtered) {
		tc := a.filtered[a.cursor]
		t = &tc
	}

	// styleOverlayKey has PaddingLeft(1)+PaddingRight(1), so Width(W) fits
	// W-2 visible characters.  The longest label is "Album Artist" (12 chars),
	// requiring Width(14).  indent must equal 2(prefix) + 14(labelW) + 2(gap) = 18.
	const labelW = 14
	const valueW = 38
	const indent = "                  " // 18 spaces: 2 + 14 + 2

	title := styleOverlayTitle.Render("󰋽  Track Info")
	div := styleOverlayMuted.Render(strings.Repeat("─", labelW+valueW+6))

	// row renders a single label+value pair.
	// Long values are word-wrapped at valueW columns, continuation lines
	// are indented to align with the first value character.
	row := func(label, value string) []string {
		if value == "" {
			return nil
		}
		l := styleOverlayKey.Width(labelW).Render(label)
		// Wrap value into segments of at most valueW display columns.
		segments := wrapText(value, valueW)
		var result []string
		for i, seg := range segments {
			v := styleOverlayValue.Render(seg)
			if i == 0 {
				result = append(result, "  "+l+"  "+v)
			} else {
				result = append(result, indent+v)
			}
		}
		return result
	}

	var rows []string
	rows = append(rows, title, div, "")
	if t != nil {
		for _, lines := range [][]string{
			row("Title",        t.DisplayTitle()),
			row("Artist",       t.DisplayArtist()),
			row("Album Artist", t.AlbumArtist),
			row("Album",        t.Album),
			row("Year",         t.Year),
			row("Track",        t.TrackNumber),
			row("Genre",        t.Genre),
			row("Comment",      t.Comment),
			row("Duration",     formatDuration(t.Duration)),
			row("Format",       t.Format()),
			row("Path",         t.Path),
		} {
			rows = append(rows, lines...)
		}
	} else {
		rows = append(rows, styleOverlayMuted.Render("  No track selected"))
	}
	rows = append(rows, "", styleOverlayMuted.Render("  Any key to close"))

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Settings overlay ──────────────────────────────────────────────────────────

func (a *App) renderSettingsOverlay() string {
	const lineW = 52

	title := styleOverlayTitle.Render("  Settings")
	topDiv := styleOverlayMuted.Render(strings.Repeat("─", lineW))

	sectionLabel := func(label string) string {
		fill := lineW - len(label) - 4
		if fill < 0 {
			fill = 0
		}
		return styleOverlayMuted.Render("── " + label + " " + strings.Repeat("─", fill))
	}

	// ── Active-input highlight helper ─────────────────────────────────────
	labelStyle := func(active bool) lipgloss.Style {
		if active {
			return styleOverlayKey.Copy().Foreground(lipgloss.Color(mauve))
		}
		return styleOverlayKey
	}

	// ── Music Library section ─────────────────────────────────────────────
	// labelW(11) + indent(2+2) = 15; input width = lineW - 15
	// styleOverlayKey has PaddingLeft(1)+PaddingRight(1), so Width(11)
	// fits 9 visible chars — exactly "Directory".
	const inputW = lineW - 15
	dirActive := a.settingsActive == 0
	dirLabel := labelStyle(dirActive).Width(11).Render("Directory")
	// Show the current value truncated to inputW so the overlay never overflows.
	// The textinput widget handles cursor/editing; we display a preview when
	// the input is not active, and the live input.View() when it is.
	var dirView string
	if dirActive {
		a.musicDirInput.Width = inputW
		dirView = a.musicDirInput.View()
	} else {
		// Inactive: show truncated value so it never wraps.
		val := a.musicDirInput.Value()
		if strWidth(val) > inputW {
			// Show the tail of the path (most useful part).
			val = "…" + val[len(val)-inputW+1:]
		}
		dirView = styleOverlayValue.Render(val)
	}
	dirLine := "  " + dirLabel + "  " + dirView
	reloadKey := styleOverlayKey.Render(" Ctrl+R ")
	reloadHint := "  " + reloadKey + styleOverlayMuted.Render(" reload library  (adds new · removes missing)")

	// ── 8-bit Conversion section ──────────────────────────────────────────
	optsActive := a.settingsActive == 1
	optsLabel := labelStyle(optsActive).Width(11).Render("Options")
	var optsView string
	if optsActive {
		a.settingsInput.Width = inputW
		optsView = a.settingsInput.View()
	} else {
		val := a.settingsInput.Value()
		if strWidth(val) > inputW {
			val = truncate(val, inputW)
		}
		optsView = styleOverlayValue.Render(val)
	}
	optsLine := "  " + optsLabel + "  " + optsView
	optsHint := styleOverlayMuted.Render("  Extra options appended to the p2chip command.")
	optsEx := styleOverlayMuted.Render("  e.g.  --sf2 nes --onset 0.6")

	// ── Footer ────────────────────────────────────────────────────────────
	enterKey := styleOverlayKey.Render(" Enter ")
	escKey := styleOverlayKey.Render(" Esc ")
	tabKey := styleOverlayKey.Render(" Tab ")
	footer := "  " + enterKey + styleOverlayMuted.Render(" save  ·  ") +
		escKey + styleOverlayMuted.Render(" cancel  ·  ") +
		tabKey + styleOverlayMuted.Render(" switch field")

	rows := []string{
		title,
		topDiv,
		"",
		sectionLabel("Music Library"),
		"",
		dirLine,
		reloadHint,
		"",
		sectionLabel("8-bit Conversion  (p2chip)"),
		"",
		optsLine,
		"",
		optsHint,
		optsEx,
		"",
		footer,
	}

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// gradientText applies a linear colour gradient across the display columns of s.
// colors must contain at least two hex colour strings (e.g. "#89B4FA").
// Each rune is coloured by interpolating between adjacent colour stops based on
// its position in the total display-column count.
// bold controls whether each rune is also rendered in bold.
func gradientText(s string, bold bool, colors ...string) string {
	if len(s) == 0 || len(colors) < 2 {
		return s
	}
	totalW := strWidth(s)
	if totalW == 0 {
		return s
	}

	// Parse hex colours into RGB float64 triples.
	type rgb struct{ r, g, b float64 }
	stops := make([]rgb, len(colors))
	for i, c := range colors {
		c = strings.TrimPrefix(c, "#")
		if len(c) != 6 {
			stops[i] = rgb{1, 1, 1}
			continue
		}
		parse := func(s string) float64 {
			v := 0
			for _, ch := range s {
				v <<= 4
				switch {
				case ch >= '0' && ch <= '9':
					v += int(ch - '0')
				case ch >= 'a' && ch <= 'f':
					v += int(ch-'a') + 10
				case ch >= 'A' && ch <= 'F':
					v += int(ch-'A') + 10
				}
			}
			return float64(v) / 255.0
		}
		stops[i] = rgb{parse(c[0:2]), parse(c[2:4]), parse(c[4:6])}
	}

	interpolate := func(t float64) rgb {
		// t ∈ [0,1]; map onto [0, len(stops)-1]
		scaled := t * float64(len(stops)-1)
		lo := int(scaled)
		if lo >= len(stops)-1 {
			return stops[len(stops)-1]
		}
		frac := scaled - float64(lo)
		a, b := stops[lo], stops[lo+1]
		return rgb{
			a.r + (b.r-a.r)*frac,
			a.g + (b.g-a.g)*frac,
			a.b + (b.b-a.b)*frac,
		}
	}

	clamp := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 255
		}
		return uint8(v * 255)
	}

	var out strings.Builder
	col := 0
	for _, r := range s {
		rw := strWidth(string(r))
		t := float64(col) / float64(totalW-1)
		if totalW == 1 {
			t = 0
		}
		c := interpolate(t)
		hex := fmt.Sprintf("#%02X%02X%02X", clamp(c.r), clamp(c.g), clamp(c.b))
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		if bold {
			st = st.Bold(true)
		}
		out.WriteString(st.Render(string(r)))
		col += rw
	}
	return out.String()
}

// gradientColors is the default gradient palette used for playing-track text.
// blue → mauve → pink (Catppuccin Mocha).
var gradientColors = []string{blue, mauve, pink}

// strWidth returns the terminal display width of s (handles CJK, Nerd Font, etc.)
// using the same wcwidth table that lipgloss uses internally.
func strWidth(s string) int {
	return ansi.StringWidth(s)
}

// truncate shortens s so its display width ≤ maxW, appending "…" if cut.
func truncate(s string, maxW int) string {
	if strWidth(s) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "…"
	}
	// Walk runes and accumulate display width until we exceed maxW-1 (reserve 1 for "…").
	w := 0
	for i, r := range s {
		rw := ansi.StringWidth(string(r))
		if w+rw > maxW-1 {
			return s[:i] + "…"
		}
		w += rw
	}
	return s
}

// padRight pads s with spaces on the right until its display width equals targetW.
func padRight(s string, targetW int) string {
	w := strWidth(s)
	if w >= targetW {
		return s
	}
	return s + strings.Repeat(" ", targetW-w)
}

// padLeft pads s with spaces on the left until its display width equals targetW.
func padLeft(s string, targetW int) string {
	w := strWidth(s)
	if w >= targetW {
		return s
	}
	return strings.Repeat(" ", targetW-w) + s
}

// wrapText splits s into lines of at most maxW display columns.
// It tries to break at '/' or ' ' boundaries; if no such boundary exists
// within a segment it hard-breaks at maxW.
func wrapText(s string, maxW int) []string {
	if maxW <= 0 {
		return []string{s}
	}
	if strWidth(s) <= maxW {
		return []string{s}
	}

	var lines []string
	for strWidth(s) > maxW {
		// Find the last break point (/ or space) within maxW columns.
		breakAt := -1
		col := 0
		for i, r := range s {
			rw := strWidth(string(r))
			if col+rw > maxW {
				break
			}
			if r == '/' || r == ' ' {
				breakAt = i + len(string(r)) // break after the delimiter
			}
			col += rw
		}

		if breakAt <= 0 {
			// No break point found: hard-break at maxW columns.
			col = 0
			breakAt = len(s)
			for i, r := range s {
				rw := strWidth(string(r))
				if col+rw > maxW {
					breakAt = i
					break
				}
				col += rw
			}
		}

		lines = append(lines, s[:breakAt])
		s = strings.TrimLeft(s[breakAt:], " ")
	}
	if s != "" {
		lines = append(lines, s)
	}
	return lines
}

// rowMidText builds the middle-column text for a track list row:
//   Album · Artist · Title
//
// Parts that are empty are omitted.  The result is used both for direct
// display (non-selected rows) and as the Marquee source (selected row).
func rowMidText(t library.Track) string {
	var parts []string
	if a := t.Album; a != "" {
		parts = append(parts, a)
	}
	if ar := t.DisplayArtist(); ar != "Unknown Artist" || t.Artist != "" {
		parts = append(parts, ar)
	}
	parts = append(parts, t.DisplayTitle())
	return strings.Join(parts, " · ")
}

// formatDuration converts a duration to mm:ss.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	t := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", t/60, t%60)
}

// progressPct returns a value in [0, 1] for the progress bar.
func progressPct(pos, dur time.Duration) float64 {
	if dur <= 0 {
		return 0
	}
	p := float64(pos) / float64(dur)
	if p > 1 {
		return 1
	}
	return p
}

// visibleWindow computes [start, end) for the visible slice of a list.
func visibleWindow(cursor, total, maxRows int) (start, end int) {
	if total == 0 || maxRows <= 0 {
		return 0, 0
	}
	start = cursor - maxRows/2
	if start < 0 {
		start = 0
	}
	end = start + maxRows
	if end > total {
		end = total
		start = end - maxRows
		if start < 0 {
			start = 0
		}
	}
	return start, end
}

// buildCoverPlaceholderSized renders a cover art placeholder box with an
// explicit outer size (outerCols × outerRows in terminal cell units).
// The border occupies 1 cell on each side, so the icon area is
// (outerCols-2) × (outerRows-2).
func buildCoverPlaceholderSized(outerCols, outerRows int) string {
	innerCols := outerCols - 2
	innerRows := outerRows - 2
	if innerCols < 1 {
		innerCols = 1
	}
	if innerRows < 1 {
		innerRows = 1
	}
	icon := lipgloss.NewStyle().
		Foreground(lipgloss.Color(blue)).Bold(true).Render("󰎄")
	inner := lipgloss.Place(innerCols, innerRows, lipgloss.Center, lipgloss.Center, icon)
	return styleCoverBorder.Width(innerCols).Height(innerRows).Render(inner)
}

// stylePlayerMuted returns a centred, muted style (used for idle hint text).
func stylePlayerMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(overlay0)).Align(lipgloss.Center)
}

// retroLabel returns a short display label for a retro preset index.
// It formats the target sample rate into a concise string, e.g. "11k", "344".
func retroLabel(idx int) string {
	hz := audio.RetroPresetRate(idx)
	if hz <= 0 {
		return "off"
	}
	if hz >= 1000 {
		return fmt.Sprintf("%dk", hz/1000)
	}
	return fmt.Sprintf("%d", hz)
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
