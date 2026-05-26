package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Help overlay ──────────────────────────────────────────────────────────────

func (a *App) renderHelpOverlay() string {
	type binding struct{ key, action string }
	bindings := []binding{
		{"j / ↓", T("action_move_down")},
		{"k / ↑", T("action_move_up")},
		{"g / G", T("action_top_bottom")},
		{"Enter", T("action_play")},
		{"Space", T("action_pause_resume")},
		{"n / p", T("action_next_prev")},
		{"< / >", T("action_seek")},
		{"m", T("action_cycle_mode")},
		{"b", T("action_chip")},
		{"r / R", T("action_lofi")},
		{",", T("action_settings")},
		{"/", T("action_search")},
		{"i", T("action_track_info")},
		{"f", T("action_fullscreen")},
		{"+ / -", T("action_volume")},
		{"Tab", T("action_switch_tab")},
		{"?", T("action_this_help")},
		{"q", T("action_quit")},
		{"", ""},
		{"F6 / F9", T("action_media_prev_next")},
		{"F7", T("action_media_play")},
		{"F11 / F12", T("action_media_vol")},
	}

	title := styleOverlayTitle.Render("󰋼  " + T("help_title"))
	// 44 = 2(indent) + 14(key chip width) + 2(gap) + 26(longest action text fit)
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
	rows = append(rows, "", styleOverlayMuted.Render("  "+T("help_close")))

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

	title := styleOverlayTitle.Render("󰋽  " + T("info_title"))
	// +6 = "  "(2 prefix spaces) + "  "(2 key-value gap spaces) + 2(visual margin).
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
			row(T("label_title"), t.DisplayTitle()),
			row(T("label_artist"), t.DisplayArtist()),
			row(T("label_album_artist"), t.AlbumArtist),
			row(T("label_album"), t.Album),
			row(T("label_year"), t.Year),
			row(T("label_track"), t.TrackNumber),
			row(T("label_genre"), t.Genre),
			row(T("label_comment"), t.Comment),
			row(T("label_duration"), formatDuration(t.Duration)),
			row(T("label_format"), t.Format()),
			row(T("label_path"), t.Path),
		} {
			rows = append(rows, lines...)
		}
	} else {
		rows = append(rows, styleOverlayMuted.Render("  "+T("info_no_track")))
	}
	rows = append(rows, "", styleOverlayMuted.Render("  "+T("help_close")))

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Settings overlay ──────────────────────────────────────────────────────────

func (a *App) renderSettingsOverlay() string {
	// lineW is the content width of the settings overlay (excluding styleOverlayBox padding).
	// 52 = 11(label chip Width) + 4(indent+gap) + 37(input field Width) chosen to fit
	// the longest input value without horizontal scrolling.
	const lineW = 52

	title := styleOverlayTitle.Render("  " + T("settings_title"))
	topDiv := styleOverlayMuted.Render(strings.Repeat("─", lineW))

	sectionLabel := func(label string) string {
		// -4 = "── "(3 prefix runes) + " "(1 trailing space before the fill dashes).
		fill := lineW - strWidth(label) - 4
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
	dirLabel := labelStyle(dirActive).Width(11).Render(T("settings_dir_label"))
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
	reloadHint := "  " + reloadKey + styleOverlayMuted.Render(" "+T("settings_reload_hint"))

	// ── 8-bit Conversion section ──────────────────────────────────────────
	optsActive := a.settingsActive == 1
	optsLabel := labelStyle(optsActive).Width(11).Render(T("settings_opts_label"))
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
	optsHint := styleOverlayMuted.Render("  " + T("settings_opts_hint"))
	optsEx := styleOverlayMuted.Render("  " + T("settings_opts_example"))

	// ── Language section ──────────────────────────────────────────────────
	langActive := a.settingsActive == 2
	langLabel := labelStyle(langActive).Width(11).Render(T("settings_lang_label"))
	var langView string
	if activeLang == LangZH {
		langView = styleOverlayValue.Render(T("settings_lang_zh"))
	} else {
		langView = styleOverlayValue.Render(T("settings_lang_en"))
	}
	langLine := "  " + langLabel + "  " + langView

	// ── Footer ────────────────────────────────────────────────────────────
	enterKey := styleOverlayKey.Render(" Enter ")
	escKey := styleOverlayKey.Render(" Esc ")
	tabKey := styleOverlayKey.Render(" Tab ")
	footer := "  " + enterKey + styleOverlayMuted.Render(" "+T("settings_save")+"  ·  ") +
		escKey + styleOverlayMuted.Render(" "+T("settings_cancel")+"  ·  ") +
		tabKey + styleOverlayMuted.Render(" "+T("settings_switch"))

	rows := []string{
		title,
		topDiv,
		"",
		sectionLabel(T("settings_section_lib")),
		"",
		dirLine,
		reloadHint,
		"",
		sectionLabel(T("settings_section_chip")),
		"",
		optsLine,
		"",
		optsHint,
		optsEx,
		"",
		sectionLabel(T("settings_lang_label")),
		"",
		langLine,
		"",
		footer,
	}

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}
