package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// TestWriteOffsets verifies only the [character.offsets] section is rewritten
// and the rest of the file (comments, other tables) is preserved.
func TestWriteOffsets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.toml")
	orig := "# a comment\n[character]\nskin = \"test\"\n\n[character.offsets]\ndesk = [{ x = 1, y = 2 }]\nbody = [{ x = 3, y = 4 }, { x = 5, y = 6 }]\n\n[character.look]\nmax_x = 10\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}

	offsets := map[string][]Offset{
		"desk": {{X: 10, Y: 20}},
		"body": {{X: 30, Y: 40}, {X: 50, Y: 60}},
	}
	if err := writeOffsets(path, []string{"desk", "body"}, offsets); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		"# a comment",
		`skin = "test"`,
		"[character.offsets]\n",
		"desk     = [{ x = 10, y = 20 }]",
		"body     = [{ x = 30, y = 40 }, { x = 50, y = 60 }]",
		"[character.look]",
		"max_x = 10",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	// The old offset values must be gone, and the section header must not
	// appear twice (a prior bug duplicated it).
	if strings.Contains(s, "x = 1, y = 2") {
		t.Errorf("old offsets not replaced:\n%s", s)
	}
	if n := strings.Count(s, "[character.offsets]"); n != 1 {
		t.Errorf("[character.offsets] appears %d times, want 1:\n%s", n, s)
	}
	// The result must still parse as a manifest.
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("saved manifest does not parse: %v\n%s", err, s)
	}
	if got := cfg.Character.Offsets["body"]; len(got) != 2 || got[0].X != 30 {
		t.Errorf("reloaded body offsets = %+v", got)
	}
}

func TestHitPart(t *testing.T) {
	// Two parts: body (10,10 20x20) and head (40,0 10x10); head drawn last.
	rig := &Rig{
		order:   []string{"body", "head"},
		parts:   map[string][]*ebiten.Image{"body": {nil}, "head": {nil}},
		offsets: map[string][]Offset{"body": {{X: 10, Y: 10}}, "head": {{X: 40, Y: 0}}},
	}
	// Fake image sizes via a 1x1? partRect uses image bounds; use real images.
	rig.parts["body"][0] = ebiten.NewImage(20, 20)
	rig.parts["head"][0] = ebiten.NewImage(10, 10)

	c := NewCharacter(rig, testConfig())
	cases := []struct {
		x, y int
		want string
	}{
		{15, 15, "body"},
		{45, 5, "head"},
		{100, 100, ""}, // empty space
	}
	for _, tc := range cases {
		part, ok := c.HitPart(tc.x, tc.y)
		if tc.want == "" {
			if ok {
				t.Errorf("HitPart(%d,%d) = %q, want none", tc.x, tc.y, part)
			}
			continue
		}
		if !ok || part != tc.want {
			t.Errorf("HitPart(%d,%d) = %q,%v want %q", tc.x, tc.y, part, ok, tc.want)
		}
	}
}

func TestShiftPartMovesAllVariants(t *testing.T) {
	rig := &Rig{
		offsets: map[string][]Offset{"left": {{X: 1, Y: 2}, {X: 3, Y: 4}}},
	}
	c := NewCharacter(rig, testConfig())
	c.ShiftPart("left", 10, -5)
	got := rig.offsets["left"]
	if got[0] != (Offset{11, -3}) || got[1] != (Offset{13, -1}) {
		t.Errorf("shifted offsets = %+v, want [{11 -3} {13 -1}]", got)
	}
}

func TestPressAnimation(t *testing.T) {
	c := testCharacter()
	// Rest: no hand is pressed.
	c.SetState(Typing)
	c.Update(0.01)
	if got := c.variantIndex("left"); got != 0 {
		t.Fatalf("left rest = %d, want 0", got)
	}
	// One key press taps a hand down.
	c.Press(1)
	active := c.variantIndex("left")
	other := c.variantIndex("right")
	if active != 1 && other != 1 {
		t.Fatalf("a hand should be pressing after Press(1): left=%d right=%d", active, other)
	}
	// It returns up after the duration.
	c.Update(0.15)
	if got := c.variantIndex("left"); got != 0 {
		t.Fatalf("left after decay = %d, want 0", got)
	}
	if got := c.variantIndex("right"); got != 0 {
		t.Fatalf("right after decay = %d, want 0", got)
	}
}

func TestPressAlternatesHands(t *testing.T) {
	c := testCharacter()
	c.Press(1)
	first := c.pressIndex
	c.Update(0.001)
	c.Press(1)
	second := c.pressIndex
	if first == second {
		t.Errorf("press did not alternate: index %d -> %d", first, second)
	}
	// With two press parts, the two presses hit different hands.
	if c.variantIndex("left") == 0 && c.variantIndex("right") == 0 {
		t.Errorf("after two presses at least one hand should be pressing")
	}
}

// TestPartVisible verifies the mouse hand and left keyboard hand are mutually
// exclusive (same physical hand), the mouse device + right hand stay visible,
// and both show in edit mode.
func TestPartVisible(t *testing.T) {
	c := testCharacter()
	mouseOn := func() {
		c.hands = Mouse
		if !c.partVisible("mouse") || c.partVisible("left") {
			t.Errorf("mouse active: mouse visible=%v left visible=%v, want true/false", c.partVisible("mouse"), c.partVisible("left"))
		}
		if !c.partVisible("mouse_dev") || !c.partVisible("right") {
			t.Error("mouse device / right hand must stay visible while mouse active")
		}
	}
	mouseOff := func() {
		c.hands = Typing
		if c.partVisible("mouse") || !c.partVisible("left") {
			t.Errorf("typing: mouse visible=%v left visible=%v, want false/true", c.partVisible("mouse"), c.partVisible("left"))
		}
		if !c.partVisible("mouse_dev") || !c.partVisible("right") {
			t.Error("mouse device / right hand must stay visible while typing")
		}
	}
	mouseOn()
	mouseOff()
	// Gaming also counts as mouse-active.
	c.hands = Gaming
	if !c.partVisible("mouse") || c.partVisible("left") {
		t.Error("gaming: mouse hand should show, left keyboard hand hidden")
	}
	// Edit mode (steady): both hands visible.
	c.SetSteady(true)
	c.hands = Typing
	if !c.partVisible("mouse") || !c.partVisible("left") {
		t.Error("edit mode should show both hand poses")
	}
}

// TestPartGroup verifies dragging a group member moves the whole group.
func TestPartGroup(t *testing.T) {
	c := testCharacter()
	got := c.PartGroup("eye")
	if len(got) != 2 || got[0] != "eye" || got[1] != "eyelid" {
		t.Fatalf("PartGroup(eye) = %v, want [eye eyelid]", got)
	}
	if got := c.PartGroup("body"); len(got) != 1 || got[0] != "body" {
		t.Fatalf("PartGroup(body) = %v, want [body]", got)
	}
	// A group drag moves every member by the same delta.
	rig := &Rig{offsets: map[string][]Offset{
		"eye":    {{X: 1, Y: 2}},
		"eyelid": {{X: 5, Y: 6}},
	}}
	c2 := NewCharacter(rig, testConfig())
	for _, p := range c2.PartGroup("eye") {
		rig.offsets[p] = shiftedOffsets(rig.offsets[p], 10, 5)
	}
	if rig.offsets["eye"][0] != (Offset{11, 7}) || rig.offsets["eyelid"][0] != (Offset{15, 11}) {
		t.Errorf("group drag offsets = %+v, want eye{11,7} eyelid{15,11}", rig.offsets)
	}
}

// TestStretchGeoM verifies the hand stretch in IMAGE coordinates (what
// DrawImage feeds): the whole top row stays fixed at the base offset and the
// bottom edge reaches the tracked mouse offset.
func TestStretchGeoM(t *testing.T) {
	const w, h = 40, 50
	off := Offset{X: 10, Y: 20}
	dx, dy := 8.0, -6.0

	closeEnough := func(name string, got, want, tol float64) {
		if got < want-tol || got > want+tol {
			t.Errorf("%s = %v, want ~%v", name, got, want)
		}
	}

	m := stretchGeoM(off, w, h, dx, dy)
	// Top row (every x) is pinned at the base position.
	tx, ty := m.Apply(float64(w)/2, 0)
	closeEnough("top-center x", tx, float64(off.X+w/2), 0.001)
	closeEnough("top-center y", ty, float64(off.Y), 0.001)
	lx, ly := m.Apply(0, 0)
	closeEnough("top-left x", lx, float64(off.X), 0.001)
	closeEnough("top-left y", ly, float64(off.Y), 0.001)
	// Bottom row slides uniformly by (dx, dy). Composed GeoM matrices carry
	// sub-pixel float drift (~0.07px), so use a 0.1 tolerance here.
	bx, by := m.Apply(float64(w)/2, float64(h))
	closeEnough("bottom-center x", bx, float64(off.X+w/2)+dx, 0.1)
	closeEnough("bottom-center y", by, float64(off.Y+h)+dy, 0.1)
	// Zero offset is identity placement.
	m0 := stretchGeoM(off, w, h, 0, 0)
	x, y := m0.Apply(0, 0)
	closeEnough("zero-stretch x", x, float64(off.X), 0.001)
	closeEnough("zero-stretch y", y, float64(off.Y), 0.001)
}

func TestLookPartsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.toml")
	if err := os.WriteFile(path, []byte("[character.look]\nparts = [\"head\"]\nmax_x = 10\nmax_y = 10\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Character.Look.Parts) != 1 || cfg.Character.Look.Parts[0] != "head" {
		t.Errorf("Look.Parts = %v, want [head]", cfg.Character.Look.Parts)
	}
}

func TestMouseTrackingIntegratesAndClamps(t *testing.T) {
	c := testCharacter()
	c.hands = Mouse // mouse active
	c.UpdateMouse(500, -300, 0.016)
	// 500*0.02 = 10, -300*0.02 = -6.
	if c.mouseOffX != 10 || c.mouseOffY != -6 {
		t.Fatalf("mouseOff = %v,%v want 10,-6", c.mouseOffX, c.mouseOffY)
	}
	// Large motion clamps to MaxX/MaxY (20/12).
	c.UpdateMouse(2000, 2000, 0.016)
	if c.mouseOffX != 20 || c.mouseOffY != 12 {
		t.Fatalf("clamped mouseOff = %v,%v want 20,12", c.mouseOffX, c.mouseOffY)
	}
}

func TestMouseTrackingDecaysWhenInactive(t *testing.T) {
	c := testCharacter()
	c.hands = Mouse
	c.UpdateMouse(500, 300, 0.016)
	if c.mouseOffX == 0 {
		t.Fatal("mouse should have moved")
	}
	c.hands = Typing // mouse hand rests
	// Over enough time it springs back to center.
	for i := 0; i < 100; i++ {
		c.UpdateMouse(0, 0, 0.1)
	}
	if c.mouseOffX != 0 || c.mouseOffY != 0 {
		t.Fatalf("mouseOff should decay to 0, got %v,%v", c.mouseOffX, c.mouseOffY)
	}
}

func TestMouseTrackingSteadyResets(t *testing.T) {
	c := testCharacter()
	c.hands = Mouse
	c.UpdateMouse(500, 300, 0.016)
	c.SetSteady(true)
	if c.mouseOffX != 0 || c.mouseOffY != 0 {
		t.Fatalf("steady should reset mouseOff, got %v,%v", c.mouseOffX, c.mouseOffY)
	}
}

// TestExitEditModeSaves verifies that leaving edit mode persists the offsets.
func TestExitEditModeSaves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.toml")
	orig := "[character]\nskin = \"t\"\n\n[character.offsets]\nbody = [{ x = 1, y = 2 }]\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	rig, err := LoadRig(cfg, dir) // no part images; rig.normalize handles it
	if err != nil {
		t.Fatal(err)
	}
	rig.order = []string{"body"}
	rig.offsets["body"][0] = Offset{X: 42, Y: 7} // simulate a drag
	g := &Game{manifestPath: path, char: NewCharacter(rig, cfg.Character)}

	g.setEditMode(false) // exit (no-op since not on) — should not save

	// Enter then exit edit mode: positions must be persisted.
	g.setEditMode(true)
	g.setEditMode(false)
	cfg2, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("saved manifest does not parse: %v", err)
	}
	if got := cfg2.Character.Offsets["body"][0]; got != (Offset{X: 42, Y: 7}) {
		t.Errorf("exiting edit mode did not save: body = %+v, want {42 7}", got)
	}
}
