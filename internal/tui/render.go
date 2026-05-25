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
	styleTrackMeta = lipgloss.NewStyle().
			Foreground(lipgloss.Color(overlay0))

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

// centeredGradientText applies the default gradient to text, strips surrounding
// whitespace, then re-centres the result within avail display columns.
func centeredGradientText(text string, avail int) string {
	core := strings.TrimSpace(text)
	grad := gradientText(core, true, gradientColors...)
	pad := avail - strWidth(core)
	return strings.Repeat(" ", pad/2) + grad + strings.Repeat(" ", pad-pad/2)
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
		// e.g. 1378 → "1.4k".
		return fmt.Sprintf("%.1fk", float64(hz)/1000.0)
	}
	return fmt.Sprintf("%d", hz)
}
