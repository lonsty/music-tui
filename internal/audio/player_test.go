package audio

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lonsty/music-tui/internal/library"
)

// defaultTestMP3 is used when MUSIC_TUI_TEST_MP3 is unset.  Tests skip when the
// file is absent, so the suite runs on machines without a personal music
// library (including CI).
const defaultTestMP3 = "/Users/eilianxiao/Music/Music/Media.localized/Music/周杰伦/不能说的秘密 电影原声带/03 早操.mp3"

var (
	playerOnce sync.Once
	sharedPl   *Player
	playerErr  error
)

// testTrack returns a playable fixture or skips the test.
func testTrack(t *testing.T) library.Track {
	t.Helper()
	path := os.Getenv("MUSIC_TUI_TEST_MP3")
	if path == "" {
		path = defaultTestMP3
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("test audio file unavailable (set MUSIC_TUI_TEST_MP3): %v", err)
	}
	return library.Track{ID: path, Path: path}
}

// getTestPlayer initialises the shared player once and skips when no audio
// device is available.
func getTestPlayer(t *testing.T) *Player {
	t.Helper()
	playerOnce.Do(func() {
		sharedPl, playerErr = NewPlayer()
	})
	if playerErr != nil {
		t.Skipf("audio device unavailable: %v", playerErr)
	}
	return sharedPl
}

// TestPlayStop verifies Play→Pause→Resume→Stop without deadlock.
func TestPlayStop(t *testing.T) {
	p := getTestPlayer(t)
	track := testTrack(t)

	if err := p.Play(track); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if p.State() != StatePlaying {
		t.Fatalf("expected StatePlaying, got %v", p.State())
	}

	time.Sleep(200 * time.Millisecond)

	pos := p.Position()
	if pos == 0 {
		t.Error("Position() returned 0 after 200ms of playback")
	}
	t.Logf("position after 200ms: %v", pos)

	p.Pause()
	if p.State() != StatePaused {
		t.Errorf("expected StatePaused, got %v", p.State())
	}

	p.Resume()
	if p.State() != StatePlaying {
		t.Errorf("expected StatePlaying, got %v", p.State())
	}

	// Stop must return promptly.
	stopDone := make(chan struct{})
	go func() { p.Stop(); close(stopDone) }()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s — possible deadlock")
	}

	if p.State() != StateStopped {
		t.Errorf("expected StateStopped after Stop(), got %v", p.State())
	}
}

// TestTogglePause verifies pause/resume cycling.
func TestTogglePause(t *testing.T) {
	p := getTestPlayer(t)
	track := testTrack(t)

	if err := p.Play(track); err != nil {
		t.Fatalf("Play: %v", err)
	}
	defer p.Stop()

	p.TogglePause()
	if p.State() != StatePaused {
		t.Errorf("after first toggle: want StatePaused, got %v", p.State())
	}

	p.TogglePause()
	if p.State() != StatePlaying {
		t.Errorf("after second toggle: want StatePlaying, got %v", p.State())
	}
}
