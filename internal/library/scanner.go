package library

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	id3 "github.com/bogem/id3v2/v2"
	"github.com/gopxl/beep/v2/mp3"
	"os"
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

// parseTrack reads ID3 metadata and audio duration from an MP3 file.
func parseTrack(path string) (Track, error) {
	// Read ID3 tags.
	var title, artist, album string
	var coverArt []byte
	if tag, err := id3.Open(path, id3.Options{Parse: true}); err == nil {
		title = tag.Title()
		artist = tag.Artist()
		album = tag.Album()
		// Extract embedded cover art from the first APIC (Attached picture) frame.
		frames := tag.GetFrames(tag.CommonID("Attached picture"))
		if len(frames) > 0 {
			if pic, ok := frames[0].(id3.PictureFrame); ok && len(pic.Picture) > 0 {
				coverArt = make([]byte, len(pic.Picture))
				copy(coverArt, pic.Picture)
			}
		}
		tag.Close()
	}

	// Read actual duration via beep/mp3 — more reliable than TLEN tag.
	duration := readMP3Duration(path)

	return Track{
		ID:       path,
		Title:    title,
		Artist:   artist,
		Album:    album,
		Duration: duration,
		Path:     path,
		Source:   SourceLocal,
		CoverArt: coverArt,
	}, nil
}

// readMP3Duration opens the file, decodes the MP3 header, and returns the
// track length. Returns 0 on any error.
func readMP3Duration(path string) time.Duration {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		return 0
	}
	defer streamer.Close()

	return format.SampleRate.D(streamer.Len())
}
