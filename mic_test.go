package main

import (
	"math"
	"testing"
)

func TestRMSLevel(t *testing.T) {
	silence := make([]int16, 256)
	if got := rmsLevel(silence); got != 0 {
		t.Errorf("rmsLevel(silence) = %v, want 0", got)
	}

	full := make([]int16, 256)
	for i := range full {
		full[i] = 32767
	}
	if got := rmsLevel(full); math.Abs(got-1.0) > 0.01 {
		t.Errorf("rmsLevel(full) = %v, want ~1", got)
	}

	half := make([]int16, 256)
	for i := range half {
		half[i] = 16384
	}
	if got := rmsLevel(half); math.Abs(got-0.5) > 0.02 {
		t.Errorf("rmsLevel(half) = %v, want ~0.5", got)
	}
}

func TestSmoothLevel(t *testing.T) {
	// Rising: tracks toward target quickly.
	if got := smoothLevel(0.0, 1.0, 0.5); got != 0.5 {
		t.Errorf("smoothLevel rise = %v, want 0.5", got)
	}
	// Falling: with a small factor the decay is slow.
	if got := smoothLevel(1.0, 0.0, 0.1); got != 0.9 {
		t.Errorf("smoothLevel fall = %v, want 0.9", got)
	}
	// Converges over repeated steps.
	l := 0.0
	for i := 0; i < 100; i++ {
		l = smoothLevel(l, 1.0, 0.35)
	}
	if l < 0.999 {
		t.Errorf("smoothLevel did not converge, got %v", l)
	}
}

func TestFixedPointRoundTrip(t *testing.T) {
	for _, l := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
		if got := fixedToLevel(levelToFixed(l)); math.Abs(got-l) > 0.001 {
			t.Errorf("fixed round trip %v -> %v", l, got)
		}
	}
	// Clamps out-of-range.
	if got := levelToFixed(-1); got != 0 {
		t.Errorf("levelToFixed(-1) = %d, want 0", got)
	}
	if got := levelToFixed(2); got != levelToFixed(1) {
		t.Errorf("levelToFixed(2) should clamp to max, got %d", got)
	}
}

func TestTalkingHysteresis(t *testing.T) {
	const thresh = 0.1
	const hyst = 0.5

	m := &Mic{}
	m.levelFixed.Store(levelToFixed(0.9))
	if !m.Talking(thresh, hyst) {
		t.Fatal("level above threshold should start talking")
	}
	if !m.Talking(thresh, hyst) {
		t.Fatal("talking should persist once started")
	}

	// Above the release threshold (0.05) -> keeps talking.
	m.levelFixed.Store(levelToFixed(0.06))
	if !m.Talking(thresh, hyst) {
		t.Fatal("level above release threshold should stay talking")
	}
	// Below the release threshold -> stops.
	m.levelFixed.Store(levelToFixed(0.04))
	if m.Talking(thresh, hyst) {
		t.Fatal("level below release threshold should stop talking")
	}
}

func TestTalkingDefaultHysteresis(t *testing.T) {
	m := &Mic{}
	m.levelFixed.Store(levelToFixed(0.5))
	m.talking = true
	// hysteresis=0 is degenerate; the 0.5 default must still release it.
	m.levelFixed.Store(levelToFixed(0.01))
	if m.Talking(0.1, 0) {
		t.Fatal("zero hysteresis should fall back to default and release")
	}
}

func TestMicNilSafe(t *testing.T) {
	var m *Mic
	if m.Level() != 0 {
		t.Error("nil Mic.Level() should be 0")
	}
	if m.Talking(0.1, 0.5) {
		t.Error("nil Mic.Talking() should be false")
	}
	m.Close() // must not panic
}
