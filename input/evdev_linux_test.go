//go:build linux

package input

import (
	"testing"

	"github.com/grafov/evdev"
)

func TestClassifyKeyPress(t *testing.T) {
	got, ok := classify(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1})
	if !ok {
		t.Fatal("key press should count as activity")
	}
	if got != KeyActivity {
		t.Errorf("key press classified as %s, want %s", got, KeyActivity)
	}
}

func TestClassifyIgnoresReleasesAndRepeat(t *testing.T) {
	for _, value := range []int32{0, 2} { // 0 = release, 2 = auto-repeat
		if _, ok := classify(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: value}); ok {
			t.Errorf("key value %d should not count as activity", value)
		}
	}
}

func TestClassifyMouseButton(t *testing.T) {
	got, ok := classify(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.BTN_LEFT, Value: 1})
	if !ok {
		t.Fatal("mouse button press should count as activity")
	}
	if got != MouseActivity {
		t.Errorf("button press classified as %s, want %s", got, MouseActivity)
	}
}

func TestClassifyRelativeMotion(t *testing.T) {
	got, ok := classify(evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_X, Value: -3})
	if !ok {
		t.Fatal("relative motion should count as activity")
	}
	if got != MouseActivity {
		t.Errorf("relative motion classified as %s, want %s", got, MouseActivity)
	}
}

func TestClassifyAbsoluteMotion(t *testing.T) {
	got, ok := classify(evdev.InputEvent{Type: evdev.EV_ABS, Value: 100})
	if !ok {
		t.Fatal("absolute motion should count as activity")
	}
	if got != MouseActivity {
		t.Errorf("absolute motion classified as %s, want %s", got, MouseActivity)
	}
}

func TestClassifyIgnoresSync(t *testing.T) {
	if _, ok := classify(evdev.InputEvent{Type: evdev.EV_SYN}); ok {
		t.Error("EV_SYN should not count as activity")
	}
}

func TestIsMouseButton(t *testing.T) {
	if !isMouseButton(evdev.BTN_LEFT) {
		t.Error("BTN_LEFT should be a mouse button")
	}
	if isMouseButton(evdev.KEY_A) {
		t.Error("KEY_A should not be a mouse button")
	}
}

func TestEventFromCarriesRelativeMotion(t *testing.T) {
	ev, ok := eventFrom(evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_X, Value: 7})
	if !ok || ev.Signal != MouseActivity || ev.DX != 7 {
		t.Errorf("REL_X -> %+v, ok=%v", ev, ok)
	}
	ev, _ = eventFrom(evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_Y, Value: -3})
	if ev.DY != -3 {
		t.Errorf("REL_Y -> DY=%d, want -3", ev.DY)
	}
	// Non-motion events carry zero deltas.
	ev, _ = eventFrom(evdev.InputEvent{Type: evdev.EV_KEY, Code: evdev.KEY_A, Value: 1})
	if ev.Signal != KeyActivity || ev.DX != 0 || ev.DY != 0 {
		t.Errorf("key press -> %+v", ev)
	}
	// Wheel motion counts as mouse activity but carries no X/Y delta.
	ev, _ = eventFrom(evdev.InputEvent{Type: evdev.EV_REL, Code: evdev.REL_WHEEL, Value: 1})
	if ev.Signal != MouseActivity || ev.DX != 0 {
		t.Errorf("wheel -> %+v", ev)
	}
}
