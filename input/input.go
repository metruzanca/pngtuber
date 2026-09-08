// Package input provides the observe-only global input pipeline: a backend
// abstraction over platform-specific monitors (Linux: evdev; macOS/Windows
// later) plus the coarse activity counters the game loop drains each tick.
package input

import (
	"time"
)

// Signal is a coarse input activity category. Backends classify raw device
// events into these; the activity state machine consumes the counters.
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

// Event is a coarse activity signal plus optional relative pointer motion.
// The motion deltas (DX/DY) are only set for mouse motion events and let the
// character track the real mouse even when the app window is unfocused (the
// events come from the global backend, not the window).
type Event struct {
	Signal Signal
	DX, DY int32
}

// Backend monitors global input while other apps are focused. Implementations
// must never block or panic the game loop: they push Events into a bounded
// channel and drop events when it is full.
type Backend interface {
	// Events returns the channel of coarse activity events.
	Events() <-chan Event
	// Close stops the backend's monitoring goroutines.
	Close()
}

// ActivitySignals accumulates per-tick signal counts, relative mouse motion,
// and the timestamp of the last observed activity. The game loop drains the
// backend channel into it each tick.
type ActivitySignals struct {
	KeyCount     int
	MouseCount   int
	MouseDX      int // accumulated relative X motion since drain started
	MouseDY      int // accumulated relative Y motion
	LastActivity time.Time
}

// Drain consumes all Events currently buffered in ch without blocking. It
// returns the number of events consumed and updates the counters,
// MouseDX/MouseDY and LastActivity. A closed channel is handled gracefully.
func (s *ActivitySignals) Drain(ch <-chan Event) int {
	n := 0
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return n
			}
			n++
			switch ev.Signal {
			case KeyActivity:
				s.KeyCount++
			case MouseActivity:
				s.MouseCount++
				s.MouseDX += int(ev.DX)
				s.MouseDY += int(ev.DY)
			}
			s.LastActivity = time.Now()
		default:
			return n
		}
	}
}

// None is a no-op backend: it never emits events and is used on platforms
// without a monitor yet, or for graceful degradation when input devices
// can't be opened (e.g. missing /dev/input permissions).
type None struct{}

// NewNone returns a no-op backend.
func NewNone() *None { return &None{} }

func (n *None) Events() <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

func (n *None) Close() {}
