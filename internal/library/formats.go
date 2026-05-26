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

// FormatQuality maps an upper-case format string to a quality score.
// Higher values indicate better audio quality.  Lossless formats (FLAC, WAV)
// score higher than lossy formats (OGG, MP3).  Formats not listed score 0.
//
// These scores are used by the format-preference filter to keep only the
// highest-quality version when the same track exists in multiple formats.
var FormatQuality = map[string]int{
	"FLAC": 40,
	"WAV":  30,
	"WAVE": 30,
	"OGG":  20,
	"OGA":  20,
	"MP3":  10,
}

// QualityOf returns the quality score for a format string.
// Unrecognised formats return 0.
func QualityOf(format string) int {
	return FormatQuality[strings.ToUpper(format)]
}
