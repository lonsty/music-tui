// Package library provides types and utilities for managing music tracks.
package library

import (
	"path/filepath"
	"strings"
	"time"
)

// Source indicates where a track originates from.
type Source int

const (
	// SourceLocal is a track stored on the local filesystem.
	SourceLocal Source = iota
	// SourceNetease is a track from NetEase Cloud Music.
	SourceNetease
)

// Track represents a single music track with metadata.
type Track struct {
	ID       string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	// Path is the local filesystem path; only set for SourceLocal tracks.
	Path string
	// URL is the remote stream URL; only set for SourceNetease tracks.
	URL    string
	Source Source
	// CoverArt holds the raw image bytes (JPEG or PNG) from the ID3 APIC frame.
	// Populated during in-memory scanning; nil when loaded from the database
	// (use CoverPath to lazy-load the cached file instead).
	CoverArt []byte
	// CoverPath is the filesystem path to the cached cover-art file written by
	// the scanner.  Empty when no cover art was found.
	CoverPath string
}

// DisplayTitle returns a human-readable title for the track.
// Falls back to the filename (without extension) if no title metadata is set.
func (t *Track) DisplayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	base := filepath.Base(t.Path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// DisplayArtist returns the artist name, or "Unknown Artist" if empty.
func (t *Track) DisplayArtist() string {
	if t.Artist != "" {
		return t.Artist
	}
	return "Unknown Artist"
}

// Format returns the uppercased file format extension (e.g. "MP3", "FLAC").
func (t *Track) Format() string {
	ext := strings.TrimPrefix(filepath.Ext(t.Path), ".")
	return strings.ToUpper(ext)
}
