// Command music-tui is a terminal music player for local MP3 files.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lonsty/music-tui/internal/audio"
	"github.com/lonsty/music-tui/internal/store"
	"github.com/lonsty/music-tui/internal/tui"
)

func main() {
	// ── Open (or create) the persistent database ──────────────────────────
	dataDir, err := store.DataDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to resolve data dir: %v\n", err)
		os.Exit(1)
	}
	st, err := store.Open(filepath.Join(dataDir, "music-tui.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	// ── Load persisted settings ───────────────────────────────────────────
	musicDir, _ := st.GetSetting(store.KeyMusicDir)
	if musicDir == "" {
		musicDir = resolveMusicDir()
		_ = st.SetSetting(store.KeyMusicDir, musicDir)
	}

	// ── Load tracks from DB ───────────────────────────────────────────────
	tracks, err := st.AllTracks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load tracks: %v\n", err)
		os.Exit(1)
	}

	// ── Restore last session ──────────────────────────────────────────────
	sess := loadSession(st)

	// ── Initialise audio player ───────────────────────────────────────────
	player, err := audio.NewPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise audio: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(player, st, musicDir, tracks, sess)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	app.WithProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running program: %v\n", err)
		os.Exit(1)
	}

	app.Cleanup()
}

// loadSession reads all persisted playback state from the DB.
// Returns nil when there is no previous session to restore.
func loadSession(st *store.Store) *tui.SessionState {
	get := func(key string) string {
		v, _ := st.GetSetting(key)
		return v
	}
	atoi := func(s string, def int) int {
		if s == "" {
			return def
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return def
		}
		return n
	}
	atof := func(s string, def float64) float64 {
		if s == "" {
			return def
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return def
		}
		return f
	}

	lastTrackID := get(store.KeyLastTrackID)
	if lastTrackID == "" {
		// No previous session.
		return &tui.SessionState{
			Volume:       atof(get(store.KeyVolume), 1.0),
			PlayMode:     atoi(get(store.KeyPlayMode), 0),
			RetroIdx:     atoi(get(store.KeyRetroIdx), 0),
			Chip8Options: get(store.KeyChip8Options),
		}
	}

	return &tui.SessionState{
		LastTrackID:    lastTrackID,
		LastPositionMs: int64(atoi(get(store.KeyLastPositionMs), 0)),
		WasPlaying:     get(store.KeyWasPlaying) == store.ValWasPlayingYes,
		Volume:         atof(get(store.KeyVolume), 1.0),
		PlayMode:       atoi(get(store.KeyPlayMode), 0),
		RetroIdx:       atoi(get(store.KeyRetroIdx), 0),
		Chip8Options:   get(store.KeyChip8Options),
	}
}

// resolveMusicDir returns a sensible default music directory.
func resolveMusicDir() string {
	if len(os.Args) >= 2 {
		return os.Args[1]
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	musicPath := filepath.Join(home, "Music")
	if info, err := os.Stat(musicPath); err == nil && info.IsDir() {
		return musicPath
	}
	return "."
}
