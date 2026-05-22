// Package audio provides a thread-safe audio player backed by the beep library.
package audio

import (
	"fmt"
	"math"
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

// retroProcessor is a beep.Streamer that produces a clean "lo-fi pixelated"
// sound effect by combining:
//
//  1. A one-pole IIR low-pass pre-filter whose cutoff is set just below the
//     Nyquist frequency of the target sample rate (targetRate/2).  This removes
//     all frequency content that would alias after downsampling, so the output
//     contains no harsh high-frequency artefacts.
//
//  2. Sample-and-hold at the target rate: every `hold` output frames the filter
//     output is captured; the same value is then repeated for the remaining
//     frames.  This is identical to the behaviour of a classic DAC running at
//     a low sample rate — each "step" plays one constant sample.
//
// No bit-depth quantisation is applied, so the only distortion is the reduced
// time resolution.  The result sounds like the music has been "pixelated" in
// time: clearly recognisable, no noise, but with a characteristic staircase
// quality.
//
// When holdLen ≤ 1 the processor is a transparent pass-through.
type retroProcessor struct {
	Streamer beep.Streamer
	holdLen  int // output frames per hold step; ≤1 = bypass

	// IIR low-pass filter state (one per channel).
	lpL, lpR float64
	alpha    float64 // IIR coefficient; computed from targetRate

	// Sample-and-hold state.
	heldL, heldR float64
	holdPos      int
}

func (rp *retroProcessor) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = rp.Streamer.Stream(samples)
	if n == 0 || rp.holdLen <= 1 {
		return
	}

	α := rp.alpha
	inv := 1 - α

	for i := range samples[:n] {
		l, r := samples[i][0], samples[i][1]

		// Low-pass filter: attenuate frequencies above targetRate/2 to prevent
		// aliasing when the hold step "quantises" time.
		rp.lpL = α*l + inv*rp.lpL
		rp.lpR = α*r + inv*rp.lpR

		// Sample-and-hold: capture a new value at the start of each hold period.
		if rp.holdPos == 0 {
			rp.heldL = rp.lpL
			rp.heldR = rp.lpR
		}
		samples[i][0] = rp.heldL
		samples[i][1] = rp.heldR

		rp.holdPos++
		if rp.holdPos >= rp.holdLen {
			rp.holdPos = 0
		}
	}
	return
}

func (rp *retroProcessor) Err() error { return rp.Streamer.Err() }

// retroParams returns the hold length and IIR alpha for a given preset and
// output sample rate.
//
// Alpha for a one-pole IIR approximation of cutoff fc at sample rate fs:
//
//	α ≈ 1 - exp(-2π × fc / fs)  (bilinear approximation, good for fc ≪ fs)
//
// Cutoff is set to targetRate/2 (Nyquist of the target rate).
func retroParams(outputRate, targetRate int) (holdLen int, alpha float64) {
	if targetRate <= 0 || targetRate >= outputRate {
		return 1, 1.0 // bypass
	}
	holdLen = outputRate / targetRate
	if holdLen < 2 {
		holdLen = 1 // effectively bypass
		return
	}
	fc := float64(targetRate) / 2.0
	fs := float64(outputRate)
	alpha = 1 - math.Exp(-2*math.Pi*fc/fs)
	return
}

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
	retro    *retroProcessor
	ctrl     *beep.Ctrl
	vol      *effects.Volume
	format   beep.Format
	state    State
	retroIdx int // retro effect preset index (0 = off)
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
	retro := &retroProcessor{Streamer: src}
	ctrl := &beep.Ctrl{Streamer: retro, Paused: false}
	vol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: 0, Silent: false}

	// 4. Snapshot onDone and effect preset, then store the new stream fields.
	p.mu.Lock()
	onDone := p.onDone
	retro.holdLen, retro.alpha = retroParams(int(defaultSampleRate), retroTargetRate(p.retroIdx))
	p.streamer = streamer
	p.retro = retro
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
	p.retro = nil
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

// retroPresets defines all retro effect presets in order.
// Index 0 is always "off" (bypass).  Remaining entries are target virtual
// sample rates in Hz, from highest (least degraded) to lowest (most degraded).
var retroPresets = []int{
	0,     // 0: off
	11025, // 1: ≈ NES APU rate
	5513,  // 2: coarser
	2756,  // 3: lo-fi telephone
	1378,  // 4: very coarse
	689,   // 5: extreme
	344,   // 6: barely intelligible
}

// RetroPresetCount is the total number of retro presets (including "off").
var RetroPresetCount = len(retroPresets)

// retroTargetRate maps a preset index to the target virtual sample rate.
// Returns 0 (bypass) for out-of-range indices.
func retroTargetRate(idx int) int {
	if idx < 0 || idx >= len(retroPresets) {
		return 0
	}
	return retroPresets[idx]
}

// RetroPresetRate returns the target virtual sample rate (Hz) for preset index idx.
// Returns 0 for off (idx=0) or out-of-range values.
func RetroPresetRate(idx int) int {
	return retroTargetRate(idx)
}

// SetRetroPreset switches the retro effect preset while playing.
// idx=0 disables the effect; valid range: [0, RetroPresetCount).
// Safe to call while a track is playing — takes effect on the next audio chunk.
func (p *Player) SetRetroPreset(idx int) {
	if idx < 0 || idx >= RetroPresetCount {
		idx = 0
	}
	p.mu.Lock()
	p.retroIdx = idx
	retro := p.retro
	p.mu.Unlock()

	if retro == nil {
		return
	}

	holdLen, alpha := retroParams(int(defaultSampleRate), retroTargetRate(idx))

	speaker.Lock()
	retro.holdLen = holdLen
	retro.alpha = alpha
	// Reset state to avoid a transient pop when switching presets.
	retro.holdPos = 0
	retro.lpL, retro.lpR = 0, 0
	retro.heldL, retro.heldR = 0, 0
	speaker.Unlock()
}

// CrossfadeTo fades out the current track, then starts newPath from
// positionOffset with a fade-in.  The call blocks until both the fade-out and
// the fade-in are fully complete, making it safe to call from a tea.Cmd
// goroutine.
//
// The fade duration is fixed at 1200 ms (smooth and audible).  If the player
// is stopped or paused when called, the crossfade still proceeds (it opens and
// seeks newPath).
func (p *Player) CrossfadeTo(newPath string, positionOffset time.Duration) error {
	const fadeDuration = 1200 * time.Millisecond
	const steps = 40 // number of volume steps (each ≈ 30 ms)
	stepSleep := fadeDuration / steps

	// ── Fade out current track ─────────────────────────────────────────────
	// Walk vol.Volume from its current dB value down to –10 (virtually silent).
	p.mu.Lock()
	vol := p.vol
	p.mu.Unlock()

	if vol != nil {
		speaker.Lock()
		startDB := vol.Volume
		speaker.Unlock()

		targetDB := -10.0 // beep's Volume is in dB (base-2 log scale)
		for i := 1; i <= steps; i++ {
			t := float64(i) / float64(steps)
			db := startDB + (targetDB-startDB)*t
			speaker.Lock()
			vol.Volume = db
			speaker.Unlock()
			time.Sleep(stepSleep)
		}
	}

	// ── Snapshot position before tearing down ────────────────────────────
	// positionOffset was captured by the caller right before the call, so we
	// use that value plus the time spent fading out.
	seekTo := positionOffset + fadeDuration

	// ── Stop current stream ───────────────────────────────────────────────
	old := p.stopCurrent()
	if old != nil {
		old.Close()
	}

	// ── Open and start new stream ─────────────────────────────────────────
	f, err := os.Open(newPath)
	if err != nil {
		return fmt.Errorf("crossfade open %q: %w", newPath, err)
	}
	streamer, format, err := mp3.Decode(f)
	if err != nil {
		f.Close()
		return fmt.Errorf("crossfade decode %q: %w", newPath, err)
	}

	// Seek to the target position (ignore errors — the file may be shorter).
	targetSample := format.SampleRate.N(seekTo)
	if targetSample > 0 && targetSample < streamer.Len() {
		_ = streamer.Seek(targetSample)
	}

	var src beep.Streamer = streamer
	if format.SampleRate != defaultSampleRate {
		src = beep.Resample(4, format.SampleRate, defaultSampleRate, streamer)
	}
	retro := &retroProcessor{Streamer: src}
	ctrl := &beep.Ctrl{Streamer: retro, Paused: false}
	newVol := &effects.Volume{Streamer: ctrl, Base: 2, Volume: -10.0, Silent: false}

	p.mu.Lock()
	onDone := p.onDone
	retro.holdLen, retro.alpha = retroParams(int(defaultSampleRate), retroTargetRate(p.retroIdx))
	p.streamer = streamer
	p.retro = retro
	p.ctrl = ctrl
	p.vol = newVol
	p.format = format
	p.state = StatePlaying
	targetVolDB := 0.0 // unity gain in beep's log scale
	p.mu.Unlock()

	speaker.Play(beep.Seq(newVol, beep.Callback(func() {
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

	// ── Fade in ────────────────────────────────────────────────────────────
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		db := -10.0 + (targetVolDB-(-10.0))*t
		speaker.Lock()
		newVol.Volume = db
		speaker.Unlock()
		time.Sleep(stepSleep)
	}
	// Ensure we land exactly at unity gain.
	speaker.Lock()
	newVol.Volume = targetVolDB
	speaker.Unlock()

	return nil
}

// State returns the current playback state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}
