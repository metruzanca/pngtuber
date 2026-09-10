// Microphone capture for voice-activity detection (Milestone 3).
//
// Connects to the default PulseAudio/PipeWire source with jfreymuth/pulse
// (pure Go, no cgo). The record callback computes per-buffer RMS and
// low-pass smooths it into an atomic fixed-point so the read is lock-free.
// The game loop samples it into a 0..1 level plus an is-talking boolean
// against a configurable threshold with hysteresis.
//
// If no audio server is reachable we log a clear warning and run without a
// mic — never crash.
package app

import (
	"github.com/charmbracelet/log"
	"math"
	"sync/atomic"
	"time"

	"github.com/jfreymuth/pulse"
)

// fixedShift scales the smoothed level into a 16.16 fixed-point uint32 so the
// audio callback can publish it atomically without any locks.
const fixedShift = 16

const (
	// smoothAttack/smoothRelease are the per-buffer low-pass factors applied
	// in the record callback. Attack is fast (level snaps up on speech onset);
	// release is slow (level decays gradually after speech stops).
	smoothAttack  = 0.35
	smoothRelease = 0.05
)

// Mic is the voice-activity detector. It owns the PulseAudio record stream.
// The zero value is not usable; use NewMic.
type Mic struct {
	// levelFixed is the smoothed 0..1 loudness as 16.16 fixed point. Written
	// by the audio callback goroutine, read by the game loop.
	levelFixed atomic.Uint32
	// talking tracks the hysteresis state; touched only by the game loop.
	talking bool

	client *pulse.Client
	stream *pulse.RecordStream
}

// NewMic connects to the default source and starts recording. If the audio
// server is unreachable it logs a clear, actionable warning and returns nil so
// the app keeps running without mic input.
func NewMic() *Mic {
	client, err := pulse.NewClient(pulse.ClientTimeout(2 * time.Second))
	if err != nil {
		log.Warnf("mic: cannot connect to PulseAudio/PipeWire (%v); running without mic. Start pulseaudio/pipewire to enable voice activity", err)
		return nil
	}

	m := &Mic{client: client}
	stream, err := client.NewRecord(pulse.Int16Writer(m.onAudio), pulse.RecordLatency(0.05))
	if err != nil {
		log.Warnf("mic: cannot open record stream (%v); running without mic", err)
		client.Close()
		return nil
	}
	m.stream = stream
	stream.Start()
	log.Infof("mic: recording from default source (rate=%d channels=%d)", stream.SampleRate(), stream.Channels())
	return m
}

// onAudio is called by the pulse library with a buffer of Int16 samples (~50ms
// via RecordLatency). It computes RMS, low-pass smooths it, and publishes the
// level atomically.
func (m *Mic) onAudio(samples []int16) (int, error) {
	if len(samples) == 0 {
		return 0, nil
	}
	rms := rmsLevel(samples)
	prev := fixedToLevel(m.levelFixed.Load())
	var next float64
	if rms > prev {
		next = smoothLevel(prev, rms, smoothAttack)
	} else {
		next = smoothLevel(prev, rms, smoothRelease)
	}
	m.levelFixed.Store(levelToFixed(next))
	return len(samples), nil
}

// Level returns the smoothed loudness in 0..1.
func (m *Mic) Level() float64 {
	if m == nil {
		return 0
	}
	return fixedToLevel(m.levelFixed.Load())
}

// Talking reports whether the voice is above the threshold, applying
// hysteresis: once talking, it stays talking until the level drops below
// threshold*hysteresis (e.g. 0.5 → half the threshold) to avoid chatter.
func (m *Mic) Talking(threshold, hysteresis float64) bool {
	if m == nil {
		return false
	}
	if hysteresis <= 0 || hysteresis > 1 {
		hysteresis = 0.5 // sensible default for older manifests
	}
	lvl := m.Level()
	if m.talking {
		if lvl < threshold*hysteresis {
			m.talking = false
		}
	} else if lvl >= threshold {
		m.talking = true
	}
	return m.talking
}

// Close stops recording and drops the connection. Safe to call on nil.
func (m *Mic) Close() {
	if m == nil {
		return
	}
	if m.stream != nil {
		m.stream.Stop()
		m.stream.Close()
	}
	if m.client != nil {
		m.client.Close()
	}
}

// rmsLevel returns the root-mean-square amplitude of Int16 samples,
// normalized to 0..1.
func rmsLevel(samples []int16) float64 {
	var acc float64
	for _, s := range samples {
		f := float64(s) / 32768.0
		acc += f * f
	}
	return math.Sqrt(acc / float64(len(samples)))
}

// smoothLevel low-pass filters prev toward rms by a factor (0..1). Larger
// factors track faster; smaller factors smooth more.
func smoothLevel(prev, rms, factor float64) float64 {
	return prev + (rms-prev)*factor
}

func levelToFixed(l float64) uint32 {
	if l < 0 {
		l = 0
	}
	if l > 1 {
		l = 1
	}
	return uint32(l * float64(int(1)<<fixedShift))
}

func fixedToLevel(f uint32) float64 {
	return float64(f) / float64(int(1)<<fixedShift)
}
