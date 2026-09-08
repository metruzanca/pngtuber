package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func testConfig() CharacterConfig {
	return CharacterConfig{
		Variants: map[string]map[string]int{
			"eyelid": {"sleep": 1},
		},
		Visibility: VisibilityConfig{
			MouseHand:    []string{"mouse"},
			KeyboardHand: []string{"left"},
		},
		Groups: map[string][]string{
			"eyes": {"eye", "eyelid"},
		},
		Animations: AnimationsConfig{
			Mouth:             MouthAnimConfig{Part: "mouth", FPS: 10},
			Press:             PressAnimConfig{Parts: []string{"left", "right"}, DurationSecs: 0.1, Variant: 1},
			Blink:             BlinkAnimConfig{Part: "eyelid", ClosedVariant: 1, IntervalSecs: 3.0, DurationSecs: 0.15},
			Breathing:         BreathingConfig{Parts: []string{"body", "head"}, PeriodSecs: 4.0, Amplitude: 0.015},
			HandMoveDelaySecs: 1.0,
		},
		Tracking: TrackingConfig{Parts: []string{"mouse", "mouse_dev"}, MaxX: 20, MaxY: 12},
	}
}

func testCharacter() *Character {
	rig := &Rig{parts: map[string][]*ebiten.Image{"mouth": {nil, nil, nil}}}
	return NewCharacter(rig, testConfig())
}

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

// TestVariantSelectionHands verifies the arms rest DOWN (variant 0) in every
// state by default — the press animation (tested separately) lifts them.
func TestVariantSelectionHands(t *testing.T) {
	c := testCharacter()
	cases := []struct {
		state ActivityState
		left  int
		right int
		mouse int
	}{
		{Idle, 0, 0, 0},
		{Typing, 0, 0, 0},
		{Mouse, 0, 0, 0},
		{Gaming, 0, 0, 0},
		{Talking, 0, 0, 0},
		{Sleep, 0, 0, 0},
	}
	for _, tc := range cases {
		c.hands = tc.state
		if got := c.variantIndex("left"); got != tc.left {
			t.Errorf("%s: left = %d, want %d", tc.state, got, tc.left)
		}
		if got := c.variantIndex("right"); got != tc.right {
			t.Errorf("%s: right = %d, want %d", tc.state, got, tc.right)
		}
		if got := c.variantIndex("mouse"); got != tc.mouse {
			t.Errorf("%s: mouse = %d, want %d", tc.state, got, tc.mouse)
		}
	}
}

func TestVariantSelectionEyelid(t *testing.T) {
	c := testCharacter()
	c.raw = Sleep
	if got := c.variantIndex("eyelid"); got != 1 {
		t.Fatalf("sleep eyelid = %d, want 1 (closed)", got)
	}

	c.raw = Idle
	if got := c.variantIndex("eyelid"); got != 0 {
		t.Fatalf("awake eyelid = %d, want 0 (open)", got)
	}
	c.blinking = true
	if got := c.variantIndex("eyelid"); got != 1 {
		t.Fatalf("blinking eyelid = %d, want 1 (closed)", got)
	}
}

func TestVariantSelectionMouth(t *testing.T) {
	c := testCharacter()
	if got := c.variantIndex("mouth"); got != 0 {
		t.Fatalf("non-talking mouth = %d, want 0 (closed)", got)
	}
	c.SetTalking(true)
	c.mouth.elapsed = 0.2
	if got := c.variantIndex("mouth"); got != 2 {
		t.Fatalf("talking mouth = %d, want 2", got)
	}
	// Dropping below threshold stops the flap immediately.
	c.SetTalking(false)
	if got := c.variantIndex("mouth"); got != 0 {
		t.Fatalf("mouth after threshold drop = %d, want 0 (closed)", got)
	}
}

// TestSetTalkingResetsAnimation verifies the mouth loop resets when talking
// starts and that the flap stops immediately when the mic drops out.
func TestSetTalkingResetsAnimation(t *testing.T) {
	c := testCharacter()
	c.SetTalking(true)
	c.mouth.elapsed = 5.0
	c.Update(0.05)
	if c.mouth.index() == 0 {
		t.Fatal("mouth loop should be past frame 0 before reset")
	}
	// Turning talking off stops the flap instantly (no held Talking state).
	c.SetTalking(false)
	if got := c.variantIndex("mouth"); got != 0 {
		t.Errorf("mouth after stop = %d, want 0", got)
	}
	// Re-entering starts the loop from 0 again.
	c.SetTalking(true)
	if got := c.variantIndex("mouth"); got != 0 {
		t.Errorf("mouth after re-enter = %d, want 0", got)
	}
}

func TestHandMoveDelay(t *testing.T) {
	c := testCharacter()
	// Committed hands are Idle; typing arrives but the hands must wait.
	c.SetState(Typing)
	c.Update(0.5)
	if c.hands != Idle {
		t.Fatalf("hands = %s before delay, want idle", c.hands)
	}
	c.Update(0.6)
	if c.hands != Typing {
		t.Fatalf("hands = %s after delay, want typing", c.hands)
	}
}

func TestHandMoveDelayFlicker(t *testing.T) {
	c := testCharacter()
	// The mouse hand is committed on the mouse (gaming). A brief mouse stall
	// flips the raw state to Typing and back — the hands must never move.
	c.hands = Gaming
	c.raw = Gaming
	c.SetState(Typing) // stall
	c.Update(0.5)
	c.SetState(Gaming) // mouse resumes before the delay elapses
	c.Update(0.5)
	if c.hands != Gaming {
		t.Fatalf("hands = %s after flicker, want gaming (mouse hand stays)", c.hands)
	}
	// A genuine sustained switch to typing does move the hands.
	c.SetState(Typing)
	c.Update(1.1)
	if c.hands != Typing {
		t.Fatalf("hands = %s after sustained typing, want typing", c.hands)
	}
}

func TestHandMoveSleepInstant(t *testing.T) {
	c := testCharacter()
	c.SetState(Typing)
	c.Update(1.1) // hands commit to typing (delay is 1.0s)
	if c.hands != Typing {
		t.Fatalf("hands = %s, want typing", c.hands)
	}
	c.SetState(Sleep)
	c.Update(0.01) // sleep entry is instant
	if c.hands != Sleep {
		t.Fatalf("hands = %s after sleep, want sleep", c.hands)
	}
	c.SetState(Gaming)
	c.Update(0.01) // waking is instant too
	if c.hands != Gaming {
		t.Fatalf("hands = %s after wake, want gaming", c.hands)
	}
}

func TestMouthInstantWhileHandsDebounced(t *testing.T) {
	c := testCharacter()
	c.SetTalking(true) // talking arrives, hands still committed to idle
	c.Update(0.05)
	if c.hands != Idle {
		t.Fatalf("hands = %s, want idle (debounced)", c.hands)
	}
	if got := c.variantIndex("mouth"); got != c.mouth.index() {
		t.Fatalf("mouth should animate instantly, got %d", got)
	}
	if got := c.variantIndex("left"); got != 0 {
		t.Fatalf("left = %d, want 0 (hands not moved by talking)", got)
	}
	// The mouth is independent of the raw activity state: even while the
	// machine would still report Talking (held), dropping the mic stops it.
	c.SetState(Talking) // raw state says talking...
	c.SetTalking(false) // ...but audio is below threshold
	if got := c.variantIndex("mouth"); got != 0 {
		t.Fatalf("mouth should stop despite held Talking state, got %d", got)
	}
}

func TestBlinkCycle(t *testing.T) {
	c := testCharacter()
	if c.blinking {
		t.Fatal("should start not blinking")
	}
	// Real-world-sized updates (50ms). Well before the 3s interval: no blink.
	const step = 0.05
	const interval = 3.0
	const duration = 0.15
	for i := 0; i < int(interval/step)-2; i++ {
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
	// Past the blink duration the blink ends.
	for i := 0; i < 4; i++ {
		c.Update(step)
	}
	if c.blinking {
		t.Fatal("blink should have ended after duration")
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

// TestManifestDrivenBehavior proves the M6 exit criterion: editing
// manifest.toml changes which variant a part shows without recompiling.
func TestManifestDrivenBehavior(t *testing.T) {
	dir := t.TempDir()
	build := func(variants string) *Character {
		content := "[character]\n" +
			"skin = \"test\"\n" +
			"parts_order = [\"left\"]\n" +
			"[character.parts]\n" +
			"left = [\"up.png\", \"down.png\"]\n" +
			"[character.variants]\n" + variants + "\n" +
			"[character.offsets]\n" +
			"left = [{ x = 0, y = 0 }]\n"
		path := filepath.Join(dir, "manifest.toml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		rig := &Rig{parts: map[string][]*ebiten.Image{"left": {nil, nil}}}
		return NewCharacter(rig, cfg.Character)
	}

	// Mapping present: typing shows variant 1.
	c := build("left = { typing = 1 }")
	c.hands = Typing
	if got := c.variantIndex("left"); got != 1 {
		t.Fatalf("left/typing = %d, want 1", got)
	}
	c.hands = Idle
	if got := c.variantIndex("left"); got != 0 {
		t.Fatalf("left/idle = %d, want 0 (unmapped -> default)", got)
	}

	// Edit the manifest (remove the mapping): same state now shows variant 0.
	c2 := build("")
	c2.hands = Typing
	if got := c2.variantIndex("left"); got != 0 {
		t.Fatalf("left/typing after edit = %d, want 0", got)
	}

	// Editing the activity thresholds also takes effect without recompiling.
	threshPath := filepath.Join(dir, "manifest.toml")
	cfg, err := LoadConfig(threshPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Activity.IdleAfterSecs != 0 {
		t.Fatalf("default IdleAfterSecs = %v, want 0", cfg.Activity.IdleAfterSecs)
	}
	content, _ := os.ReadFile(threshPath)
	content = append(content, []byte("\n[activity]\nidle_after_secs = 7.0\n")...)
	if err := os.WriteFile(threshPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(threshPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Activity.IdleAfterSecs != 7.0 {
		t.Fatalf("edited IdleAfterSecs = %v, want 7.0", cfg.Activity.IdleAfterSecs)
	}
}
