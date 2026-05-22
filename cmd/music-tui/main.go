// Command music-tui is a terminal music player for local MP3 files.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/store"
	"github.com/eilianxiao/music-tui/internal/tui"
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
	defer st.Close()

	// ── Load persisted settings ───────────────────────────────────────────
	musicDir, _ := st.GetSetting("music_dir")
	if musicDir == "" {
		musicDir = resolveMusicDir()
		// Persist the resolved default so the user can see and edit it.
		_ = st.SetSetting("music_dir", musicDir)
	}

	chip8Opts, _ := st.GetSetting("chip8_options")

	// ── Load tracks from DB (no filesystem scan on startup) ───────────────
	tracks, err := st.AllTracks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load tracks: %v\n", err)
		os.Exit(1)
	}

	// ── Initialise audio player ───────────────────────────────────────────
	player, err := audio.NewPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise audio: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(player, st, musicDir, tracks, chip8Opts)

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
