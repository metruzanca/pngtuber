// Package input provides the observe-only global input pipeline: a backend
// abstraction over platform-specific monitors (Linux: evdev; macOS/Windows
// later) plus the coarse activity counters the game loop drains each tick.
package input

import (
	"time"
)

// Signal is a coarse input activity category. Backends classify raw device
// events into these; the activity state machine (a later milestone) consumes
// the counters.
type Signal int

const (
	// KeyActivity is a physical key press (auto-repeat ignored).
	KeyActivity Signal = iota
	// MouseActivity is pointer motion or a mouse button press.
	MouseActivity
)

func (s Signal) String() string {
	switch s {
	case KeyActivity:
		return "key"
	case MouseActivity:
		return "mouse"
	default:
		return "unknown"
	}
}

// Backend monitors global input while other apps are focused. Implementations
// must never block or panic the game loop: they push coarse Signals into a
// bounded channel and drop events when it is full.
type Backend interface {
	// Signals returns the channel of coarse activity signals.
	Signals() <-chan Signal
	// Close stops the backend's monitoring goroutines.
	Close()
}

// ActivitySignals accumulates per-tick signal counts plus the timestamp of the
// last observed activity. The game loop drains the backend channel into it
// each tick.
type ActivitySignals struct {
	KeyCount     int
	MouseCount   int
	LastActivity time.Time
}

// Drain consumes all Signals currently buffered in ch without blocking. It
// returns the number of signals consumed and updates the counters and
// LastActivity. A closed channel is handled gracefully.
func (s *ActivitySignals) Drain(ch <-chan Signal) int {
	n := 0
	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return n
			}
			n++
			switch sig {
			case KeyActivity:
				s.KeyCount++
			case MouseActivity:
				s.MouseCount++
			}
			s.LastActivity = time.Now()
		default:
			return n
		}
	}
}

// None is a no-op backend: it never emits signals and is used on platforms
// without a monitor yet, or for graceful degradation when input devices
// can't be opened (e.g. missing /dev/input permissions).
type None struct{}

// NewNone returns a no-op backend.
func NewNone() *None { return &None{} }

func (n *None) Signals() <-chan Signal {
	ch := make(chan Signal)
	close(ch)
	return ch
}

func (n *None) Close() {}
