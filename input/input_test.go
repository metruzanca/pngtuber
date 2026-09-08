package input

import (
	"testing"
	"time"
)

func TestActivitySignalsDrainCounts(t *testing.T) {
	ch := make(chan Event, 8)
	ch <- Event{Signal: KeyActivity}
	ch <- Event{Signal: MouseActivity, DX: 3, DY: -2}
	ch <- Event{Signal: KeyActivity}

	var s ActivitySignals
	got := s.Drain(ch)

	if got != 3 {
		t.Fatalf("Drain() consumed %d events, want 3", got)
	}
	if s.KeyCount != 2 {
		t.Errorf("KeyCount = %d, want 2", s.KeyCount)
	}
	if s.MouseCount != 1 {
		t.Errorf("MouseCount = %d, want 1", s.MouseCount)
	}
	if s.MouseDX != 3 || s.MouseDY != -2 {
		t.Errorf("MouseDX/DY = %d/%d, want 3/-2", s.MouseDX, s.MouseDY)
	}
	if s.LastActivity.IsZero() {
		t.Error("LastActivity should be set after draining")
	}
	if !s.LastActivity.Before(time.Now().Add(time.Second)) {
		t.Errorf("LastActivity %v looks wrong", s.LastActivity)
	}
}

func TestActivitySignalsDrainAccumulatesMotion(t *testing.T) {
	ch := make(chan Event, 4)
	ch <- Event{Signal: MouseActivity, DX: 5, DY: 1}
	ch <- Event{Signal: MouseActivity, DX: -2, DY: 4}
	ch <- Event{Signal: KeyActivity, DX: 99, DY: 99} // key events carry no motion

	var s ActivitySignals
	s.Drain(ch)
	if s.MouseDX != 3 || s.MouseDY != 5 {
		t.Errorf("accumulated MouseDX/DY = %d/%d, want 3/5", s.MouseDX, s.MouseDY)
	}
}

func TestActivitySignalsDrainEmpty(t *testing.T) {
	var s ActivitySignals
	if got := s.Drain(make(chan Event, 4)); got != 0 {
		t.Fatalf("Drain() on empty channel consumed %d, want 0", got)
	}
	if s.KeyCount != 0 || s.MouseCount != 0 || !s.LastActivity.IsZero() {
		t.Errorf("empty drain mutated state: %+v", s)
	}
}

func TestActivitySignalsDrainClosedChannel(t *testing.T) {
	ch := make(chan Event)
	close(ch)
	var s ActivitySignals
	if got := s.Drain(ch); got != 0 {
		t.Fatalf("Drain() on closed channel consumed %d, want 0", got)
	}
}

func TestNoneBackendChannelIsInert(t *testing.T) {
	// The None backend returns an already-closed channel: reading it never
	// blocks and never yields a real event, so Drain consumes nothing.
	n := NewNone()
	var s ActivitySignals
	if got := s.Drain(n.Events()); got != 0 {
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
