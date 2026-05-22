package audio

import "github.com/gopxl/beep/v2"

// fadeProcessor is a beep.Streamer that linearly ramps the amplitude of each
// sample from fromVol to toVol over totalSamples output frames.
//
// When the ramp completes (currentSample >= totalSamples) the volume is held
// at toVol for any remaining samples from the wrapped Streamer.
//
// If done is non-nil it is called exactly once after the last ramped frame has
// been written to the output buffer.  It is called from within Stream(), which
// runs on the beep mixer goroutine — keep it non-blocking.
type fadeProcessor struct {
	Streamer      beep.Streamer
	fromVol       float64
	toVol         float64
	totalSamples  int
	currentSample int
	done          func()
	doneCalled    bool
}

// newFade returns a fadeProcessor that transitions from fromVol to toVol over
// durationSamples audio frames.
func newFade(s beep.Streamer, fromVol, toVol float64, durationSamples int, done func()) *fadeProcessor {
	if durationSamples < 1 {
		durationSamples = 1
	}
	return &fadeProcessor{
		Streamer:     s,
		fromVol:      fromVol,
		toVol:        toVol,
		totalSamples: durationSamples,
		done:         done,
	}
}

// Stream implements beep.Streamer.
func (f *fadeProcessor) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = f.Streamer.Stream(samples)
	if n == 0 {
		return
	}

	rampDelta := f.toVol - f.fromVol

	for i := range samples[:n] {
		var vol float64
		if f.currentSample < f.totalSamples {
			// Linear ramp.
			t := float64(f.currentSample) / float64(f.totalSamples)
			vol = f.fromVol + rampDelta*t
			f.currentSample++
		} else {
			vol = f.toVol
		}
		samples[i][0] *= vol
		samples[i][1] *= vol
	}

	// Fire done callback once after the ramp finishes.
	if !f.doneCalled && f.currentSample >= f.totalSamples && f.done != nil {
		f.doneCalled = true
		f.done()
	}

	return
}

// Err implements beep.Streamer.
func (f *fadeProcessor) Err() error { return f.Streamer.Err() }
