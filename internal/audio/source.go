// Package audio provides a thread-safe audio player backed by the beep library.
package audio

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// StreamSource is the abstraction over all audio origins.
// Implementations include local files, HTTP streams, and platform APIs.
type StreamSource interface {
	// Open opens the audio stream for the given context.
	// On success the returned StreamSeekCloser is owned by the caller, which
	// must Close it when done.  On error the returned value is nil.
	Open(ctx context.Context) (beep.StreamSeekCloser, beep.Format, error)
}

// ── Decoder registry ──────────────────────────────────────────────────────────

// decoderFn is the signature for format-specific audio decoders.
// r is the raw file reader; it will be closed by the returned StreamSeekCloser
// (or by the caller on error).
type decoderFn func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error)

// decoders maps lower-case file extensions (e.g. ".mp3") to their decoder.
// To add support for a new format:
//  1. Import the appropriate beep codec package (or other Go audio library).
//  2. Insert an entry here — the decoder receives an open io.ReadCloser.
//  3. Add the same extension to SupportedExtensions in internal/library/formats.go.
//
// All decoders in this map must return a beep.StreamSeekCloser whose Seek
// method works correctly for local files (panic-on-seek is acceptable for
// non-seekable sources).
var decoders = map[string]decoderFn{
	// ── MP3 ──────────────────────────────────────────────────────────────────
	".mp3": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return mp3.Decode(r)
	},

	// ── FLAC ─────────────────────────────────────────────────────────────────
	// beep/flac wraps github.com/mewkiz/flac (already an indirect dependency).
	// The decoder accepts an io.Reader; we wrap the ReadCloser transparently
	// because flac.Decode stores the reader and calls Close() via its own
	// StreamSeekCloser.Close() — the file is still closed exactly once.
	".flac": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return flac.Decode(r)
	},

	// ── WAV / WAVE ───────────────────────────────────────────────────────────
	// beep/wav accepts an io.Reader.  Standard uncompressed PCM WAV files are
	// supported; compressed WAV variants (ADPCM, etc.) may fail to decode.
	".wav": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return wav.Decode(r)
	},
	".wave": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return wav.Decode(r)
	},

	// ── OGG / Vorbis ─────────────────────────────────────────────────────────
	// beep/vorbis wraps github.com/jfreymuth/oggvorbis (already an indirect
	// dependency).  Only Vorbis-encoded OGG containers are supported here;
	// OGG Opus uses a different codec and is not covered.
	".ogg": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return vorbis.Decode(r)
	},
	".oga": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return vorbis.Decode(r)
	},
}

// ── LocalSource ───────────────────────────────────────────────────────────────

// LocalSource reads an audio file from the local filesystem.
// The file format is detected from the file extension; see the decoders map
// above for the list of supported extensions.
type LocalSource struct {
	// Path is the absolute path to the local audio file.
	Path string
}

// Open implements StreamSource for a local audio file.
// It dispatches to the registered decoder for the file extension.
func (s LocalSource) Open(_ context.Context) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(s.Path))
	dec, ok := decoders[ext]
	if !ok {
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format %q", ext)
	}
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("open %q: %w", s.Path, err)
	}
	streamer, format, err := dec(f)
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, fmt.Errorf("decode %q: %w", s.Path, err)
	}
	// streamer wraps f; streamer.Close() will close f.
	return streamer, format, nil
}
