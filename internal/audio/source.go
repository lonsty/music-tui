// Package audio provides a thread-safe audio player backed by the beep library.
package audio

import (
	"context"
	"fmt"
	"os"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
)

// StreamSource is the abstraction over all audio origins.
// Implementations include local MP3 files, HTTP streams, and platform APIs.
type StreamSource interface {
	// Open opens the audio stream for the given context.
	// On success the returned StreamSeekCloser is owned by the caller, which
	// must Close it when done.  On error the returned value is nil.
	Open(ctx context.Context) (beep.StreamSeekCloser, beep.Format, error)
}

// LocalSource reads an MP3 file from the local filesystem.
type LocalSource struct {
	// Path is the absolute path to the local MP3 file.
	Path string
}

// Open implements StreamSource for a local MP3 file.
func (s LocalSource) Open(_ context.Context) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("open %q: %w", s.Path, err)
	}
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		_ = f.Close()
		return nil, beep.Format{}, fmt.Errorf("decode mp3 %q: %w", s.Path, err)
	}
	// streamer wraps f; streamer.Close() will close f.
	return streamer, format, nil
}
