package library

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	id3 "github.com/bogem/id3v2/v2"
	"github.com/gopxl/beep/v2/mp3"
)

// ScanDir walks the given directory and returns all MP3 tracks found.
// Subdirectories are traversed recursively.
// CoverArt is populated in-memory; use ParseTrackWithCover to persist covers.
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

// ParseTrackWithCover reads ID3 metadata for a single file.
// If cover art is found and coverCacheDir is non-empty, the cover is written
// to coverCacheDir/<sha256(path)[:8]>.jpg and Track.CoverPath is set.
// Track.CoverArt is NOT populated (to avoid keeping large blobs in memory).
func ParseTrackWithCover(path, coverCacheDir string) (Track, error) {
	t, err := parseTrack(path)
	if err != nil {
		return t, err
	}

	// Write cover art to the cache directory if available.
	if len(t.CoverArt) > 0 && coverCacheDir != "" {
		h := sha256.Sum256([]byte(path))
		fname := fmt.Sprintf("%x.jpg", h[:8])
		coverPath := filepath.Join(coverCacheDir, fname)
		if werr := os.WriteFile(coverPath, t.CoverArt, 0o644); werr == nil {
			t.CoverPath = coverPath
		}
		t.CoverArt = nil // release memory
	}

	// Stable ID derived from path.
	h := sha256.Sum256([]byte(path))
	t.ID = fmt.Sprintf("%x", h[:8])

	return t, nil
}

// isSupportedAudio reports whether the file extension is a supported audio format.
func isSupportedAudio(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp3"
}

// parseTrack reads ID3 metadata and audio duration from an MP3 file.
// CoverArt is populated in-memory; the caller decides whether to persist it.
func parseTrack(path string) (Track, error) {
	var title, artist, album string
	var coverArt []byte

	if tag, err := id3.Open(path, id3.Options{Parse: true}); err == nil {
		title = tag.Title()
		artist = tag.Artist()
		album = tag.Album()
		frames := tag.GetFrames(tag.CommonID("Attached picture"))
		if len(frames) > 0 {
			if pic, ok := frames[0].(id3.PictureFrame); ok && len(pic.Picture) > 0 {
				coverArt = make([]byte, len(pic.Picture))
				copy(coverArt, pic.Picture)
			}
		}
		tag.Close()
	}

	duration := readMP3Duration(path)

	return Track{
		ID:       path, // overwritten by ParseTrackWithCover
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
