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
	"github.com/gopxl/beep/v2/mp3"
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
//  1. Import the appropriate beep codec package.
//  2. Insert an entry here.
//  3. Add the extension to SupportedExtensions in internal/library/formats.go.
var decoders = map[string]decoderFn{
	".mp3": func(r io.ReadCloser) (beep.StreamSeekCloser, beep.Format, error) {
		return mp3.Decode(r)
	},
}

// ── LocalSource ───────────────────────────────────────────────────────────────

// LocalSource reads an audio file from the local filesystem.
// The file format is detected from the file extension; see decoders above for
// the list of supported extensions.
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
