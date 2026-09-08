package input

import (
	"testing"
	"time"
)

func TestActivitySignalsDrainCounts(t *testing.T) {
	ch := make(chan Signal, 8)
	ch <- KeyActivity
	ch <- MouseActivity
	ch <- KeyActivity

	var s ActivitySignals
	got := s.Drain(ch)

	if got != 3 {
		t.Fatalf("Drain() consumed %d signals, want 3", got)
	}
	if s.KeyCount != 2 {
		t.Errorf("KeyCount = %d, want 2", s.KeyCount)
	}
	if s.MouseCount != 1 {
		t.Errorf("MouseCount = %d, want 1", s.MouseCount)
	}
	if s.LastActivity.IsZero() {
		t.Error("LastActivity should be set after draining")
	}
	if !s.LastActivity.Before(time.Now().Add(time.Second)) {
		t.Errorf("LastActivity %v looks wrong", s.LastActivity)
	}
}

func TestActivitySignalsDrainEmpty(t *testing.T) {
	var s ActivitySignals
	if got := s.Drain(make(chan Signal, 4)); got != 0 {
		t.Fatalf("Drain() on empty channel consumed %d, want 0", got)
	}
	if s.KeyCount != 0 || s.MouseCount != 0 || !s.LastActivity.IsZero() {
		t.Errorf("empty drain mutated state: %+v", s)
	}
}

func TestActivitySignalsDrainClosedChannel(t *testing.T) {
	ch := make(chan Signal)
	close(ch)
	var s ActivitySignals
	if got := s.Drain(ch); got != 0 {
		t.Fatalf("Drain() on closed channel consumed %d, want 0", got)
	}
}

func TestNoneBackendChannelIsInert(t *testing.T) {
	// The None backend returns an already-closed channel: reading it never
	// blocks and never yields a real signal, so Drain consumes nothing.
	n := NewNone()
	var s ActivitySignals
	if got := s.Drain(n.Signals()); got != 0 {
		t.Fatalf("Drain() from None backend consumed %d, want 0", got)
	}
}

func TestSignalString(t *testing.T) {
	if got := KeyActivity.String(); got != "key" {
		t.Errorf("KeyActivity.String() = %q, want %q", got, "key")
	}
	if got := MouseActivity.String(); got != "mouse" {
		t.Errorf("MouseActivity.String() = %q, want %q", got, "mouse")
	}
}
