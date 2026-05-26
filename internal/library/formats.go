package library

import (
	"path/filepath"
	"strings"
)

// SupportedExtensions is the set of audio file extensions that music-tui can
// scan and play.  When adding a new format, you must also register a decoder
// in internal/audio/source.go (the decoders map) — both maps must stay in sync.
var SupportedExtensions = map[string]bool{
	// MP3 — MPEG Audio Layer III
	".mp3": true,
	// FLAC — Free Lossless Audio Codec (via beep/flac + mewkiz/flac)
	".flac": true,
	// WAV — Waveform Audio File Format, uncompressed PCM (via beep/wav)
	".wav":  true,
	".wave": true,
	// OGG / Vorbis — Ogg container with Vorbis audio (via beep/vorbis + jfreymuth/oggvorbis)
	".ogg": true,
	".oga": true,
}

// IsSupportedAudio reports whether path has a supported audio file extension.
func IsSupportedAudio(path string) bool {
	return SupportedExtensions[strings.ToLower(filepath.Ext(path))]
}
