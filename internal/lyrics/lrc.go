package lyrics

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eilianxiao/music-tui/internal/library"
)

// LocalLRCProvider fetches lyrics from a .lrc file located in the same
// directory as the audio file, with the same base name.
//
// For example, given /music/song.mp3 it looks for /music/song.lrc.
type LocalLRCProvider struct{}

// Fetch implements Provider.
func (p LocalLRCProvider) Fetch(_ context.Context, track library.Track) ([]Line, error) {
	if track.Path == "" {
		return nil, nil
	}
	lrcPath := lrcPathFor(track.Path)
	f, err := os.Open(lrcPath)
	if os.IsNotExist(err) {
		return nil, nil // no lyrics file is normal
	}
	if err != nil {
		return nil, fmt.Errorf("open lrc %q: %w", lrcPath, err)
	}
	defer func() { _ = f.Close() }()

	return parseLRC(f)
}

// lrcPathFor returns the expected .lrc path for a given audio file path.
func lrcPathFor(audioPath string) string {
	ext := filepath.Ext(audioPath)
	return audioPath[:len(audioPath)-len(ext)] + ".lrc"
}

// parseLRC parses an LRC file and returns the timed lines sorted by timestamp.
// It handles the common [mm:ss.xx] and [mm:ss.xxx] timestamp formats.
// Lines without timestamps are silently ignored.
func parseLRC(r *os.File) ([]Line, error) {
	var lines []Line
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || !strings.HasPrefix(raw, "[") {
			continue
		}
		// A line may have multiple timestamps: [00:01.00][00:02.00]text
		rest := raw
		var timestamps []time.Duration
		for strings.HasPrefix(rest, "[") {
			end := strings.Index(rest, "]")
			if end < 0 {
				break
			}
			tag := rest[1:end]
			rest = rest[end+1:]
			if d, ok := parseTimestamp(tag); ok {
				timestamps = append(timestamps, d)
			}
		}
		text := strings.TrimSpace(rest)
		for _, ts := range timestamps {
			lines = append(lines, Line{Time: ts, Text: text})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read lrc: %w", err)
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].Time < lines[j].Time
	})
	return lines, nil
}

// parseTimestamp parses "mm:ss.xx" or "mm:ss.xxx" into a Duration.
// Returns (0, false) for unrecognised formats such as metadata tags.
func parseTimestamp(s string) (time.Duration, bool) {
	// Expect "mm:ss" or "mm:ss.xx[x]"
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

	// Parse seconds, possibly with centiseconds/milliseconds.
	dotIdx := strings.Index(secStr, ".")
	var secs int
	var frac time.Duration
	if dotIdx >= 0 {
		var err2 error
		secs, err2 = strconv.Atoi(secStr[:dotIdx])
		if err2 != nil {
			return 0, false
		}
		fracStr := secStr[dotIdx+1:]
		// Normalise to milliseconds (pad/truncate to 3 digits).
		for len(fracStr) < 3 {
			fracStr += "0"
		}
		fracStr = fracStr[:3]
		fracMs, err3 := strconv.Atoi(fracStr)
		if err3 != nil {
			return 0, false
		}
		frac = time.Duration(fracMs) * time.Millisecond
	} else {
		var err4 error
		secs, err4 = strconv.Atoi(secStr)
		if err4 != nil {
			return 0, false
		}
	}

	d := time.Duration(mins)*time.Minute +
		time.Duration(secs)*time.Second +
		frac
	return d, true
}
