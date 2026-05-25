package library

import (
	"path/filepath"
	"strings"
)

// SupportedExtensions is the set of audio file extensions that music-tui can play.
// To add a new format, insert its lower-case extension (with leading dot) here.
var SupportedExtensions = map[string]bool{
	".mp3": true,
}

// IsSupportedAudio reports whether path has a supported audio file extension.
func IsSupportedAudio(path string) bool {
	return SupportedExtensions[strings.ToLower(filepath.Ext(path))]
}
