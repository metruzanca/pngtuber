package app

import (
	"testing"
	"time"
)

const (
	ms = time.Millisecond
	s  = time.Second
)

func testActivityConfig() ActivityConfig {
	return ActivityConfig{
		IdleAfterSecs:    5,
		SleepAfterSecs:   10,
		GamingWindowSecs: 1,
	}
}

func TestActivityStartsIdle(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	if a.State() != Idle {
		t.Fatalf("initial state = %s, want idle", a.State())
	}
	// No activity shortly after start: stays idle.
	if got := a.Tick(t0.Add(2*s), 0, 0, false); got != Idle {
		t.Fatalf("state = %s, want idle", got)
	}
}

func TestActivityTyping(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	if got := a.Tick(t0, 3, 0, false); got != Typing {
		t.Fatalf("state = %s, want typing", got)
	}
}

func TestActivityMouse(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	if got := a.Tick(t0, 0, 5, false); got != Mouse {
		t.Fatalf("state = %s, want mouse", got)
	}
}

func TestActivityGaming(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	if got := a.Tick(t0, 1, 1, false); got != Gaming {
		t.Fatalf("simultaneous key+mouse = %s, want gaming", got)
	}
}

func TestActivityGamingWindow(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	// Key, then mouse 0.5s later: both inside the 1s window -> gaming.
	a.Tick(t0, 1, 0, false)
	if got := a.Tick(t0.Add(500*ms), 0, 1, false); got != Gaming {
		t.Fatalf("key+mouse within window = %s, want gaming", got)
	}
	// Mouse 1.5s after the key: key is outside the window -> mouse only.
	if got := a.Tick(t0.Add(1500*ms), 0, 1, false); got != Mouse {
		t.Fatalf("key outside window = %s, want mouse", got)
	}
}

func TestActivityTalkingPriority(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	// Talking wins even with simultaneous typing.
	if got := a.Tick(t0, 3, 0, true); got != Talking {
		t.Fatalf("state = %s, want talking", got)
	}
	// Talking wins even from a gaming state.
	if got := a.Tick(t0.Add(500*ms), 1, 1, true); got != Talking {
		t.Fatalf("state = %s, want talking", got)
	}
}

func TestActivityIdleThenSleep(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	a.Tick(t0, 1, 0, false) // typing at t0

	// Inside the idle window with no new events: holds the typing pose.
	if got := a.Tick(t0.Add(3*s), 0, 0, false); got != Typing {
		t.Fatalf("state = %s, want typing (held)", got)
	}
	// Past idle_after (5s) -> idle.
	if got := a.Tick(t0.Add(6*s), 0, 0, false); got != Idle {
		t.Fatalf("state = %s, want idle", got)
	}
	// Past sleep_after (10s) -> sleep.
	if got := a.Tick(t0.Add(11*s), 0, 0, false); got != Sleep {
		t.Fatalf("state = %s, want sleep", got)
	}
}

func TestActivityActivityWakesFromSleep(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	a.Tick(t0.Add(11*s), 0, 0, false) // sleep
	if a.State() != Sleep {
		t.Fatal("expected sleep before wake")
	}
	if got := a.Tick(t0.Add(12*s), 2, 0, false); got != Typing {
		t.Fatalf("state = %s, want typing (woke from sleep)", got)
	}
}

func TestActivityTalkingResetsIdleTimer(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)
	// Continuous talking for 20s keeps the avatar awake (not idle/sleep).
	a.Tick(t0, 0, 0, true)
	if got := a.Tick(t0.Add(20*s), 0, 0, true); got != Talking {
		t.Fatalf("state = %s, want talking", got)
	}
	// Talking stops at t0+20s; 6s later -> idle (timer started when talking stopped).
	if got := a.Tick(t0.Add(26*s), 0, 0, false); got != Idle {
		t.Fatalf("state = %s, want idle", got)
	}
}

func TestActivityStateChangedEvents(t *testing.T) {
	t0 := time.Unix(0, 0)
	a := NewActivity(testActivityConfig(), t0)

	var events []ActivityState
	a.SetOnChange(func(s ActivityState) { events = append(events, s) })

	a.Tick(t0, 1, 0, false)             // idle -> typing
	a.Tick(t0.Add(500*ms), 0, 0, false) // hold typing (no event)
	a.Tick(t0.Add(6*s), 0, 0, false)    // typing -> idle
	a.Tick(t0.Add(11*s), 0, 0, false)   // idle -> sleep

	want := []ActivityState{Typing, Idle, Sleep}
	if len(events) != len(want) {
		t.Fatalf("got %d events %v, want %d (%v)", len(events), events, len(want), want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("event[%d] = %s, want %s", i, events[i], want[i])
		}
	}
}

func TestActivityStateString(t *testing.T) {
	want := map[ActivityState]string{
		Sleep: "sleep", Idle: "idle", Mouse: "mouse",
		Typing: "typing", Gaming: "gaming", Talking: "talking",
	}
	for st, name := range want {
		if got := st.String(); got != name {
			t.Errorf("String() = %q, want %q", got, name)
		}
	}
}
