package lyrics

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	id3 "github.com/bogem/id3v2/v2"

	"github.com/eilianxiao/music-tui/internal/library"
)

// LocalLRCProvider fetches lyrics from a lyrics file located near the audio
// file.  It searches the following candidate paths in priority order:
//
//  1. <audio-dir>/<base>.lrc   — same directory, standard name
//  2. <audio-dir>/Lyrics/<base>.lrc — Lyrics sub-directory (title-case)
//  3. <audio-dir>/lyrics/<base>.lrc — lyrics sub-directory (lower-case)
//  4. <audio-dir>/<base>.srt   — SRT subtitle format, same directory
//
// It understands three file formats:
//   - Standard LRC:  [mm:ss.xx] line text
//   - Enhanced LRC:  [mm:ss.xx]<mm:ss.xx>word … (word-by-word tags stripped,
//     treated as standard line-level LRC)
//   - SRT:           index / timestamp-range / text blocks
//   - Plain-text:    .lrc with no valid timestamps; lines shown statically
//     (all assigned Time = 0 so they display from the start)
type LocalLRCProvider struct{}

// Fetch implements Provider.
func (p LocalLRCProvider) Fetch(_ context.Context, track library.Track) ([]Line, error) {
	if track.Path == "" {
		return nil, nil
	}

	// 1. Try external lyrics files in priority order.
	for _, candidate := range candidatePaths(track.Path) {
		lines, err := tryLoad(candidate)
		if err != nil {
			return nil, err
		}
		if lines != nil {
			return lines, nil
		}
	}

	// 2. Fall back to ID3 USLT (unsynchronised lyrics) embedded in the tag.
	//    USLT carries no timestamps, so lines are returned with Time=0 for
	//    static display (same as the plain-text LRC fallback).
	return loadUSLT(track.Path)
}

// candidatePaths returns the ordered list of file paths to check for lyrics.
func candidatePaths(audioPath string) []string {
	dir := filepath.Dir(audioPath)
	ext := filepath.Ext(audioPath)
	base := filepath.Base(audioPath[:len(audioPath)-len(ext)])

	return []string{
		filepath.Join(dir, base+".lrc"),
		filepath.Join(dir, "Lyrics", base+".lrc"),
		filepath.Join(dir, "lyrics", base+".lrc"),
		filepath.Join(dir, base+".srt"),
	}
}

// tryLoad opens path and parses its lyrics.
// Returns (nil, nil) when the file does not exist.
// Returns (lines, nil) on success, or (nil, err) on a real error.
func tryLoad(path string) ([]Line, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open lyrics %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".srt":
		return parseSRT(f)
	default:
		return parseLRC(f)
	}
}

// ── LRC parser ────────────────────────────────────────────────────────────────

// ParseLRCString parses an LRC-formatted string and returns the resulting lines.
// It is the public entry point for callers that already have the lyrics as a
// string (e.g. fetched from an online API).
func ParseLRCString(s string) []Line {
	lines, _ := parseLRCReader(strings.NewReader(s))
	return lines
}

// PlainTextToLines splits a plain-text lyrics string into static lines
// (all with Time = 0).  Empty lines are dropped.
func PlainTextToLines(s string) []Line {
	var out []Line
	for _, raw := range strings.Split(s, "\n") {
		text := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if text != "" {
			out = append(out, Line{Time: 0, Text: text})
		}
	}
	return out
}

// parseLRC parses an LRC file, supporting:
//
//  1. Standard LRC:       [mm:ss.xx]text
//  2. Multi-stamp LRC:    [mm:ss.xx][mm:ss.xx]text  — same text at multiple times
//  3. QQ word-by-word:    [mm:ss.xxx]字[mm:ss.xxx]字… — merged into one line
//  4. Enhanced LRC:       <mm:ss.xx> word tags stripped, treated as standard
//  5. Metadata tags:      [ar:…] [ti:…] silently ignored
//  6. Plain-text:         no timestamps → lines returned with Time=0
func parseLRC(f *os.File) ([]Line, error) {
	return parseLRCReader(f)
}

// parseLRCReader is the shared implementation used by parseLRC and ParseLRCString.
func parseLRCReader(r io.Reader) ([]Line, error) {
	var timed []Line
	var plainLines []string

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		if !strings.HasPrefix(raw, "[") {
			plainLines = append(plainLines, raw)
			continue
		}

		line, plain := parseLRCLine(raw)
		if plain != "" {
			// Line had no valid timestamps — treat as plain text.
			plainLines = append(plainLines, plain)
		}
		timed = append(timed, line...)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read lrc: %w", err)
	}

	if len(timed) > 0 {
		sort.Slice(timed, func(i, j int) bool {
			return timed[i].Time < timed[j].Time
		})
		return timed, nil
	}

	// Plain-text fallback: no valid timestamps found.
	if len(plainLines) > 0 {
		out := make([]Line, len(plainLines))
		for i, t := range plainLines {
			out[i] = Line{Time: 0, Text: t}
		}
		return out, nil
	}

	return nil, nil // file was effectively empty
}

// parseLRCLine parses one raw LRC line and returns the resulting timed lines.
//
// Format detection is structural, not heuristic:
//
//   - Standard / multi-stamp: all [time] tags are consecutive at line start,
//     followed by a single text block.  Tokenised as: stamp stamp … text.
//     All stamps share the same text.
//
//   - QQ word-by-word: [time] tags and text fragments alternate.
//     Tokenised as: stamp text stamp text …
//     Each pair has its own text immediately after the stamp.  The full
//     sentence (all fragments joined) is emitted once at the first stamp.
//
//   - Enhanced LRC <mm:ss.xx> word tags are stripped before tokenising.
//
// If no valid timestamp is found, plain contains the human-readable text.
func parseLRCLine(raw string) (lines []Line, plain string) {
	// Tokenise the line into alternating [stamp] and text segments.
	type segment struct {
		stamp time.Duration
		valid bool   // true = valid timestamp tag
		text  string // non-empty for text between/after tags
	}

	var segments []segment
	rest := raw

	for len(rest) > 0 {
		if rest[0] == '[' {
			end := strings.Index(rest, "]")
			if end < 0 {
				segments = append(segments, segment{text: rest})
				break
			}
			tag := rest[1:end]
			rest = rest[end+1:]
			if d, ok := parseTimestamp(tag); ok {
				segments = append(segments, segment{stamp: d, valid: true})
			}
			// Metadata tags (ar:, ti:, …) → ok=false, silently dropped.
		} else {
			next := strings.Index(rest, "[")
			var chunk string
			if next < 0 {
				chunk = rest
				rest = ""
			} else {
				chunk = rest[:next]
				rest = rest[next:]
			}
			chunk = stripWordTags(chunk) // remove Enhanced-LRC <word> tags
			if chunk != "" {
				segments = append(segments, segment{text: chunk})
			}
		}
	}

	// Build (timestamp, immediateText) pairs.
	// immediateText = the text segment that immediately follows a stamp
	// (before the next stamp).  In standard LRC this is only non-empty for
	// the last stamp; in QQ word-by-word every stamp has non-empty text.
	type pair struct {
		t             time.Duration
		immediateText string // text directly after this stamp, before next stamp
	}
	var pairs []pair

	for _, seg := range segments {
		if seg.valid {
			pairs = append(pairs, pair{t: seg.stamp})
		} else if len(pairs) > 0 {
			// Attach text to the stamp that immediately preceded it.
			pairs[len(pairs)-1].immediateText += seg.text
		}
	}

	if len(pairs) == 0 {
		// No valid timestamps — collect all text as plain.
		var sb strings.Builder
		for _, seg := range segments {
			sb.WriteString(seg.text)
		}
		if p := strings.TrimSpace(sb.String()); p != "" {
			plain = p
		}
		return nil, plain
	}

	// ── Format detection ────────────────────────────────────────────────────
	//
	// QQ word-by-word: more than one pair has non-empty immediateText.
	// Standard:        at most one pair (the last) has non-empty immediateText.
	//
	// This is a structural property of the token sequence and does not depend
	// on text length, language, or character count.
	stampsWithText := 0
	for _, p := range pairs {
		if strings.TrimSpace(p.immediateText) != "" {
			stampsWithText++
		}
	}
	isWordByWord := stampsWithText > 1

	if isWordByWord {
		// Join all character fragments to form the complete sentence, emitted
		// once at the timestamp of the first stamp.
		var sb strings.Builder
		for _, p := range pairs {
			sb.WriteString(strings.TrimSpace(p.immediateText))
		}
		sentence := strings.TrimSpace(sb.String())
		return []Line{{Time: pairs[0].t, Text: sentence}}, ""
	}

	// Standard / multi-stamp: all stamps share the text from the last pair.
	// (In a single-stamp line the last pair is also the only pair.)
	text := strings.TrimSpace(pairs[len(pairs)-1].immediateText)
	for _, p := range pairs {
		lines = append(lines, Line{Time: p.t, Text: text})
	}
	return lines, ""
}

// stripWordTags removes Enhanced LRC word-level time tags of the form
// <mm:ss.xx> or <mm:ss.xxx> from s, leaving only the lyric text.
func stripWordTags(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		if s[0] != '<' {
			out.WriteByte(s[0])
			s = s[1:]
			continue
		}
		end := strings.Index(s, ">")
		if end < 0 {
			out.WriteString(s)
			break
		}
		// Only strip if the content looks like a timestamp.
		inner := s[1:end]
		if _, ok := parseTimestamp(inner); ok {
			s = s[end+1:]
		} else {
			out.WriteByte(s[0])
			s = s[1:]
		}
	}
	return out.String()
}

// ── SRT parser ────────────────────────────────────────────────────────────────

// parseSRT parses a SubRip subtitle file into timed lyric lines.
//
// SRT block format:
//
//	<index>
//	<start> --> <end>
//	<text line 1>
//	<text line 2>
//	<blank line>
//
// Only the start timestamp is used; end timestamps and HTML tags are ignored.
// Multiple text lines within one block are joined with a space.
func parseSRT(f *os.File) ([]Line, error) {
	var lines []Line
	scanner := bufio.NewScanner(f)

	type state int
	const (
		stateIndex state = iota
		stateTimestamp
		stateText
	)

	cur := stateIndex
	var startTime time.Duration
	var textParts []string

	flush := func() {
		if len(textParts) > 0 {
			lines = append(lines, Line{
				Time: startTime,
				Text: strings.Join(textParts, " "),
			})
			textParts = textParts[:0]
		}
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch cur {
		case stateIndex:
			if line == "" {
				continue
			}
			// Expect a block index (integer).  Skip non-numeric lines
			// gracefully (e.g. BOM or byte-order marks at file start).
			if _, err := strconv.Atoi(line); err == nil {
				cur = stateTimestamp
			}

		case stateTimestamp:
			if line == "" {
				cur = stateIndex
				continue
			}
			// "00:01:02,300 --> 00:01:04,500"
			parts := strings.SplitN(line, "-->", 2)
			if len(parts) != 2 {
				cur = stateIndex
				continue
			}
			t, ok := parseSRTTimestamp(strings.TrimSpace(parts[0]))
			if !ok {
				cur = stateIndex
				continue
			}
			startTime = t
			cur = stateText

		case stateText:
			if line == "" {
				flush()
				cur = stateIndex
				continue
			}
			// Strip basic HTML tags used in SRT (<b>, <i>, <font …>, etc.)
			textParts = append(textParts, stripHTMLTags(line))
		}
	}
	// Flush any trailing block not terminated by a blank line.
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read srt: %w", err)
	}
	return lines, nil
}

// parseSRTTimestamp parses "hh:mm:ss,mmm" or "hh:mm:ss.mmm" into a Duration.
func parseSRTTimestamp(s string) (time.Duration, bool) {
	// Normalise comma decimal separator to dot.
	s = strings.ReplaceAll(s, ",", ".")

	// Expected: HH:MM:SS.mmm
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return 0, false
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	// parts[2] may be "SS.mmm"
	secParts := strings.SplitN(parts[2], ".", 2)
	sec, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, false
	}
	var ms int
	if len(secParts) == 2 {
		frac := secParts[1]
		for len(frac) < 3 {
			frac += "0"
		}
		frac = frac[:3]
		ms, err = strconv.Atoi(frac)
		if err != nil {
			return 0, false
		}
	}
	d := time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond
	return d, true
}

// stripHTMLTags removes simple HTML/XML tags from s (e.g. <b>, </i>, <font …>).
func stripHTMLTags(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		if s[0] != '<' {
			out.WriteByte(s[0])
			s = s[1:]
			continue
		}
		end := strings.Index(s, ">")
		if end < 0 {
			out.WriteString(s)
			break
		}
		s = s[end+1:]
	}
	return out.String()
}

// ── Shared timestamp parser ───────────────────────────────────────────────────

// parseTimestamp parses "mm:ss", "mm:ss.xx", or "mm:ss.xxx" into a Duration.
// Returns (0, false) for unrecognised formats such as LRC metadata tags.
func parseTimestamp(s string) (time.Duration, bool) {
	colonIdx := strings.Index(s, ":")
	if colonIdx < 0 {
		return 0, false
	}
	minStr := s[:colonIdx]
	secStr := s[colonIdx+1:]

	mins, err := strconv.Atoi(minStr)
	if err != nil {
		return 0, false
	}

	dotIdx := strings.Index(secStr, ".")
	var secs int
	var frac time.Duration
	if dotIdx >= 0 {
		secs, err = strconv.Atoi(secStr[:dotIdx])
		if err != nil {
			return 0, false
		}
		fracStr := secStr[dotIdx+1:]
		// Normalise to milliseconds (pad/truncate to 3 digits).
		for len(fracStr) < 3 {
			fracStr += "0"
		}
		fracStr = fracStr[:3]
		fracMs, err2 := strconv.Atoi(fracStr)
		if err2 != nil {
			return 0, false
		}
		frac = time.Duration(fracMs) * time.Millisecond
	} else {
		secs, err = strconv.Atoi(secStr)
		if err != nil {
			return 0, false
		}
	}

	return time.Duration(mins)*time.Minute +
		time.Duration(secs)*time.Second +
		frac, true
}

// ── ID3 tag lyrics ────────────────────────────────────────────────────────────

// loadUSLT reads the ID3v2 USLT (Unsynchronised Lyrics) frame from an MP3
// file and returns its content as static lines (all with Time = 0).
//
// USLT carries no timestamps, so the lyrics are shown as plain text from the
// start of playback — the same experience as a plain-text .lrc fallback.
// Returns (nil, nil) when the file has no USLT frame or is not an MP3.
func loadUSLT(audioPath string) ([]Line, error) {
	// Only attempt for .mp3 files — other formats use different tag specs.
	if !strings.EqualFold(filepath.Ext(audioPath), ".mp3") {
		return nil, nil
	}

	tag, err := id3.Open(audioPath, id3.Options{Parse: true})
	if err != nil {
		// Not a valid ID3 file or other I/O error — treat as "no lyrics".
		return nil, nil
	}
	defer func() { _ = tag.Close() }()

	frames := tag.GetFrames(tag.CommonID("Unsynchronised lyrics/text transcription"))
	if len(frames) == 0 {
		return nil, nil
	}

	// Use the first USLT frame.  Multiple frames with different languages are
	// possible; we pick the first one (usually the primary language).
	uslf, ok := frames[0].(id3.UnsynchronisedLyricsFrame)
	if !ok || strings.TrimSpace(uslf.Lyrics) == "" {
		return nil, nil
	}

	// Split the lyrics text into individual lines.
	var lines []Line
	for _, raw := range strings.Split(uslf.Lyrics, "\n") {
		text := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if text != "" {
			lines = append(lines, Line{Time: 0, Text: text})
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}
	return lines, nil
}
