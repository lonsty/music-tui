package lyrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eilianxiao/music-tui/internal/library"
)

// ── LRC parser tests ──────────────────────────────────────────────────────────

func TestParseLRC_Standard(t *testing.T) {
	content := "[00:01.00]First line\n[00:05.50]Second line\n[00:10.00]Third line\n"
	lines := parseLRCString(t, content)

	want := []Line{
		{Time: 1 * time.Second, Text: "First line"},
		{Time: 5*time.Second + 500*time.Millisecond, Text: "Second line"},
		{Time: 10 * time.Second, Text: "Third line"},
	}
	assertLines(t, want, lines)
}

func TestParseLRC_MultipleTimestamps(t *testing.T) {
	content := "[00:10.00][00:20.00]Chorus line\n"
	lines := parseLRCString(t, content)

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Time != 10*time.Second || lines[1].Time != 20*time.Second {
		t.Errorf("unexpected timestamps: %v", lines)
	}
	if lines[0].Text != "Chorus line" || lines[1].Text != "Chorus line" {
		t.Errorf("unexpected text: %v", lines)
	}
}

func TestParseLRC_MetadataTags(t *testing.T) {
	content := "[ar:Artist]\n[ti:Title]\n[00:01.00]Lyric line\n"
	lines := parseLRCString(t, content)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line (metadata ignored), got %d", len(lines))
	}
	if lines[0].Text != "Lyric line" {
		t.Errorf("unexpected text: %q", lines[0].Text)
	}
}

func TestParseLRC_EnhancedWordTags(t *testing.T) {
	// Enhanced LRC — word tags stripped, only line text retained.
	content := "[00:10.00]<00:10.00>Hello <00:10.50>world\n"
	lines := parseLRCString(t, content)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Hello world" {
		t.Errorf("expected %q, got %q", "Hello world", lines[0].Text)
	}
}

func TestParseLRC_QQWordByWord(t *testing.T) {
	// QQ Music word-by-word format: [time]字[time]字[time]字…
	// Should produce ONE line at the first timestamp with the full sentence.
	content := "[00:29.264]故[00:29.654]事[00:30.046]的[00:30.494]小[00:31.416]黄[00:31.790]花[00:32.294]\n"
	lines := parseLRCString(t, content)

	if len(lines) != 1 {
		t.Fatalf("expected 1 merged line, got %d: %v", len(lines), lines)
	}
	if lines[0].Time != 29*time.Second+264*time.Millisecond {
		t.Errorf("expected first stamp time, got %v", lines[0].Time)
	}
	if lines[0].Text != "故事的小黄花" {
		t.Errorf("expected %q, got %q", "故事的小黄花", lines[0].Text)
	}
}

func TestParseLRC_QQWordByWordFullLine(t *testing.T) {
	// Multiple QQ word-by-word lines.
	content := "[00:10.000]从[00:10.400]前[00:10.800]从[00:11.200]前\n" +
		"[00:15.000]有[00:15.400]个[00:15.800]人[00:16.200]爱[00:16.600]你[00:17.000]很[00:17.400]久\n"
	lines := parseLRCString(t, content)

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0].Text != "从前从前" {
		t.Errorf("line 0: expected %q, got %q", "从前从前", lines[0].Text)
	}
	if lines[1].Text != "有个人爱你很久" {
		t.Errorf("line 1: expected %q, got %q", "有个人爱你很久", lines[1].Text)
	}
}

func TestParseLRC_TwoShortLinesNotMerged(t *testing.T) {
	// Regression: two separate short-text LRC lines must NOT be merged.
	// Previously the heuristic mistook this for QQ word-by-word.
	content := "[00:10.00]好\n[00:20.00]的\n"
	lines := parseLRCString(t, content)

	if len(lines) != 2 {
		t.Fatalf("expected 2 independent lines, got %d: %v", len(lines), lines)
	}
	if lines[0].Time != 10*time.Second || lines[0].Text != "好" {
		t.Errorf("line 0: want {10s, 好}, got %v", lines[0])
	}
	if lines[1].Time != 20*time.Second || lines[1].Text != "的" {
		t.Errorf("line 1: want {20s, 的}, got %v", lines[1])
	}
}

func TestParseLRC_MultiStampShortText(t *testing.T) {
	// Multi-stamp with short text: [t1][t2]OK — both stamps share "OK".
	content := "[00:10.00][00:20.00]OK\n"
	lines := parseLRCString(t, content)

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0].Text != "OK" || lines[1].Text != "OK" {
		t.Errorf("both stamps should share text 'OK', got %v", lines)
	}
}

func TestParseLRC_PlainTextFallback(t *testing.T) {
	// No timestamps — all non-empty lines returned with Time=0.
	content := "First line\nSecond line\n\nThird line\n"
	lines := parseLRCString(t, content)

	if len(lines) != 3 {
		t.Fatalf("expected 3 plain-text lines, got %d", len(lines))
	}
	for _, l := range lines {
		if l.Time != 0 {
			t.Errorf("plain-text line should have Time=0, got %v", l.Time)
		}
	}
	if lines[0].Text != "First line" {
		t.Errorf("unexpected first line: %q", lines[0].Text)
	}
}

// ── SRT parser tests ──────────────────────────────────────────────────────────

func TestParseSRT_Basic(t *testing.T) {
	content := "1\n00:00:01,000 --> 00:00:03,000\nFirst subtitle\n\n" +
		"2\n00:00:05,500 --> 00:00:07,000\nSecond subtitle\n"
	lines := parseSRTString(t, content)

	want := []Line{
		{Time: 1 * time.Second, Text: "First subtitle"},
		{Time: 5*time.Second + 500*time.Millisecond, Text: "Second subtitle"},
	}
	assertLines(t, want, lines)
}

func TestParseSRT_MultiLineBlock(t *testing.T) {
	content := "1\n00:00:01,000 --> 00:00:04,000\nLine one\nLine two\n\n"
	lines := parseSRTString(t, content)

	if len(lines) != 1 {
		t.Fatalf("expected 1 merged line, got %d", len(lines))
	}
	if lines[0].Text != "Line one Line two" {
		t.Errorf("unexpected text: %q", lines[0].Text)
	}
}

func TestParseSRT_HTMLTags(t *testing.T) {
	content := "1\n00:00:01,000 --> 00:00:03,000\n<b>Bold text</b>\n\n"
	lines := parseSRTString(t, content)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Text != "Bold text" {
		t.Errorf("expected %q, got %q", "Bold text", lines[0].Text)
	}
}

// ── Timestamp parser tests ────────────────────────────────────────────────────

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
		ok    bool
	}{
		{"01:30.00", 90 * time.Second, true},
		{"01:30.50", 90*time.Second + 500*time.Millisecond, true},
		{"01:30.500", 90*time.Second + 500*time.Millisecond, true},
		{"01:30", 90 * time.Second, true},
		{"ar:Artist", 0, false},
		{"notatime", 0, false},
	}
	for _, c := range cases {
		d, ok := parseTimestamp(c.input)
		if ok != c.ok {
			t.Errorf("parseTimestamp(%q) ok=%v, want %v", c.input, ok, c.ok)
			continue
		}
		if ok && d != c.want {
			t.Errorf("parseTimestamp(%q) = %v, want %v", c.input, d, c.want)
		}
	}
}

// ── candidatePaths tests ──────────────────────────────────────────────────────

func TestCandidatePaths(t *testing.T) {
	paths := candidatePaths("/music/song.mp3")
	want := []string{
		"/music/song.lrc",
		"/music/Lyrics/song.lrc",
		"/music/lyrics/song.lrc",
		"/music/song.srt",
	}
	if len(paths) != len(want) {
		t.Fatalf("expected %d paths, got %d: %v", len(want), len(paths), paths)
	}
	for i, p := range paths {
		if p != want[i] {
			t.Errorf("path[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestFetch_PriorityOrder(t *testing.T) {
	// Both song.lrc and Lyrics/song.lrc exist — top-level wins.
	dir := t.TempDir()
	songPath := filepath.Join(dir, "song.mp3")

	if err := os.WriteFile(filepath.Join(dir, "song.lrc"),
		[]byte("[00:01.00]Top level\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "Lyrics")
	_ = os.Mkdir(subdir, 0o755)
	if err := os.WriteFile(filepath.Join(subdir, "song.lrc"),
		[]byte("[00:01.00]Sub level\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := LocalLRCProvider{}
	lines, err := p.Fetch(nil, library.Track{Path: songPath})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(lines) == 0 || lines[0].Text != "Top level" {
		t.Errorf("expected top-level lrc to win, got %v", lines)
	}
}

func TestFetch_FallsBackToSubdir(t *testing.T) {
	// Only Lyrics/song.lrc exists.
	dir := t.TempDir()
	songPath := filepath.Join(dir, "song.mp3")
	subdir := filepath.Join(dir, "Lyrics")
	_ = os.Mkdir(subdir, 0o755)
	if err := os.WriteFile(filepath.Join(subdir, "song.lrc"),
		[]byte("[00:01.00]Sub only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := LocalLRCProvider{}
	lines, err := p.Fetch(nil, library.Track{Path: songPath})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(lines) == 0 || lines[0].Text != "Sub only" {
		t.Errorf("expected sub-dir lrc, got %v", lines)
	}
}

func TestFetch_SRT(t *testing.T) {
	// No .lrc exists — should fall back to .srt.
	dir := t.TempDir()
	songPath := filepath.Join(dir, "song.mp3")
	content := "1\n00:00:01,000 --> 00:00:03,000\nSRT line\n\n"
	if err := os.WriteFile(filepath.Join(dir, "song.srt"),
		[]byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := LocalLRCProvider{}
	lines, err := p.Fetch(nil, library.Track{Path: songPath})
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(lines) == 0 || lines[0].Text != "SRT line" {
		t.Errorf("expected SRT line, got %v", lines)
	}
}

func TestFetch_NoLyrics(t *testing.T) {
	dir := t.TempDir()
	songPath := filepath.Join(dir, "song.mp3")

	p := LocalLRCProvider{}
	lines, err := p.Fetch(nil, library.Track{Path: songPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil lines when no lyrics file exists, got %v", lines)
	}
}

func TestLoadUSLT_NonMP3(t *testing.T) {
	// loadUSLT should return nil for non-MP3 files without error.
	lines, err := loadUSLT("/some/file.flac")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil for non-MP3, got %v", lines)
	}
}

func TestLoadUSLT_MissingFile(t *testing.T) {
	// A path that doesn't exist should return nil, nil (treated as no lyrics).
	lines, err := loadUSLT("/nonexistent/song.mp3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lines != nil {
		t.Errorf("expected nil for missing file, got %v", lines)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func parseLRCString(t *testing.T, content string) []Line {
	t.Helper()
	f := writeTempFile(t, content, ".lrc")
	lines, err := parseLRC(f)
	if err != nil {
		t.Fatalf("parseLRC error: %v", err)
	}
	return lines
}

func parseSRTString(t *testing.T, content string) []Line {
	t.Helper()
	f := writeTempFile(t, content, ".srt")
	lines, err := parseSRT(f)
	if err != nil {
		t.Fatalf("parseSRT error: %v", err)
	}
	return lines
}

func writeTempFile(t *testing.T, content, ext string) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*"+ext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func assertLines(t *testing.T, want, got []Line) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("line count: want %d, got %d\nwant: %v\ngot:  %v",
			len(want), len(got), want, got)
	}
	for i := range want {
		if got[i].Time != want[i].Time || got[i].Text != want[i].Text {
			t.Errorf("line[%d]: want %v, got %v", i, want[i], got[i])
		}
	}
}
