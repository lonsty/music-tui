package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lonsty/music-tui/internal/store"
)

// overlayInnerW returns the usable content width (columns) for an overlay box
// given the current terminal width.  It subtracts the box border (2 cols each
// side = 2 total for RoundedBorder) and the horizontal padding defined by
// styleOverlayBox.Padding(1, 2) (2 cols each side = 4 total), plus a small
// outer margin so the box never touches the terminal edges.
//
//	available = a.W - border(2) - padding(4) - outerMargin(4)
//
// The result is clamped to [minW, idealW].
func (a *App) overlayInnerW(idealW, minW int) int {
	const boxBorderW = 2   // left + right border chars
	const boxPadW = 4      // Padding(1,2) → 2 left + 2 right
	const outerMarginW = 4 // keep 2 cols free on each side of the box
	avail := a.W - boxBorderW - boxPadW - outerMarginW
	if avail < minW {
		avail = minW
	}
	if avail > idealW {
		avail = idealW
	}
	return avail
}

// All three overlays (help, info, settings) share the same ideal and minimum
// content widths so they appear the same size on wide terminals.
//
// idealOverlayW is derived from the widest help action string across both
// languages (Chinese "设置（音乐目录…）" = 58 cols) plus the key-chip column:
//
//	2(indent) + 14(keyColW) + 2(gap) + 58(maxAction) = 76
const (
	idealOverlayW = 72
	minOverlayW   = 28
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
		{"l", T("action_lyrics_mode")},
		{`\`, T("action_collapse_panel")},
		{"+ / -", T("action_volume")},
		{"Tab", T("action_switch_tab")},
		{"?", T("action_this_help")},
		{"q", T("action_quit")},
		{"", ""},
		{"F6 / F9", T("action_media_prev_next")},
		{"F7", T("action_media_play")},
		{"F11 / F12", T("action_media_vol")},
	}

	// keyColW: fixed width reserved for the key chip column.
	// actionW: remaining width for the action description.
	const keyColW = 14
	innerW := a.overlayInnerW(idealOverlayW, minOverlayW)
	actionW := innerW - 2 - keyColW - 2 // innerW - indent - keyColW - gap
	if actionW < 8 {
		actionW = 8
	}

	div := styleOverlayMuted.Render(strings.Repeat("─", innerW))
	titleRows := []string{
		styleOverlayTitle.Render(iconWithSpace(iconHelp()) + T("help_title")),
		div,
	}

	var bodyRows []string
	for _, b := range bindings {
		if b.key == "" && b.action == "" {
			bodyRows = append(bodyRows, "")
			continue
		}
		chip := styleOverlayKey.Render("❮" + b.key + "❯")
		chipW := 2 + strWidth(b.key)
		if chipW < keyColW {
			chip += strings.Repeat(" ", keyColW-chipW)
		}
		// Truncate action text if terminal is too narrow to fit it.
		action := b.action
		if strWidth(action) > actionW {
			action = truncate(action, actionW)
		}
		v := styleOverlayValue.Render(action)
		bodyRows = append(bodyRows, "  "+chip+"  "+v)
	}

	hintRows := []string{"", styleOverlayMuted.Render("  " + T("overlay_hint_close"))}

	return a.renderScrollableOverlay(titleRows, bodyRows, hintRows, innerW)
}

// ── Info overlay ──────────────────────────────────────────────────────────────

func (a *App) renderInfoOverlay() string {
	t := a.currentTrack
	if t == nil {
		t = a.cursorTrack()
	}

	// labelW: fixed width for the field-name column.
	// valueW: remaining width for the field value (adaptive).
	const labelW = 14
	innerW := a.overlayInnerW(idealOverlayW, minOverlayW)
	valueW := innerW - 2 - labelW - 2
	if valueW < 10 {
		valueW = 10
	}
	// indent aligns continuation lines with the value column.
	indent := strings.Repeat(" ", 2+labelW+2)

	div := styleOverlayMuted.Render(strings.Repeat("─", innerW))
	titleRows := []string{
		styleOverlayTitle.Render(iconWithSpace(iconInfo()) + T("info_title")),
		div,
	}

	row := func(label, value string) []string {
		if value == "" {
			return nil
		}
		labelText := styleOverlayKey.Render(label)
		lw := strWidth(label)
		if lw < labelW {
			labelText += strings.Repeat(" ", labelW-lw)
		}
		segments := wrapText(value, valueW)
		var result []string
		for i, seg := range segments {
			v := styleOverlayValue.Render(seg)
			if i == 0 {
				result = append(result, "  "+labelText+"  "+v)
			} else {
				result = append(result, indent+v)
			}
		}
		return result
	}

	var bodyRows []string
	bodyRows = append(bodyRows, "")
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
			bodyRows = append(bodyRows, lines...)
		}
	} else {
		bodyRows = append(bodyRows, styleOverlayMuted.Render("  "+T("info_no_track")))
	}

	hintRows := []string{"", styleOverlayMuted.Render("  " + T("overlay_hint_close"))}

	return a.renderScrollableOverlay(titleRows, bodyRows, hintRows, innerW)
}

// ── Settings overlay ──────────────────────────────────────────────────────────

func (a *App) renderSettingsOverlay() string {
	// lineW: total content width (adaptive), shared with the other overlays.
	// labelColW: fixed width for the label chip column (9 visible + 2 padding = 11).
	const labelColW = 11
	lineW := a.overlayInnerW(idealOverlayW, minOverlayW)
	inputW := lineW - labelColW - 4 // 2(indent) + 2(gap)
	if inputW < 8 {
		inputW = 8
	}

	title := styleOverlayTitle.Render("  " + T("settings_title"))
	topDiv := styleOverlayMuted.Render(strings.Repeat("─", lineW))

	sectionLabel := func(label string) string {
		fill := lineW - strWidth(label) - 4
		if fill < 0 {
			fill = 0
		}
		return styleOverlayMuted.Render("── " + label + " " + strings.Repeat("─", fill))
	}

	// selBg returns the surface0 colour when active, empty (transparent) otherwise.
	selBg := func(active bool) lipgloss.Color {
		if active {
			return lipgloss.Color(surface0)
		}
		return lipgloss.Color("")
	}

	spaceStyle := func(active bool) lipgloss.Style {
		return lipgloss.NewStyle().Background(selBg(active))
	}

	labelStyle := func(active bool) lipgloss.Style {
		s := styleOverlayKey.Background(selBg(active))
		if active {
			return s.Foreground(lipgloss.Color(mauve))
		}
		return s
	}
	valueStyle := func(active bool) lipgloss.Style {
		return styleOverlayValue.Background(selBg(active))
	}

	buildFieldLine := func(label, value string, active bool) string {
		sp := spaceStyle(active)
		line := sp.Render("  ") + label + sp.Render("  ") + value
		if active {
			return styleSettingsSelected.Width(lineW).Render(line)
		}
		return line
	}

	// ── Music Library section ─────────────────────────────────────────────
	dirActive := a.settingsActive == settingsFieldMusicDir
	dirPad := labelColW - 2 - strWidth(T("settings_dir_label"))
	if dirPad < 0 {
		dirPad = 0
	}
	dirLabel := labelStyle(dirActive).Render(T("settings_dir_label") + strings.Repeat(" ", dirPad))
	var dirView string
	if dirActive && a.settingsEditing {
		a.musicDirInput.Width = inputW
		dirView = a.musicDirInput.View()
	} else {
		val := a.musicDirInput.Value()
		if strWidth(val) > inputW {
			val = "…" + val[len(val)-inputW+1:]
		}
		dirView = valueStyle(dirActive).Render(val)
	}
	dirLine := buildFieldLine(dirLabel, dirView, dirActive)
	reloadKey := styleOverlayKey.Render("❮Ctrl+R❯")
	reloadHint := "  " + reloadKey + styleOverlayMuted.Render(" "+T("settings_reload_hint"))

	// ── 8-bit Conversion section ──────────────────────────────────────────
	optsActive := a.settingsActive == settingsFieldChipOpts
	optsPad := labelColW - 2 - strWidth(T("settings_opts_label"))
	if optsPad < 0 {
		optsPad = 0
	}
	optsLabel := labelStyle(optsActive).Render(T("settings_opts_label") + strings.Repeat(" ", optsPad))
	var optsView string
	if optsActive && a.settingsEditing {
		a.settingsInput.Width = inputW
		optsView = a.settingsInput.View()
	} else {
		val := a.settingsInput.Value()
		if strWidth(val) > inputW {
			val = truncate(val, inputW)
		}
		optsView = valueStyle(optsActive).Render(val)
	}
	optsLine := buildFieldLine(optsLabel, optsView, optsActive)
	optsHint := styleOverlayMuted.Render("  " + T("settings_opts_hint"))
	optsEx := styleOverlayMuted.Render("  " + T("settings_opts_example"))

	// ── Language section ──────────────────────────────────────────────────
	langActive := a.settingsActive == settingsFieldLanguage
	langPad := labelColW - 2 - strWidth(T("settings_lang_label"))
	if langPad < 0 {
		langPad = 0
	}
	langLabel := labelStyle(langActive).Render(T("settings_lang_label") + strings.Repeat(" ", langPad))
	var langView string
	if activeLang == LangZH {
		langView = valueStyle(langActive).Render(T("settings_lang_zh"))
	} else {
		langView = valueStyle(langActive).Render(T("settings_lang_en"))
	}
	langLine := buildFieldLine(langLabel, langView, langActive)

	// ── Format filter section ─────────────────────────────────────────────
	fmtActive := a.settingsActive == settingsFieldFormat
	fmtPad := labelColW - 2 - strWidth(T("settings_fmt_label"))
	if fmtPad < 0 {
		fmtPad = 0
	}
	fmtLabel := labelStyle(fmtActive).Render(T("settings_fmt_label") + strings.Repeat(" ", fmtPad))
	fmtLine := buildFieldLine(fmtLabel, valueStyle(fmtActive).Render(formatPrefLabel(a.formatPref)), fmtActive)

	// ── Icon set section ───────────────────────────────────────────────────
	iconSetActive := a.settingsActive == settingsFieldIconSet
	iconSetPad := labelColW - 2 - strWidth(T("settings_icon_set_label"))
	if iconSetPad < 0 {
		iconSetPad = 0
	}
	iconSetLabel := labelStyle(iconSetActive).Render(T("settings_icon_set_label") + strings.Repeat(" ", iconSetPad))
	iconSetLine := buildFieldLine(iconSetLabel, valueStyle(iconSetActive).Render(iconSetDisplayLabel(ActiveIconSet())), iconSetActive)

	// ── Footer ────────────────────────────────────────────────────────────
	// Build footer that fits within lineW by progressively dropping hints.
	enterKey := styleOverlayKey.Render("❮Enter❯")
	escKey := styleOverlayKey.Render("❮Esc❯")
	upKey := styleOverlayKey.Render("❮↑/↓❯")
	sep := styleOverlayMuted.Render("  ·  ")
	var footer string
	if a.settingsEditing {
		full := "  " + enterKey + styleOverlayMuted.Render(" "+T("settings_save")) + sep +
			escKey + styleOverlayMuted.Render(" "+T("settings_cancel"))
		short := "  " + escKey + styleOverlayMuted.Render(" "+T("settings_cancel"))
		if strWidth(full) <= lineW {
			footer = full
		} else {
			footer = short
		}
	} else {
		full := "  " + enterKey + styleOverlayMuted.Render(" "+T("settings_edit")) + sep +
			upKey + styleOverlayMuted.Render(" "+T("settings_navigate")) + sep +
			escKey + styleOverlayMuted.Render(" "+T("settings_cancel"))
		noNav := "  " + enterKey + styleOverlayMuted.Render(" "+T("settings_edit")) + sep +
			escKey + styleOverlayMuted.Render(" "+T("settings_cancel"))
		short := "  " + escKey + styleOverlayMuted.Render(" "+T("settings_cancel"))
		switch {
		case strWidth(full) <= lineW:
			footer = full
		case strWidth(noNav) <= lineW:
			footer = noNav
		default:
			footer = short
		}
	}

	titleRows := []string{title, topDiv}

	bodyRows := []string{
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
		sectionLabel(T("settings_fmt_label")),
		"",
		fmtLine,
		"",
		sectionLabel(T("settings_icon_set_label")),
		"",
		iconSetLine,
	}

	hintRows := []string{"", footer}

	return a.renderScrollableOverlay(titleRows, bodyRows, hintRows, lineW)
}

// renderScrollableOverlay renders a scrollable modal overlay centred on screen.
//
// innerW is the content width (columns) computed by overlayInnerW.  Every row
// is clipped to innerW before being passed to lipgloss so the box never grows
// wider than intended regardless of what the callers put in the rows slices.
func (a *App) renderScrollableOverlay(titleRows, bodyRows, hintRows []string, innerW int) string {
	const boxBorderH = 2  // top + bottom rounded-border lines
	const boxPadV = 2     // styleOverlayBox.Padding(1, 2) → 1 top + 1 bottom
	const outerMargin = 2 // keep 1 blank line above and below the box

	// clipRow clips a pre-rendered (possibly ANSI-coloured) string to innerW
	// visible columns.  This prevents any single row from stretching the box
	// beyond the intended width.
	clipStyle := lipgloss.NewStyle().MaxWidth(innerW)
	clipRow := func(s string) string { return clipStyle.Render(s) }

	clip := func(rows []string) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = clipRow(r)
		}
		return out
	}
	titleRows = clip(titleRows)
	bodyRows = clip(bodyRows)
	hintRows = clip(hintRows)

	// Maximum usable rows inside the box (excluding the box's own border+padding).
	maxInner := a.H - boxBorderH - boxPadV - outerMargin
	if maxInner < len(titleRows)+len(hintRows)+1 {
		maxInner = len(titleRows) + len(hintRows) + 1
	}

	// Rows available for the body after reserving title + divider row + hint.
	bodyAvail := maxInner - len(titleRows) - len(hintRows)
	if bodyAvail < 1 {
		bodyAvail = 1
	}

	// Clamp scroll offset.
	total := len(bodyRows)
	maxOffset := total - bodyAvail
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := a.ovlScrollRow
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	a.ovlScrollRow = offset // write back clamped value

	// Visible body slice.
	end := offset + bodyAvail
	if end > total {
		end = total
	}
	visibleBody := bodyRows[offset:end]

	// Build scroll indicator glyphs to embed in the hint row (no extra line).
	var scrollIndicator string
	if total > bodyAvail {
		up, down := " ", " "
		if offset > 0 {
			up = "↑"
		}
		if offset < maxOffset {
			down = "↓"
		}
		scrollIndicator = styleOverlayMuted.Render(" " + up + down)
	}

	// Embed scroll indicator at the right end of the last hint row so it
	// doesn't consume an extra line.
	hintRowsRendered := make([]string, len(hintRows))
	copy(hintRowsRendered, hintRows)
	if scrollIndicator != "" && len(hintRowsRendered) > 0 {
		last := hintRowsRendered[len(hintRowsRendered)-1]
		hintRowsRendered[len(hintRowsRendered)-1] = last + scrollIndicator
	}

	// Assemble: title + visible body + hint (with embedded scroll indicator).
	var all []string
	all = append(all, titleRows...)
	all = append(all, visibleBody...)
	all = append(all, hintRowsRendered...)

	box := styleOverlayBox.Render(lipgloss.JoinVertical(lipgloss.Left, all...))

	// Centre the box vertically when it fits; pin to top (with margin) when
	// the terminal is too short.
	return strings.Repeat("\n", topPad) +
		lipgloss.Place(a.W, a.H-topPad, lipgloss.Center, lipgloss.Center, box)
}

// ── Add-to-playlist overlay ───────────────────────────────────────────────────

// renderAddToPlaylistOverlay renders the playlist-picker overlay shown when
// the user presses 'a' in the Library tab.
func (a *App) renderAddToPlaylistOverlay() string {
	innerW := a.overlayInnerW(idealOverlayW, minOverlayW)
	div := styleOverlayMuted.Render(strings.Repeat("─", innerW))

	titleRows := []string{
		styleOverlayTitle.Render(iconWithSpace(iconPlaylist()) + T("playlist_add_title")),
		div,
	}

	var bodyRows []string
	if len(a.ovlPlaylists) == 0 {
		bodyRows = append(bodyRows, "  "+styleOverlayMuted.Render(T("playlist_empty")))
	} else {
		for i, pl := range a.ovlPlaylists {
			isSelected := i == a.ovlPlCursor
			name := pl.Name
			if pl.ID == store.FavoritesPlaylistID {
				name = T("playlist_favorites_name")
			}
			var row string
			if isSelected {
				row = styleTrackRowSelected.Render("  ▶ " + name)
			} else {
				row = styleTrackRowDefault.Render("    " + name)
			}
			bodyRows = append(bodyRows, row)
		}
	}

	enterKey := styleOverlayKey.Render("❮Enter❯")
	escKey := styleOverlayKey.Render("❮Esc❯")
	hintRows := []string{
		div,
		"  " + enterKey + styleOverlayMuted.Render(" add  ") +
			escKey + styleOverlayMuted.Render(" cancel"),
	}

	return a.renderScrollableOverlay(titleRows, bodyRows, hintRows, innerW)
}
