package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/eilianxiao/music-tui/internal/audio"
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
	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mauve)).
			Background(lipgloss.Color(surface0)).
			PaddingLeft(2).PaddingRight(2).
			BorderStyle(lipgloss.ThickBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color(mauve))

	styleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0)).
				PaddingLeft(2).PaddingRight(2).
				BorderStyle(lipgloss.ThickBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color(surface0))

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
	}

	tab := a.renderTabBar()
	body := a.renderNormalBody()
	status := a.renderStatusBar()

	return strings.Repeat("\n", topPad) +
		lipgloss.JoinVertical(lipgloss.Left, tab, body, status)
}

// ── Tab bar ───────────────────────────────────────────────────────────────────

func (a *App) renderTabBar() string {
	tabs := []struct {
		id    tabID
		icon  string
		label string
	}{
		{tabLocal, "󰋌", "Local"},
		{tabOnline, "󰖟", "Online"},
	}

	var parts []string
	for _, t := range tabs {
		label := t.icon + "  " + t.label
		if t.id == a.activeTab {
			parts = append(parts, styleTabActive.Render(label))
		} else {
			parts = append(parts, styleTabInactive.Render(label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, parts...)
	// Pad the bar to full terminal width with the background colour.
	padded := styleTabBar.Width(a.W).Render(bar)
	return padded
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

		// Playing icon (2 runes wide)
		icon := "  "
		if isPlaying {
			icon = "󰎆 "
		}

		// Right column: "MP3 04:58" — fixed width, right-aligned.
		rightText := t.Format() + " " + formatDuration(t.Duration)
		const rightColW = 11 // "FLAC 00:00" = 9, pad to 11

		// Left column: fills the rest.
		leftAvail := innerW - rightColW - 1 // -1 for separator space
		leftText := icon + t.DisplayArtist() + " — " + t.DisplayTitle()
		leftText = truncate(leftText, leftAvail)

		// Base style shared by all rows — Width ensures background fills the line.
		base := lipgloss.NewStyle().Width(innerW)
		switch {
		case isPlaying && isSelected:
			base = base.Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(blue)).Bold(true)
		case isPlaying:
			base = base.Foreground(lipgloss.Color(blue)).Bold(true)
		case isSelected:
			base = base.Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(text)).Bold(true)
		default:
			base = base.Foreground(lipgloss.Color(subtext0))
		}

		// Inline-render: left text padded to leftAvail, then space, then right.
		// We build the full line as a single string and let base.Width do the
		// background fill — no manual space-padding needed.
		leftPart := lipgloss.NewStyle().
			Width(leftAvail).
			Inherit(base).
			MaxWidth(leftAvail).
			Render(leftText)
		rightPart := lipgloss.NewStyle().
			Width(rightColW).
			Align(lipgloss.Right).
			Inherit(base).
			Render(rightText)
		rendered := base.Render(leftPart + " " + rightPart)

		sb.WriteString(rendered + "\n")
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
	if a.currentTrack == nil {
		idle := lipgloss.JoinVertical(lipgloss.Center,
			styleModeIcon.Render("󰎄"),
			"",
			stylePlayerArtist.Width(w).Render("No track selected"),
			stylePlayerMuted().Width(w).Render("Press Enter to play"),
		)
		return idle
	}

	t := a.currentTrack
	div := styleDivider.Render(strings.Repeat("─", w-2))

	// Track info
	title := stylePlayerTitle.Width(w).Render(truncate(t.DisplayTitle(), w-2))
	meta := stylePlayerArtist.Width(w).Render(
		truncate(t.DisplayArtist()+" · "+t.Album, w-2),
	)

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
		"",
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
	t := a.currentTrack

	// Cover placeholder
	coverW := min(w-4, 16)
	coverH := coverW / 2
	coverInner := lipgloss.Place(coverW-2, coverH,
		lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Foreground(lipgloss.Color(blue)).Bold(true).
			Render("󰎄"),
	)
	cover := styleCoverBorder.Width(coverW - 2).Height(coverH).Render(coverInner)
	coverLine := lipgloss.NewStyle().Width(w).Align(lipgloss.Center).Render(cover)

	// Track info lines
	title := stylePlayerTitle.Width(w).Render(truncate(t.DisplayTitle(), w-2))
	artist := stylePlayerArtist.Width(w).Render(truncate(t.DisplayArtist(), w-2))
	album := stylePlayerAlbum.Width(w).Render(truncate(t.Album, w-2))

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

	line := " " + stateChip + "  " + modeChip + "  " + hints
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

	title := styleOverlayTitle.Render("󰋽  Track Info")
	div := styleOverlayMuted.Render(strings.Repeat("─", 42))

	row := func(label, value string) string {
		l := styleOverlayKey.Width(10).Render(label)
		v := styleOverlayValue.Render(truncate(value, 36))
		return "  " + l + "  " + v
	}

	var rows []string
	rows = append(rows, title, div, "")
	if t != nil {
		rows = append(rows,
			row("Title", t.DisplayTitle()),
			row("Artist", t.DisplayArtist()),
			row("Album", t.Album),
			row("Duration", formatDuration(t.Duration)),
			row("Format", t.Format()),
			row("Path", t.Path),
		)
	} else {
		rows = append(rows, styleOverlayMuted.Render("  No track selected"))
	}
	rows = append(rows, "", styleOverlayMuted.Render("  Any key to close"))

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// truncate shortens s to at most n runes, appending "…".
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
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

// displayWidth approximates the terminal display width of a string by
// counting runes (works for ASCII + CJK estimate; good enough for truncation).
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x2E7F {
			w += 2 // CJK and wide chars
		} else {
			w++
		}
	}
	return w
}

// stylePlayerMuted returns a centred, muted style (used for idle hint text).
func stylePlayerMuted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(overlay0)).Align(lipgloss.Center)
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
