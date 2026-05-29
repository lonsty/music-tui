package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/library"
)

// ── Catppuccin Mocha palette ─────────────────────────────────────────────────

const (
	crust     = "#11111B"
	mantle    = "#181825"
	base      = "#1E1E2E"
	surface0  = "#313244"
	surface1  = "#45475A"
	surface2  = "#585B70"
	overlay0  = "#6C7086"
	overlay1  = "#7F849C"
	overlay2  = "#9399B2"
	subtext0  = "#A6ADC8"
	subtext1  = "#BAC2DE"
	text      = "#CDD6F4"
	lavender  = "#B4BEFE"
	blue      = "#89B4FA"
	sapphire  = "#74C7EC"
	sky       = "#89DCEB"
	teal      = "#94E2D5"
	green     = "#A6E3A1"
	yellow    = "#F9E2AF"
	peach     = "#FAB387"
	maroon    = "#EBA0AC"
	red       = "#F38BA8"
	mauve     = "#CBA6F7"
	pink      = "#F5C2E7"
	flamingo  = "#F2CDCD"
	rosewater = "#F5E0DC"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	// ── Tab bar ──────────────────────────────────────────────────────────────
	// Single-line tabs: active tab is bold + underlined; inactive tabs are dim.
	// No background colour on any tab element — the terminal background shows through.
	styleTabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(mauve))

	styleTabInactive = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0))

	styleTabBar = lipgloss.NewStyle()

	// ── Panels ───────────────────────────────────────────────────────────────
	stylePanelBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(surface1))

	stylePanelTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(lavender))

	// ── Track list ───────────────────────────────────────────────────────────
	styleTrackMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color(overlay0))

	// styleTrackRow* are the four row states in the track list.
	// Pre-declared here to avoid per-row allocation in the render loop.
	styleTrackRowDefault = lipgloss.NewStyle().
				Foreground(lipgloss.Color(subtext0))
	styleTrackRowSelected = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0)).
				Foreground(lipgloss.Color(text)).
				Bold(true)
	// Playing rows: gradient covers the foreground; background + bold only.
	styleTrackRowPlaying = lipgloss.NewStyle().
				Bold(true)
	// Playing + selected: same as playing but with the selection background.
	styleTrackRowPlayingSelected = lipgloss.NewStyle().
					Background(lipgloss.Color(surface0)).
					Bold(true)
	// Playing rows use a solid blue accent for the icon and duration columns.
	styleTrackRowPlayingAccent = lipgloss.NewStyle().
					Foreground(lipgloss.Color(blue)).
					Bold(true)

	// ── Mini / fullscreen player ─────────────────────────────────────────────
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

	// Lyric decoration styles used by renderActiveLyricLine,
	// renderBrowseCursorLine, and renderLyricsPlain.
	// Pre-declared to avoid per-call allocation on the 500ms render path.
	styleLyricActiveText = lipgloss.NewStyle().
				Foreground(lipgloss.Color(mauve)).
				Bold(true)
	styleLyricBrowseText = lipgloss.NewStyle().
				Foreground(lipgloss.Color(text)).
				Bold(true).
				Align(lipgloss.Center)
	styleLyricBrowseBorder = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay1))
	styleLyricBrowseIcon = lipgloss.NewStyle().
				Foreground(lipgloss.Color(subtext0))
	styleLyricPlain = lipgloss.NewStyle().
			Foreground(lipgloss.Color(subtext0)).
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
	// styleOverlayBox uses only a border and padding — no background fill.
	// Setting a background colour inside a terminal that already has its own
	// background causes a solid colour block that clashes on transparent or
	// image-background terminals.  The rounded mauve border provides sufficient
	// visual separation without a background.
	styleOverlayBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(mauve)).
			Padding(1, 2)

	styleOverlayTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(mauve))

	// styleOverlayKey renders keyboard shortcut chips as ❮key❯ using Unicode
	// angle quotation marks (U+276E / U+276F).  No background fill is used to
	// avoid the rectangular colour-block artefacts that appear on some terminals
	// when a lipgloss style with Background + Width is rendered inside a box
	// that already has a background colour.
	styleOverlayKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(lavender)).
			Bold(true)

	styleOverlayValue = lipgloss.NewStyle().
				Foreground(lipgloss.Color(text))

	styleOverlayMuted = lipgloss.NewStyle().
				Foreground(lipgloss.Color(overlay0))

	// styleSettingsSelected highlights the currently selected settings field row.
	styleSettingsSelected = lipgloss.NewStyle().
				Background(lipgloss.Color(surface0))

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

// ── Helpers ───────────────────────────────────────────────────────────────────

// rgbStop holds a pre-parsed colour stop as normalised [0,1] float64 triples.
// Pre-parsing hex strings once at init time avoids repeated string-to-float
// conversions in the hot render path.
type rgbStop struct{ r, g, b float64 }

// parseHexStop converts a "#RRGGBB" hex string into an rgbStop.
// Malformed input returns white {1,1,1} so gradients degrade gracefully.
func parseHexStop(hex string) rgbStop {
	c := strings.TrimPrefix(hex, "#")
	if len(c) != 6 {
		return rgbStop{1, 1, 1}
	}
	parseComp := func(s string) float64 {
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
	return rgbStop{parseComp(c[0:2]), parseComp(c[2:4]), parseComp(c[4:6])}
}

// interpolateStops linearly interpolates between colour stops at position t ∈ [0,1].
func interpolateStops(stops []rgbStop, t float64) rgbStop {
	scaled := t * float64(len(stops)-1)
	lo := int(scaled)
	if lo >= len(stops)-1 {
		return stops[len(stops)-1]
	}
	frac := scaled - float64(lo)
	a, b := stops[lo], stops[lo+1]
	return rgbStop{
		a.r + (b.r-a.r)*frac,
		a.g + (b.g-a.g)*frac,
		a.b + (b.b-a.b)*frac,
	}
}

// clampU8 clamps a normalised float64 to a uint8 byte value.
func clampU8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v * 255)
}

// gradientColors is the default gradient palette used for playing-track text.
// blue → mauve → pink (Catppuccin Mocha).
var gradientColors = []string{blue, mauve, pink}

// gradientStops is the pre-parsed version of gradientColors.
// Initialised once at program start; avoids repeated hex parsing on the hot path.
var gradientStops = []rgbStop{
	parseHexStop(blue),
	parseHexStop(mauve),
	parseHexStop(pink),
}

// gradientText applies a linear colour gradient across the display columns of s.
// colors must contain at least two "#RRGGBB" hex strings.
// Each rune is coloured by interpolating between the adjacent colour stops at
// its position in the total display-column count.
// bold controls whether the output is rendered in bold ANSI.
//
// Performance: colours are pre-parsed to []rgbStop before the rune loop.
// The output is built using direct ANSI 24-bit colour escape codes rather than
// creating a lipgloss.Style per rune, which eliminates the per-character heap
// allocation that was the primary hot-path cost.
func gradientText(s string, bold bool, colors ...string) string {
	if len(s) == 0 || len(colors) < 2 {
		return s
	}
	totalW := strWidth(s)
	if totalW == 0 {
		return s
	}

	// Use pre-parsed stops when the caller passes the default palette.
	var stops []rgbStop
	if len(colors) == len(gradientColors) &&
		colors[0] == gradientColors[0] &&
		colors[1] == gradientColors[1] &&
		colors[2] == gradientColors[2] {
		stops = gradientStops
	} else {
		stops = make([]rgbStop, len(colors))
		for i, c := range colors {
			stops[i] = parseHexStop(c)
		}
	}

	var out strings.Builder
	// Pre-grow: each rune needs at most ~20 bytes of ANSI overhead + 4 bytes UTF-8.
	out.Grow(len(s) * 24)

	// ANSI SGR prefix constants.
	const (
		boldOn      = "\x1b[1m"
		reset       = "\x1b[0m"
		fgTrueColor = "\x1b[38;2;"
	)

	col := 0
	for _, r := range s {
		rw := strWidth(string(r))
		var t float64
		if totalW > 1 {
			t = float64(col) / float64(totalW-1)
		}
		c := interpolateStops(stops, t)
		r8, g8, b8 := clampU8(c.r), clampU8(c.g), clampU8(c.b)

		// Write: ESC[38;2;R;G;Bm [ESC[1m] char ESC[0m
		out.WriteString(fgTrueColor)
		// Inline integer formatting to avoid fmt.Sprintf allocation.
		writeUint8(&out, r8)
		out.WriteByte(';')
		writeUint8(&out, g8)
		out.WriteByte(';')
		writeUint8(&out, b8)
		out.WriteByte('m')
		if bold {
			out.WriteString(boldOn)
		}
		out.WriteRune(r)
		out.WriteString(reset)
		col += rw
	}
	return out.String()
}

// writeUint8 writes a uint8 value in decimal to b without any allocation.
func writeUint8(b *strings.Builder, v uint8) {
	if v >= 100 {
		b.WriteByte('0' + v/100)
		b.WriteByte('0' + (v/10)%10)
		b.WriteByte('0' + v%10)
	} else if v >= 10 {
		b.WriteByte('0' + v/10)
		b.WriteByte('0' + v%10)
	} else {
		b.WriteByte('0' + v)
	}
}

// centeredGradientText applies the default gradient to text, strips surrounding
// whitespace, then re-centres the result within avail display columns.
func centeredGradientText(text string, avail int) string {
	core := strings.TrimSpace(text)
	grad := gradientText(core, true, gradientColors...)
	pad := avail - strWidth(core)
	return strings.Repeat(" ", pad/2) + grad + strings.Repeat(" ", pad-pad/2)
}

// absInt returns the absolute value of n.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

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
//
//	Album · Artist · Title
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
		// e.g. 1378 → "1.4k".
		return fmt.Sprintf("%.1fk", float64(hz)/1000.0)
	}
	return fmt.Sprintf("%d", hz)
}
