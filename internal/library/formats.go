package library

import (
	"path/filepath"
	"strings"
)

// SupportedExtensions is the set of audio file extensions that music-tui can
// scan and play.  When adding a new format, you must also register a decoder
// in internal/audio/source.go (the decoders map) — both maps must stay in sync.
var SupportedExtensions = map[string]bool{
	".mp3": true,
}

// IsSupportedAudio reports whether path has a supported audio file extension.
func IsSupportedAudio(path string) bool {
	return SupportedExtensions[strings.ToLower(filepath.Ext(path))]
}
