package tui

import (
	"strings"
	"unicode/utf8"
)

// Marquee is a scrolling-text widget. When the text fits within the given
// width it is returned as-is; when it overflows the text scrolls left at
// marqueeScrollChars complete characters per Tick call, wrapping with a
// separator gap.  Advancing by whole characters (rather than a fixed column
// count) keeps the perceived scroll speed uniform regardless of whether the
// text is Latin (1 col/char) or CJK (2 cols/char).
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

	// marqueeScrollChars is the number of complete characters to advance per
	// tick.  Using characters (not columns) ensures CJK and Latin text scroll
	// at the same perceived speed: 1 char/tick × 2 ticks/s = 2 chars/s.
	marqueeScrollChars = 2
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

// Tick advances the scroll position by marqueeScrollChars complete characters.
// Advancing by whole characters ensures uniform perceived speed across scripts:
// Latin text (1 col/char) and CJK text (2 cols/char) both move at the same
// character-per-second rate rather than the same column-per-second rate.
// Call this on every tickMsg.
func (m *Marquee) Tick(width int) {
	if m.loopW == 0 || strWidth(m.text) <= width {
		return // no scrolling needed
	}
	if m.pauseLeft > 0 {
		m.pauseLeft--
		return
	}
	m.offset = nextCharBoundary(m.loopBuf, m.offset, marqueeScrollChars)
	if m.offset >= m.loopW {
		m.offset = 0
		m.pauseLeft = marqueePauseTicks
	}
}

// nextCharBoundary returns the new display-column offset after advancing n
// complete characters forward from startCol in s.
// It first skips to startCol, then counts n more complete characters and
// returns the column position at the start of the (n+1)-th character.
// If s ends before n characters are consumed the returned value will be ≥
// loopW which the caller uses as the wrap-around signal.
func nextCharBoundary(s string, startCol, n int) int {
	// Phase 1: skip to startCol.
	col := 0
	idx := 0
	for i, r := range s {
		if col >= startCol {
			idx = i
			break
		}
		col += runeWidth(r)
		idx = i + utf8.RuneLen(r)
	}

	// Phase 2: advance n complete characters.
	advanced := 0
	for _, r := range s[idx:] {
		if advanced >= n {
			break
		}
		col += runeWidth(r)
		advanced++
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
		col += runeWidth(r)
	}

	// Collect `width` display columns using a strings.Builder to avoid the
	// per-rune []byte conversions (string(r) → []byte(string(r)) → append).
	col = 0
	var sb strings.Builder
	for _, r := range s[startByte:] {
		rw := runeWidth(r)
		if col+rw > width {
			break
		}
		sb.WriteRune(r)
		col += rw
	}
	result := sb.String()
	// Pad if we collected fewer columns than requested (e.g. double-wide char
	// would overflow).
	if col < width {
		result += padRight("", width-col)
	}
	return result
}
