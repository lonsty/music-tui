// Package audio provides a thread-safe audio player backed by the beep library.
package audio

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"

	"github.com/eilianxiao/music-tui/internal/library"
)

const (
	defaultSampleRate = beep.SampleRate(44100)
	bufferSize        = 4410 // ≈ 100 ms at 44100 Hz
)

// State represents the current playback state.
type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

// Player is a goroutine-safe audio player.
//
// # Lock discipline
//
// beep exposes speaker.Lock/Unlock to protect direct manipulation of streamer
// objects *while the mixer is running*.  The rules are:
//
//   - speaker.Play() and speaker.Clear() manage their own internal locking;
//     call them WITHOUT holding speaker.Lock.
//   - To mutate fields of a streamer that is already queued (e.g. ctrl.Paused,
//     vol.Volume), hold speaker.Lock for the duration.
//   - p.mu protects pure-Go state: state, onDone, streamer pointer, format.
//     Never hold p.mu while calling any speaker.* function.
type Player struct {
	mu       sync.Mutex
	streamer beep.StreamSeekCloser
	ctrl     *beep.Ctrl
	vol      *effects.Volume
	format   beep.Format
	state    State
	onDone   func()
}

// NewPlayer initialises the global beep speaker and returns a ready Player.
// Call exactly once per process.
func NewPlayer() (*Player, error) {
	if err := speaker.Init(defaultSampleRate, bufferSize); err != nil {
		return nil, fmt.Errorf("init speaker: %w", err)
	}
	return &Player{state: StateStopped}, nil
}

// SetOnDone registers a callback fired (in the beep mixer goroutine) when the
// current track finishes naturally.  Must be non-blocking.
func (p *Player) SetOnDone(fn func()) {
	p.mu.Lock()
	p.onDone = fn
	p.mu.Unlock()
}

// Play stops any current playback, then loads and starts the given track.
func (p *Player) Play(track library.Track) error {
	// 1. Stop current audio.  stopCurrent returns the old streamer so we can
	//    close it *after* installing the new one (see step 4).
	old := p.stopCurrent()

	// 2. Open and decode the file.
	f, err := os.Open(track.Path)
	if err != nil {
		if old != nil {
			old.Close()
		}
		return fmt.Errorf("open %q: %w", track.Path, err)
	}
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		f.Close()
		if old != nil {
			old.Close()
		}
		return fmt.Errorf("decode mp3 %q: %w", track.Path, err)
	}

	// 3. Resample if the file's rate differs from the speaker's.
	var src beep.Streamer = streamer
	if format.SampleRate != defaultSampleRate {
		src = beep.Resample(4, format.SampleRate, defaultSampleRate, streamer)
	}
	ctrl := &beep.Ctrl{Streamer: src, Paused: false}
	vol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: 0, Silent: false}

	// 4. Snapshot onDone, then store the new stream fields.
	p.mu.Lock()
	onDone := p.onDone
	p.streamer = streamer
	p.ctrl = ctrl
	p.vol = vol
	p.format = format
	p.state = StatePlaying
	p.mu.Unlock()

	// 5. Hand the stream to the speaker.  speaker.Play is self-locking;
	//    do NOT wrap it in speaker.Lock.
	speaker.Play(beep.Seq(vol, beep.Callback(func() {
		p.mu.Lock()
		wasPlaying := p.state == StatePlaying
		if wasPlaying {
			p.state = StateStopped
		}
		p.mu.Unlock()
		if wasPlaying && onDone != nil {
			onDone()
		}
	})))

	// 6. Close the previous streamer now that the speaker is no longer using it.
	if old != nil {
		old.Close()
	}
	return nil
}

// stopCurrent halts any active playback and returns the old streamer
// (caller must Close it).  Returns nil when already stopped.
func (p *Player) stopCurrent() beep.StreamSeekCloser {
	p.mu.Lock()
	if p.state == StateStopped {
		p.mu.Unlock()
		return nil
	}
	// Mark stopped so the Callback skips onDone.
	old := p.streamer
	p.streamer = nil
	p.ctrl = nil
	p.vol = nil
	p.format = beep.Format{}
	p.state = StateStopped
	p.mu.Unlock()

	// speaker.Clear is self-locking; do NOT hold p.mu here.
	speaker.Clear()
	return old
}

// Stop halts playback and frees resources.
func (p *Player) Stop() {
	if old := p.stopCurrent(); old != nil {
		old.Close()
	}
}

// Pause suspends playback. No-op when not playing.
func (p *Player) Pause() {
	p.mu.Lock()
	if p.state != StatePlaying {
		p.mu.Unlock()
		return
	}
	ctrl := p.ctrl
	p.state = StatePaused
	p.mu.Unlock()

	if ctrl != nil {
		// ctrl.Paused is a beep field; mutate under speaker.Lock.
		speaker.Lock()
		ctrl.Paused = true
		speaker.Unlock()
	}
}

// Resume continues a paused track. No-op when not paused.
func (p *Player) Resume() {
	p.mu.Lock()
	if p.state != StatePaused {
		p.mu.Unlock()
		return
	}
	ctrl := p.ctrl
	p.state = StatePlaying
	p.mu.Unlock()

	if ctrl != nil {
		speaker.Lock()
		ctrl.Paused = false
		speaker.Unlock()
	}
}

// TogglePause switches between playing and paused states.
func (p *Player) TogglePause() {
	p.mu.Lock()
	state := p.state
	p.mu.Unlock()

	switch state {
	case StatePlaying:
		p.Pause()
	case StatePaused:
		p.Resume()
	}
}

// Position returns the current playback offset.
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	streamer := p.streamer
	format := p.format
	p.mu.Unlock()

	if streamer == nil {
		return 0
	}
	// Read position under speaker.Lock to avoid racing with the mixer.
	speaker.Lock()
	pos := streamer.Position()
	speaker.Unlock()
	return format.SampleRate.D(pos)
}

// Duration returns the total length of the current track.
func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	streamer := p.streamer
	format := p.format
	p.mu.Unlock()

	if streamer == nil {
		return 0
	}
	speaker.Lock()
	length := streamer.Len()
	speaker.Unlock()
	return format.SampleRate.D(length)
}

// Seek moves the playback head to the given offset.
func (p *Player) Seek(d time.Duration) error {
	p.mu.Lock()
	streamer := p.streamer
	format := p.format
	p.mu.Unlock()

	if streamer == nil {
		return nil
	}
	speaker.Lock()
	err := streamer.Seek(format.SampleRate.N(d))
	speaker.Unlock()
	if err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	return nil
}

// SetVolume sets playback volume. v ∈ [0.0, 2.0]; 1.0 = unity gain.
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	vol := p.vol
	p.mu.Unlock()

	if vol == nil {
		return
	}
	speaker.Lock()
	vol.Volume = (v/2.0)*4.0 - 3.0
	vol.Silent = v == 0
	speaker.Unlock()
}

// State returns the current playback state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}
