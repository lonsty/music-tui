package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Marquee is a scrolling-text widget. When the text fits within the given
// width it is returned as-is; when it overflows the text scrolls left at one
// display-column per Tick call, wrapping with a separator gap.
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
	marqueePauseTicks = 8
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

// Tick advances the scroll position by one display column.
// Call this on every tickMsg.
func (m *Marquee) Tick(width int) {
	if m.loopW == 0 || strWidth(m.text) <= width {
		return // no scrolling needed
	}
	if m.pauseLeft > 0 {
		m.pauseLeft--
		return
	}
	m.offset++
	if m.offset >= m.loopW {
		m.offset = 0
		m.pauseLeft = marqueePauseTicks
	}
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
// column `start`.  It handles multi-byte and double-wide characters correctly.
func colSlice(s string, start, width int) string {
	// Skip `start` display columns.
	col := 0
	startByte := 0
	for i, r := range s {
		if col >= start {
			startByte = i
			break
		}
		col += ansi.StringWidth(string(r))
	}

	// Collect `width` display columns.
	col = 0
	var out []byte
	for _, r := range s[startByte:] {
		rw := ansi.StringWidth(string(r))
		if col+rw > width {
			break
		}
		out = append(out, []byte(string(r))...)
		col += rw
	}
	result := string(out)
	// Pad if we collected fewer columns than requested (e.g. double-wide char
	// would overflow).
	if col < width {
		result += padRight("", width-col)
	}
	return result
}
