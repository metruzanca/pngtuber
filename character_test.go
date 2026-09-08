package main

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestFrameLoopLooping(t *testing.T) {
	f := frameLoop{fps: 10, nframes: 3}
	f.update(0.05) // 0.5s*10 -> frame 0
	if got := f.index(); got != 0 {
		t.Errorf("frame at 0.05s = %d, want 0", got)
	}
	f.update(0.05) // 0.1s -> frame 1
	if got := f.index(); got != 1 {
		t.Errorf("frame at 0.1s = %d, want 1", got)
	}
	f.update(0.1) // 0.2s -> frame 2
	if got := f.index(); got != 2 {
		t.Errorf("frame at 0.2s = %d, want 2", got)
	}
	f.update(0.1) // 0.3s -> wraps to 0
	if got := f.index(); got != 0 {
		t.Errorf("frame at 0.3s = %d, want 0 (wrap)", got)
	}
}

func TestFrameLoopResetAndSingle(t *testing.T) {
	f := frameLoop{fps: 10, nframes: 3}
	f.update(0.25)
	if f.index() != 2 {
		t.Fatalf("frame = %d, want 2", f.index())
	}
	f.reset()
	if f.index() != 0 {
		t.Fatalf("frame after reset = %d, want 0", f.index())
	}

	one := frameLoop{fps: 10, nframes: 1}
	one.update(100)
	if one.index() != 0 {
		t.Errorf("single-frame loop = %d, want 0", one.index())
	}
}

func TestVariantSelectionHands(t *testing.T) {
	c := NewCharacter(&Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}})
	cases := []struct {
		state ActivityState
		left  int
		right int
	}{
		{Idle, 0, 0},
		{Typing, 1, 0},
		{Mouse, 0, 1},
		{Gaming, 1, 1},
		{Talking, 0, 0},
		{Sleep, 0, 0},
	}
	for _, tc := range cases {
		c.state = tc.state
		if got := c.variantIndex("left"); got != tc.left {
			t.Errorf("%s: left = %d, want %d", tc.state, got, tc.left)
		}
		if got := c.variantIndex("right"); got != tc.right {
			t.Errorf("%s: right = %d, want %d", tc.state, got, tc.right)
		}
	}
}

func TestVariantSelectionEyelid(t *testing.T) {
	c := NewCharacter(&Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}})
	c.state = Sleep
	if got := c.variantIndex("eyelid"); got != 1 {
		t.Fatalf("sleep eyelid = %d, want 1 (closed)", got)
	}

	c.state = Idle
	if got := c.variantIndex("eyelid"); got != 0 {
		t.Fatalf("awake eyelid = %d, want 0 (open)", got)
	}
	c.blinking = true
	if got := c.variantIndex("eyelid"); got != 1 {
		t.Fatalf("blinking eyelid = %d, want 1 (closed)", got)
	}
}

func TestVariantSelectionMouth(t *testing.T) {
	c := NewCharacter(&Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}})
	if got := c.variantIndex("mouth"); got != 0 {
		t.Fatalf("non-talking mouth = %d, want 0 (closed)", got)
	}
	c.state = Talking
	c.mouth.elapsed = 0.2
	if got := c.variantIndex("mouth"); got != 2 {
		t.Fatalf("talking mouth = %d, want 2", got)
	}
}

func TestSetStateResetsAnimation(t *testing.T) {
	c := NewCharacter(&Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}})
	c.state = Talking
	c.mouth.elapsed = 5.0
	c.Update(0.05)
	if c.mouth.index() == 0 {
		t.Fatal("mouth loop should be past frame 0 before reset")
	}
	c.SetState(Idle)
	if c.mouth.elapsed != 0 {
		t.Errorf("mouth elapsed not reset, got %v", c.mouth.elapsed)
	}
	// Switching back to Talking starts the loop from 0 again.
	c.SetState(Talking)
	if got := c.variantIndex("mouth"); got != 0 {
		t.Errorf("mouth after re-enter = %d, want 0", got)
	}
}

func TestBlinkCycle(t *testing.T) {
	c := NewCharacter(&Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}})
	if c.blinking {
		t.Fatal("should start not blinking")
	}
	// Real-world-sized updates (50ms). Well before the 3s interval: no blink.
	const step = 0.05
	for i := 0; i < int(blinkInterval/step)-2; i++ {
		c.Update(step)
	}
	if c.blinking {
		t.Fatal("should not blink before the interval")
	}
	// Cross the interval; the blink must start within a few steps (guards
	// against float rounding at the exact boundary).
	started := false
	for i := 0; i < 4; i++ {
		c.Update(step)
		if c.blinking {
			started = true
			break
		}
	}
	if !started {
		t.Fatal("expected blinking at the interval")
	}
	// Mid-blink (~0.1s into a 0.15s blink).
	c.Update(step)
	if !c.blinking {
		t.Fatal("still blinking mid-blink")
	}
	// Past blinkDuration the blink ends.
	for i := 0; i < 4; i++ {
		c.Update(step)
	}
	if c.blinking {
		t.Fatal("blink should have ended after blinkDuration")
	}
}

func TestLookOffset(t *testing.T) {
	const w, h = 192, 192
	// Cursor dead center -> no offset.
	if x, y := lookOffset(96, 96, w, h, 10, 10); x != 0 || y != 0 {
		t.Errorf("center offset = (%v, %v), want (0, 0)", x, y)
	}
	// Cursor to the right -> full +X.
	if x, y := lookOffset(w, h/2, w, h, 10, 10); x != 10 || y != 0 {
		t.Errorf("right offset = (%v, %v), want (10, 0)", x, y)
	}
	// Far cursor is capped by max.
	x, y := lookOffset(w*10, h/2, w, h, 10, 10)
	if x != 10 || y != 0 {
		t.Errorf("capped offset = (%v, %v), want (10, 0)", x, y)
	}
	// Up-right diagonal: components are scaled by max, not both at max.
	x, y = lookOffset(w, 0, w, h, 10, 10)
	if x <= 0 || y >= 0 {
		t.Errorf("diagonal offset = (%v, %v), want x>0, y<0", x, y)
	}
	if x > 10 || y < -10 {
		t.Errorf("diagonal offset out of range: (%v, %v)", x, y)
	}
}
