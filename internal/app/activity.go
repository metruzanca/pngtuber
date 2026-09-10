// Activity state machine (Milestone 4).
//
// Merges input signals (key/mouse counts + mic talking) each tick into a single
// ActivityState, in priority order Talking > Gaming > Typing > Mouse > (Idle /
// Sleep via timers). The idle timer counts real time since the last *any*
// activity (keyboard, mouse, or voice); after idle_after_secs the character
// goes Idle, after sleep_after_secs Sleep. Any activity resets the timer and
// wakes from sleep instantly. State transitions are delivered via an optional
// callback so animation code can react cleanly.
package app

import (
	"time"
)

// ActivityState is the character's behavioral state derived from input.
type ActivityState int

const (
	// Sleep is long inactivity.
	Sleep ActivityState = iota
	// Idle is no activity, brief.
	Idle
	// Mouse is mouse-only activity.
	Mouse
	// Typing is keyboard-only activity.
	Typing
	// Gaming is keyboard + mouse within a short window.
	Gaming
	// Talking is mic above threshold.
	Talking
)

func (s ActivityState) String() string {
	switch s {
	case Sleep:
		return "sleep"
	case Idle:
		return "idle"
	case Mouse:
		return "mouse"
	case Typing:
		return "typing"
	case Gaming:
		return "gaming"
	case Talking:
		return "talking"
	default:
		return "unknown"
	}
}

// Activity merges input signals into a state and runs the idle/sleep timers.
type Activity struct {
	cfg ActivityConfig

	lastKey   time.Time
	lastMouse time.Time
	lastAny   time.Time
	state     ActivityState

	onChange func(ActivityState)
}

// NewActivity creates an Activity in the Idle state. now seeds the idle timer
// so a freshly started app doesn't drop straight into Sleep.
func NewActivity(cfg ActivityConfig, now time.Time) *Activity {
	return &Activity{
		cfg:     cfg,
		lastAny: now,
		state:   Idle,
	}
}

// State returns the current state.
func (a *Activity) State() ActivityState { return a.state }

// SetOnChange registers a callback invoked on every state transition.
func (a *Activity) SetOnChange(fn func(ActivityState)) { a.onChange = fn }

// Tick merges this tick's activity signals into a state and returns it.
//
// key and mouse are the counts since the previous tick; micTalking is the
// current voice-activity boolean. The clock is explicit so tests can drive
// the idle/sleep timers deterministically.
func (a *Activity) Tick(now time.Time, key, mouse int, micTalking bool) ActivityState {
	if key > 0 {
		a.lastKey = now
		a.lastAny = now
	}
	if mouse > 0 {
		a.lastMouse = now
		a.lastAny = now
	}
	if micTalking {
		// Voice is activity too: talking keeps the avatar awake.
		a.lastAny = now
	}

	idleFor := now.Sub(a.lastAny)
	window := secsToDuration(a.cfg.GamingWindowSecs)
	keyRecent := !a.lastKey.IsZero() && now.Sub(a.lastKey) <= window
	mouseRecent := !a.lastMouse.IsZero() && now.Sub(a.lastMouse) <= window

	var next ActivityState
	switch {
	case micTalking:
		next = Talking
	case keyRecent && mouseRecent:
		next = Gaming
	case keyRecent:
		next = Typing
	case mouseRecent:
		next = Mouse
	case idleFor >= secsToDuration(a.cfg.SleepAfterSecs):
		next = Sleep
	case idleFor >= secsToDuration(a.cfg.IdleAfterSecs):
		next = Idle
	default:
		// Inside the active window but no fresh event this tick: hold the
		// current pose until the idle timer elapses.
		next = a.state
	}

	if next != a.state {
		a.state = next
		if a.onChange != nil {
			a.onChange(next)
		}
	}
	return a.state
}

// secsToDuration converts a fractional-seconds config value to a duration.
// Non-positive values mean "immediately" (0).
func secsToDuration(s float64) time.Duration {
	if s <= 0 {
		return 0
	}
	return time.Duration(s * float64(time.Second))
}
