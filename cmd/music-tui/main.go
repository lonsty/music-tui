// Command music-tui is a terminal music player for local MP3 files.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/eilianxiao/music-tui/internal/audio"
	"github.com/eilianxiao/music-tui/internal/tui"
)

func main() {
	musicDir := resolveMusicDir()

	player, err := audio.NewPlayer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise audio: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(player, musicDir)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Wire the "track done" callback so it can send messages into the program.
	app.WithProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error running program: %v\n", err)
		os.Exit(1)
	}
}

// resolveMusicDir returns the music directory from CLI args or a sensible default.
func resolveMusicDir() string {
	if len(os.Args) >= 2 {
		return os.Args[1]
	}

	// Try ~/Music first, fall back to current directory.
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
