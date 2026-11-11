package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSupportedAudio(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"song.mp3", true},
		{"song.MP3", true},
		{"song.flac", true},
		{"song.wav", true},
		{"song.wave", true},
		{"song.ogg", true},
		{"song.oga", true},
		{"song.m4a", false},
		{"song.txt", false},
		{"noext", false},
	}
	for _, tt := range tests {
		if got := IsSupportedAudio(tt.path); got != tt.want {
			t.Errorf("IsSupportedAudio(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestQualityOf(t *testing.T) {
	tests := []struct {
		format string
		want   int
	}{
		{"FLAC", 40},
		{"flac", 40},
		{"WAV", 30},
		{"OGG", 20},
		{"MP3", 10},
		{"m4a", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := QualityOf(tt.format); got != tt.want {
			t.Errorf("QualityOf(%q) = %d, want %d", tt.format, got, tt.want)
		}
	}
	// Lossless formats must outrank lossy ones: the format-preference filter
	// relies on this ordering.
	if QualityOf("FLAC") <= QualityOf("MP3") {
		t.Fatal("FLAC must score above MP3")
	}
}

func TestTrackDisplayFallbacks(t *testing.T) {
	full := &Track{
		Title:       "Title",
		Artist:      "Artist",
		AlbumArtist: "AlbumArtist",
		Path:        "/music/song.flac",
		FileFormat:  "FLAC",
	}
	if got := full.DisplayTitle(); got != "Title" {
		t.Errorf("DisplayTitle = %q, want Title", got)
	}
	if got := full.DisplayArtist(); got != "Artist" {
		t.Errorf("DisplayArtist = %q, want Artist", got)
	}
	if got := full.DisplayAlbumArtist(); got != "AlbumArtist" {
		t.Errorf("DisplayAlbumArtist = %q, want AlbumArtist", got)
	}
	if got := full.Format(); got != "FLAC" {
		t.Errorf("Format = %q, want FLAC", got)
	}

	bare := &Track{Path: "/music/No Tags/07 - Untitled.mp3"}
	if got := bare.DisplayTitle(); got != "07 - Untitled" {
		t.Errorf("DisplayTitle fallback = %q, want filename without extension", got)
	}
	if got := bare.DisplayArtist(); got != "Unknown Artist" {
		t.Errorf("DisplayArtist fallback = %q, want Unknown Artist", got)
	}
	if got := bare.DisplayAlbumArtist(); got != "Unknown Artist" {
		t.Errorf("DisplayAlbumArtist fallback = %q, want Unknown Artist", got)
	}
	if got := bare.Format(); got != "MP3" {
		t.Errorf("Format fallback = %q, want MP3", got)
	}

	// AlbumArtist falls back to Artist before the unknown sentinel.
	partial := &Track{Artist: "Artist", Path: "/music/x.flac"}
	if got := partial.DisplayAlbumArtist(); got != "Artist" {
		t.Errorf("DisplayAlbumArtist fallback = %q, want Artist", got)
	}
}

func TestSortByArtistAlbum(t *testing.T) {
	tracks := []Track{
		{ID: "beta", Artist: "Beta", Year: "2001", Album: "b", TrackNumber: "2"},
		{ID: "alpha10", AlbumArtist: "alpha", Year: "2000", Album: "a", TrackNumber: "10"},
		{ID: "alpha2", AlbumArtist: "alpha", Year: "2000", Album: "a", TrackNumber: "2"},
		{ID: "zeta", AlbumArtist: "Zeta", Album: "z", TrackNumber: "1"},
		{ID: "alpha1999", AlbumArtist: "alpha", Year: "1999", Album: "a", TrackNumber: "3/12"},
	}
	SortByArtistAlbum(tracks)

	want := []string{"alpha1999", "alpha2", "alpha10", "beta", "zeta"}
	for i, id := range want {
		if tracks[i].ID != id {
			got := make([]string, len(tracks))
			for j := range tracks {
				got[j] = tracks[j].ID
			}
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestNormaliseYear(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"2003", 2003},
		{" 2003 ", 2003},
		{"2003-01-15", 2003},
		{"", 0},
		{"unknown", 0},
	}
	for _, tt := range tests {
		if got := normaliseYear(tt.in); got != tt.want {
			t.Errorf("normaliseYear(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestTrackNum(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"3", 3},
		{"3/12", 3},
		{" 7 / 9 ", 7},
		{"", 0},
		{"A1", 0},
	}
	for _, tt := range tests {
		if got := trackNum(tt.in); got != tt.want {
			t.Errorf("trackNum(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

// TestParseTrackWithCoverGracefulDegradation covers files whose tags and audio
// frames are both unreadable: the track must still be returned with path,
// format and a stable ID rather than failing the whole scan.
func TestParseTrackWithCoverGracefulDegradation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Untagged.mp3")
	if err := os.WriteFile(path, []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	coverDir := filepath.Join(dir, "covers")
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		t.Fatal(err)
	}

	track, err := ParseTrackWithCover(path, coverDir)
	if err != nil {
		t.Fatalf("ParseTrackWithCover: %v", err)
	}
	if track.Path != path {
		t.Errorf("Path = %q, want %q", track.Path, path)
	}
	if track.FileFormat != "MP3" {
		t.Errorf("FileFormat = %q, want MP3", track.FileFormat)
	}
	if track.ID == "" {
		t.Error("ID must be derived from the path")
	}
	if track.CoverPath != "" {
		t.Errorf("CoverPath = %q, want empty when no cover exists", track.CoverPath)
	}

	// The ID is stable across parses of the same path.
	again, err := ParseTrackWithCover(path, "")
	if err != nil {
		t.Fatalf("second ParseTrackWithCover: %v", err)
	}
	if again.ID != track.ID {
		t.Errorf("ID changed between parses: %q vs %q", track.ID, again.ID)
	}
}
