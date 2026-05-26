package audio

import (
	"testing"
	"time"

	"github.com/lonsty/music-tui/internal/library"
)

const testMP3 = "/Users/eilianxiao/Music/Music/Media.localized/Music/周杰伦/不能说的秘密 电影原声带/03 早操.mp3"

// newTestPlayer creates a Player for testing.
// speaker.Init is called once per test binary via TestMain would be ideal,
// but for simplicity we create the player once per test file with sync.Once
// semantics by relying on a package-level variable.
var testPlayer *Player

func init() {
	var err error
	testPlayer, err = NewPlayer()
	if err != nil {
		panic("NewPlayer: " + err.Error())
	}
}

// TestPlayStop verifies Play→Pause→Resume→Stop without deadlock.
func TestPlayStop(t *testing.T) {
	p := testPlayer
	track := library.Track{ID: testMP3, Path: testMP3}

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
	p := testPlayer
	track := library.Track{ID: testMP3, Path: testMP3}

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
