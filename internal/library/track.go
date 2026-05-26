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
	ID          string
	Title       string
	Artist      string
	AlbumArtist string // TPE2 / ALBUMARTIST tag
	Album       string
	Year        string // TDRC / TYER tag (stored as string, e.g. "2003")
	TrackNumber string // TRCK tag (e.g. "3" or "3/12")
	Genre       string // TCON tag
	Comment     string // COMM tag
	Duration    time.Duration
	// FileFormat is the upper-case audio format string (e.g. "MP3", "FLAC").
	// It is stored in the database (migration v4) so queries can filter by
	// format without parsing file extensions.  Use the Format() method to
	// read this field; it falls back to deriving the value from Path when
	// FileFormat is empty (e.g. for tracks loaded from older DB versions).
	FileFormat string
	// Path is the local filesystem path; only set for SourceLocal tracks.
	Path string
	// URL is the remote stream URL; only set for SourceNetease tracks.
	URL    string
	Source Source
	// ProviderID identifies the TrackProvider that owns this track (e.g. "local",
	// "netease").  It corresponds to the provider_id column in the database
	// (migration version 3) and replaces the legacy Source enumeration for
	// multi-source support.
	ProviderID string
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

// DisplayAlbumArtist returns the album artist for sorting and display.
// Falls back to Artist, then "Unknown Artist".
func (t *Track) DisplayAlbumArtist() string {
	if t.AlbumArtist != "" {
		return t.AlbumArtist
	}
	if t.Artist != "" {
		return t.Artist
	}
	return "Unknown Artist"
}

// Format returns the upper-case audio format for the track (e.g. "MP3", "FLAC").
// It uses the stored FileFormat field when available, and falls back to
// deriving the value from the file extension in Path.  This ensures correct
// results for tracks loaded from older database versions that predate the
// migration that added the format column.
func (t *Track) Format() string {
	if t.FileFormat != "" {
		return t.FileFormat
	}
	ext := strings.TrimPrefix(filepath.Ext(t.Path), ".")
	return strings.ToUpper(ext)
}

// MatchesQuery reports whether t matches a case-insensitive substring query
// against Title, Artist, AlbumArtist, and Album fields.
func MatchesQuery(t Track, query string) bool {
	q := strings.ToLower(query)
	return strings.Contains(strings.ToLower(t.DisplayTitle()), q) ||
		strings.Contains(strings.ToLower(t.DisplayArtist()), q) ||
		strings.Contains(strings.ToLower(t.AlbumArtist), q) ||
		strings.Contains(strings.ToLower(t.Album), q)
}
