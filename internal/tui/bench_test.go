package tui

import (
	"strings"
	"testing"
)

// ── Test data ─────────────────────────────────────────────────────────────────

const (
	// benchLatin is a typical English song title that overflows a 40-col panel.
	benchLatin = "The Quick Brown Fox Jumps Over The Lazy Dog And Keeps On Running"

	// benchCJK is a typical Chinese song title of similar visual length.
	benchCJK = "一生所爱 · 卢冠廷 · 大话西游之大圣娶亲原声带"

	// benchMixed is a mixed Chinese/English title common in Asian music libraries.
	benchMixed = "Beautiful Day (美好的一天) feat. 陈奕迅 - Extended Version"

	// benchEmoji contains ZWJ sequences and flag emoji to exercise grapheme-cluster
	// handling.
	benchEmoji = "Best Of 2024 🎵 Hits 🇨🇳 Top Songs 👨‍👩‍👧 Family Edition"
)

// panelWidth simulates a typical mini-player panel width.
const panelWidth = 40

// ── colSlice ─────────────────────────────────────────────────────────────────

// BenchmarkColSlice_Latin measures the cost of slicing a Latin scrolling string.
func BenchmarkColSlice_Latin(b *testing.B) {
	s := benchLatin + "  •  " + benchLatin + "  •  " // loopBuf
	b.ResetTimer()
	for range b.N {
		colSlice(s, 10, panelWidth)
	}
}

// BenchmarkColSlice_CJK measures the cost with CJK text (2-col grapheme clusters).
func BenchmarkColSlice_CJK(b *testing.B) {
	s := benchCJK + "  •  " + benchCJK + "  •  "
	b.ResetTimer()
	for range b.N {
		colSlice(s, 10, panelWidth)
	}
}

// BenchmarkColSlice_Mixed measures mixed Latin+CJK text.
func BenchmarkColSlice_Mixed(b *testing.B) {
	s := benchMixed + "  •  " + benchMixed + "  •  "
	b.ResetTimer()
	for range b.N {
		colSlice(s, 10, panelWidth)
	}
}

// BenchmarkColSlice_Emoji measures text with ZWJ emoji sequences.
func BenchmarkColSlice_Emoji(b *testing.B) {
	s := benchEmoji + "  •  " + benchEmoji + "  •  "
	b.ResetTimer()
	for range b.N {
		colSlice(s, 10, panelWidth)
	}
}

// ── nextColBoundary ───────────────────────────────────────────────────────────

// BenchmarkNextColBoundary_Latin measures the tick-advance cost for Latin text.
func BenchmarkNextColBoundary_Latin(b *testing.B) {
	loop := benchLatin + "  •  "
	s := loop + loop
	b.ResetTimer()
	for range b.N {
		nextColBoundary(s, 0, 2)
	}
}

// BenchmarkNextColBoundary_CJK measures the tick-advance cost for CJK text.
func BenchmarkNextColBoundary_CJK(b *testing.B) {
	loop := benchCJK + "  •  "
	s := loop + loop
	b.ResetTimer()
	for range b.N {
		nextColBoundary(s, 0, 2)
	}
}

// ── wrapText ──────────────────────────────────────────────────────────────────

// BenchmarkWrapText_Short measures a short string that fits without wrapping.
func BenchmarkWrapText_Short(b *testing.B) {
	s := "Short title"
	b.ResetTimer()
	for range b.N {
		wrapText(s, panelWidth)
	}
}

// BenchmarkWrapText_Latin measures wrapping a long Latin file path.
func BenchmarkWrapText_Latin(b *testing.B) {
	s := "/Users/music/Albums/The Quick Brown Fox/01 - Jumps Over The Lazy Dog.mp3"
	b.ResetTimer()
	for range b.N {
		wrapText(s, panelWidth)
	}
}

// BenchmarkWrapText_CJK measures wrapping a long CJK path.
func BenchmarkWrapText_CJK(b *testing.B) {
	s := "/音乐/专辑/一生所爱大话西游之大圣娶亲原声带/01 - 一生所爱.flac"
	b.ResetTimer()
	for range b.N {
		wrapText(s, panelWidth)
	}
}

// BenchmarkWrapText_Long measures a pathological long string with no break points.
func BenchmarkWrapText_Long(b *testing.B) {
	s := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 10) // 260 chars, no spaces
	b.ResetTimer()
	for range b.N {
		wrapText(s, panelWidth)
	}
}

// ── lyricStyleForDistance ─────────────────────────────────────────────────────

// BenchmarkLyricStyleForDistance_Active measures the hot active-line path.
func BenchmarkLyricStyleForDistance_Active(b *testing.B) {
	for range b.N {
		_ = lyricStyleForDistance(0, true)
	}
}

// BenchmarkLyricStyleForDistance_Distant measures a far-away dimmed line.
func BenchmarkLyricStyleForDistance_Distant(b *testing.B) {
	for range b.N {
		_ = lyricStyleForDistance(8, false)
	}
}

// BenchmarkLyricStyleForDistance_Render measures the full render call on the
// hot path: style lookup + Width + Render.
func BenchmarkLyricStyleForDistance_Render(b *testing.B) {
	text := "一生所爱 · 卢冠廷"
	b.ResetTimer()
	for range b.N {
		_ = lyricStyleForDistance(0, true).Width(panelWidth).Render(text)
	}
}

// BenchmarkLyricStyleForDistance_Frame simulates rendering a full 20-row lyric
// panel (the innermost loop of renderLyricsScroll).
func BenchmarkLyricStyleForDistance_Frame(b *testing.B) {
	lines := []string{
		"Intro", "Verse 1", "一生所爱", "Beautiful day", "Bridge",
		"Chorus", "一生所爱", "Oh yeah", "Pre-chorus", "Verse 2",
		"一生所爱", "Keep going", "Outro", "一生所爱", "End",
		"Fade out", "一生所爱", "Done", "Credits", "EOF",
	}
	const h = 20
	centerRow := h / 2
	b.ResetTimer()
	for range b.N {
		for row := range h {
			dist := row - centerRow
			if dist < 0 {
				dist = -dist
			}
			isActive := row == centerRow
			_ = lyricStyleForDistance(dist, isActive).Width(panelWidth).Render(lines[row])
		}
	}
}
