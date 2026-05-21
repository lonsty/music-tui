package library

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	id3 "github.com/bogem/id3v2/v2"
)

// ScanDir walks the given directory and returns all MP3 tracks found.
// Subdirectories are traversed recursively.
func ScanDir(dir string) ([]Track, error) {
	var tracks []Track

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isSupportedAudio(path) {
			return nil
		}

		track, parseErr := parseTrack(path)
		if parseErr != nil {
			// Skip files we can't parse but continue scanning.
			return nil
		}
		tracks = append(tracks, track)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan dir %q: %w", dir, err)
	}

	return tracks, nil
}

// isSupportedAudio reports whether the file extension is a supported audio format.
func isSupportedAudio(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp3"
}

// parseTrack reads ID3 metadata from an MP3 file and returns a Track.
func parseTrack(path string) (Track, error) {
	tag, err := id3.Open(path, id3.Options{Parse: true})
	if err != nil {
		// Return a minimal track with just the filename when ID3 parsing fails.
		return Track{
			ID:     path,
			Path:   path,
			Source: SourceLocal,
		}, nil
	}
	defer tag.Close()

	title := tag.Title()
	artist := tag.Artist()
	album := tag.Album()

	// Try to read duration from TLEN frame (milliseconds).
	var duration time.Duration
	if tlen := tag.GetTextFrame(tag.CommonID("Length")); tlen.Text != "" {
		var ms int64
		if _, scanErr := fmt.Sscanf(tlen.Text, "%d", &ms); scanErr == nil {
			duration = time.Duration(ms) * time.Millisecond
		}
	}

	return Track{
		ID:       path,
		Title:    title,
		Artist:   artist,
		Album:    album,
		Duration: duration,
		Path:     path,
		Source:   SourceLocal,
	}, nil
}
