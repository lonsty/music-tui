package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		{"", ""},
		{"F6 / F9", "Prev / Next  (media key mapping)"},
		{"F7", "Play / Pause  (media key mapping)"},
		{"F11 / F12", "Vol down / up  (media key mapping)"},
	}

	title := styleOverlayTitle.Render("󰋼  Keyboard shortcuts")
	div := styleOverlayMuted.Render(strings.Repeat("─", 44))

	var rows []string
	rows = append(rows, title, div, "")
	for _, b := range bindings {
		if b.key == "" && b.action == "" {
			rows = append(rows, "")
			continue
		}
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
			// Assign a copy to avoid mutating the package-level style.
			s := styleOverlayKey
			return s.Foreground(lipgloss.Color(mauve))
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
