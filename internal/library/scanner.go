package library

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// ScanDir walks the given directory and returns all supported audio tracks.
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
		if !IsSupportedAudio(path) {
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

// ParseTrackWithCover reads metadata for a single audio file.
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

// parseTrack reads metadata and audio duration from any supported audio file.
// CoverArt is populated in-memory; the caller decides whether to persist it.
//
// Metadata is read via github.com/dhowden/tag which supports the tag formats
// used by all supported audio containers:
//
//   - MP3:  ID3v1, ID3v2.2, ID3v2.3, ID3v2.4
//   - FLAC: Vorbis Comment (+ native FLAC metadata blocks)
//   - OGG:  Vorbis Comment
//   - WAV:  ID3v2 chunk (when present; WAV has no standard tag format)
func parseTrack(path string) (Track, error) {
	var title, artist, albumArtist, album, year, trackNum, genre, comment string
	var coverArt []byte

	if m, err := readTags(path); err == nil {
		title = m.Title()
		artist = m.Artist()
		album = m.Album()
		albumArtist = m.AlbumArtist()
		genre = m.Genre()

		if m.Year() != 0 {
			year = fmt.Sprintf("%d", m.Year())
		}

		if tn, _ := m.Track(); tn != 0 {
			trackNum = fmt.Sprintf("%d", tn)
		}

		// Lyrics — dhowden/tag exposes embedded lyrics for some formats.
		comment = m.Lyrics()

		// Cover art — returns nil when no embedded picture is found.
		if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
			coverArt = make([]byte, len(pic.Data))
			copy(coverArt, pic.Data)
		}
	}

	// Derive the format from the file extension and store it explicitly so
	// the database can filter by format without parsing paths.
	ext := strings.ToLower(filepath.Ext(path))
	fileFormat := strings.ToUpper(strings.TrimPrefix(ext, "."))

	duration := readDuration(path)

	return Track{
		ID:          path, // overwritten by ParseTrackWithCover
		Title:       title,
		Artist:      artist,
		AlbumArtist: albumArtist,
		Album:       album,
		Year:        year,
		TrackNumber: trackNum,
		Genre:       genre,
		Comment:     comment,
		Duration:    duration,
		FileFormat:  fileFormat,
		Path:        path,
		Source:      SourceLocal,
		CoverArt:    coverArt,
	}, nil
}

// readTags opens path and reads audio tags using github.com/dhowden/tag.
// The caller must not close the returned Metadata; its internal file handle
// is closed when the function returns.
func readTags(path string) (tag.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ── Duration readers ──────────────────────────────────────────────────────────

// scannerDecoderFn is the signature for format-specific duration readers.
// It mirrors the decoderFn type in the audio package but is kept separate to
// avoid an import cycle (library → audio would be circular since audio already
// uses library types indirectly via the TUI layer).
type scannerDecoderFn func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error)

// scannerDecoders maps lower-case file extensions to their duration-decoder.
// This map must be kept in sync with audio.decoders in internal/audio/source.go.
var scannerDecoders = map[string]scannerDecoderFn{
	".mp3": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return mp3.Decode(r)
	},
	".flac": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return flac.Decode(r)
	},
	".wav": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return wav.Decode(r)
	},
	".wave": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return wav.Decode(r)
	},
	".ogg": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return vorbis.Decode(r)
	},
	".oga": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return vorbis.Decode(r)
	},
}

// readDuration opens path, decodes enough of the stream to determine length,
// and returns the track duration.  Returns 0 on any error.
//
// The format is detected from the file extension via scannerDecoders.
// This function is intentionally separate from the tag-reading path so that
// a missing or malformed tag does not prevent the duration from being read.
func readDuration(path string) time.Duration {
	ext := strings.ToLower(filepath.Ext(path))
	dec, ok := scannerDecoders[ext]
	if !ok {
		return 0
	}

	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	streamer, format, err := dec(f)
	if err != nil {
		return 0
	}
	defer func() { _ = streamer.Close() }()

	return format.SampleRate.D(streamer.Len())
}
