package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Marquee is a scrolling-text widget. When the text fits within the given
// width it is returned as-is; when it overflows the text scrolls left at
// marqueeScrollClusters complete grapheme clusters per Tick call, wrapping
// with a separator gap.  Advancing by whole grapheme clusters keeps the
// perceived scroll speed uniform across Latin, CJK, and emoji sequences:
// a family emoji (👨‍👩‍👧, 2 cols, many runes) scrolls at the same rate as
// one Latin character (1 col, 1 rune).
//
// Usage:
//
//	m := NewMarquee("very long title here…", "  •  ")
//	m.SetText("new text")  // replace content, reset scroll
//	view := m.Render(width) // call each frame
//	m.Tick()               // advance scroll offset by one column
type Marquee struct {
	text string // raw display text
	sep  string // separator between repetitions, e.g. "  •  "

	// loop is text+sep pre-computed for scrolling.
	// loopBuf is loop+loop, also pre-computed so Render/RenderCentered can
	// slice directly without allocating a temporary concatenation each call.
	loop      string
	loopBuf   string // loop + loop — pre-computed in SetText
	loopW     int    // display width of loop
	offset    int    // current scroll column (0 = leftmost)
	pauseLeft int    // ticks to pause before starting / after reset
}

const (
	// marqueePauseTicks is how many ticks to show text stationary before
	// scrolling begins (or after the text wraps around).
	// At tickInterval=500ms this gives a 1 second pause before scrolling starts.
	marqueePauseTicks = 2

	// marqueeScrollCols is the target number of display columns to advance per
	// tick.  The scroll always stops on a grapheme-cluster boundary, so the
	// actual advance may be slightly more than this value when the next cluster
	// is wider than 1 column (e.g. a CJK character occupies 2 columns).
	// Using columns (not cluster count) keeps Latin and CJK text scrolling at
	// the same perceived speed: both advance ~2 visible columns per tick.
	marqueeScrollCols = 2
)

// NewMarquee creates a Marquee with the given text and separator string.
func NewMarquee(text, sep string) *Marquee {
	m := &Marquee{sep: sep}
	m.SetText(text)
	return m
}

// SetText replaces the content and resets the scroll position.
// It pre-computes loop and loopBuf so that Render and RenderCentered can
// perform a direct slice without allocating a temporary string each call.
func (m *Marquee) SetText(text string) {
	if text == m.text {
		return
	}
	m.text = text
	m.loop = text + m.sep
	m.loopW = strWidth(m.loop)
	m.loopBuf = m.loop + m.loop // pre-compute double buffer
	m.offset = 0
	m.pauseLeft = marqueePauseTicks
}

// Tick advances the scroll position by approximately marqueeScrollCols display
// columns, stopping on the nearest grapheme-cluster boundary.  This keeps
// Latin (1 col/cluster) and CJK (2 cols/cluster) text scrolling at the same
// perceived speed.  Call this on every tickMsg.
func (m *Marquee) Tick(width int) {
	if m.loopW == 0 || strWidth(m.text) <= width {
		return // no scrolling needed
	}
	if m.pauseLeft > 0 {
		m.pauseLeft--
		return
	}
	m.offset = nextColBoundary(m.loopBuf, m.offset, marqueeScrollCols)
	if m.offset >= m.loopW {
		m.offset = 0
		m.pauseLeft = marqueePauseTicks
	}
}

// nextColBoundary returns the new display-column offset after advancing at
// least targetCols display columns from startCol in s, stopping on the next
// grapheme-cluster boundary.
//
// "At least" semantics: if the cluster that crosses the target boundary is
// wider than 1 column (e.g. a CJK character), the returned offset lands after
// that cluster, so Latin and CJK text both advance ~targetCols visible columns
// per call — giving uniform perceived scroll speed regardless of script.
//
// Phase 1: skip ahead to startCol by consuming clusters.
// Phase 2: consume clusters until col ≥ startCol + targetCols.
// If s ends before the target is reached the returned value will be ≥ loopW,
// which the caller uses as the wrap-around signal.
func nextColBoundary(s string, startCol, targetCols int) int {
	// Phase 1: skip to startCol.
	col := 0
	rest := s
	for col < startCol && len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		col += w
		rest = rest[len(cluster):]
	}

	// Phase 2: advance until we have moved at least targetCols columns.
	target := col + targetCols
	for col < target && len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		col += w
		rest = rest[len(cluster):]
	}
	return col
}

// Render returns a string of exactly `width` display columns.
// If the text fits, it is left-aligned and padded with spaces.
// If it overflows, a scrolling window of `width` columns is returned.
func (m *Marquee) Render(width int) string {
	if width <= 0 {
		return ""
	}
	tw := strWidth(m.text)
	if tw <= width {
		// Text fits — no scrolling, just pad.
		return padRight(m.text, width)
	}

	// Build a double-loop buffer so we can always slice `width` columns
	// starting at m.offset without worrying about running off the end.
	// loopBuf was pre-computed in SetText to avoid a per-call allocation.
	return colSlice(m.loopBuf, m.offset, width)
}

// RenderCentered returns a string of exactly `width` display columns.
// If the text fits, it is centered with equal left/right padding.
// If it overflows, a scrolling window of `width` columns is returned
// (same as Render — scrolling text is always left-edge aligned).
func (m *Marquee) RenderCentered(width int) string {
	if width <= 0 {
		return ""
	}
	tw := strWidth(m.text)
	if tw <= width {
		pad := width - tw
		left := pad / 2
		right := pad - left
		return strings.Repeat(" ", left) + m.text + strings.Repeat(" ", right)
	}

	buf := m.loopBuf // pre-computed in SetText; no allocation here
	return colSlice(buf, m.offset, width)
}

// colSlice returns `width` display columns from `s` starting at display
// column `start`.
//
// Iteration is done grapheme-cluster by grapheme-cluster (via
// ansi.FirstGraphemeCluster) rather than rune by rune.  This guarantees
// correct handling of combining diacritics, ZWJ emoji sequences, and flag
// pairs whose display width is determined by the cluster as a whole, not
// by the sum of individual rune widths.
func colSlice(s string, start, width int) string {
	// Phase 1: skip `start` display columns.
	col := 0
	rest := s
	for col < start && len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		col += w
		rest = rest[len(cluster):]
	}

	// Phase 2: collect `width` display columns.
	col = 0
	var sb strings.Builder
	for col < width && len(rest) > 0 {
		cluster, w := ansi.FirstGraphemeCluster(rest, ansi.GraphemeWidth)
		if col+w > width {
			break
		}
		sb.WriteString(cluster)
		col += w
		rest = rest[len(cluster):]
	}

	result := sb.String()
	// Pad if we collected fewer columns than requested (e.g. a double-wide
	// cluster would overflow the remaining slot).
	if col < width {
		result += padRight("", width-col)
	}
	return result
}
