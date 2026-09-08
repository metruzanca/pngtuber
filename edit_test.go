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
