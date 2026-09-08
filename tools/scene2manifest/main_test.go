package main

import (
	"strings"
	"testing"
)

// TestMapNode covers the BitBuddy coworker naming conventions, including the
// per-skin variation frog uses (Body + BodyNoMouth).
func TestMapNode(t *testing.T) {
	cases := map[string]struct {
		part string
		ord  int
	}{
		"Body":             {"body", 0},
		"BodyNoMouth":      {"body", 0},
		"Head":             {"head", 0},
		"Eye":              {"eye", 0},
		"Eyelid":           {"eyelid", 0},
		"Mouth1":           {"mouth", 0},
		"Mouth3":           {"mouth", 2},
		"LeftHandUp":       {"left", 0},
		"LeftHandDown":     {"left", 1},
		"RightHandUp":      {"right", 0},
		"RightHandDown":    {"right", 1},
		"MouseHandUp":      {"mouse", 0},
		"MouseHandDown":    {"mouse", 1},
		"Desk":             {"", 0},
		"AnimatedSprite2D": {"", 0},
	}
	for name, want := range cases {
		part, ord, ok := mapNode(name)
		if want.part == "" {
			if ok {
				t.Errorf("mapNode(%q) unexpectedly ok (%s,%d)", name, part, ord)
			}
			continue
		}
		if !ok || part != want.part || ord != want.ord {
			t.Errorf("mapNode(%q) = (%s,%d,%v), want (%s,%d,true)", name, part, ord, ok, want.part, want.ord)
		}
	}
}

// TestBuildManifestPrefersBodyNoMouth verifies a skin that provides both Body
// and BodyNoMouth (frog) ends up with the no-mouth body, since pngtuber draws
// the mouth as a separate part.
func TestBuildManifestPrefersBodyNoMouth(t *testing.T) {
	nodes := []node{
		{Name: "Body", Type: "Sprite2D", Pos: []float64{150, 68}, ZIndex: 0, Texture: "res://frog_body.png", TexSize: []int{100, 100}},
		{Name: "BodyNoMouth", Type: "Sprite2D", Pos: []float64{150, 68}, ZIndex: 0, Texture: "res://frog_body_no_mouth.png", TexSize: []int{100, 100}},
		{Name: "LeftHandUp", Type: "Sprite2D", Pos: []float64{100, 50}, ZIndex: 6, Texture: "res://frog_left_up.png", TexSize: []int{20, 20}},
	}
	out := buildManifest(nodes, "frog")
	if !strings.Contains(out, `body     = ["frog_body_no_mouth.png"]`) {
		t.Errorf("expected no-mouth body preferred, got:\n%s", out)
	}
}

// TestBuildManifestEmitsDesk verifies the shared desk environment parts are
// always emitted and ordered behind the character, before the hands.
func TestBuildManifestEmitsDesk(t *testing.T) {
	nodes := []node{
		{Name: "Body", Type: "Sprite2D", Pos: []float64{150, 68}, ZIndex: 0, Texture: "res://b.png", TexSize: []int{40, 40}},
		{Name: "LeftHandUp", Type: "Sprite2D", Pos: []float64{100, 50}, ZIndex: 6, Texture: "res://l.png", TexSize: []int{20, 20}},
	}
	out := buildManifest(nodes, "test")
	for _, want := range []string{
		`parts_order = ["desk", "body", "keyboard", "mouse_dev", "left"]`,
		`desk     = ["desk_1.png"]`,
		`keyboard = ["keyboard_1.png"]`,
		`mouse_dev = ["mouse_1.png"]`,
		`[character.tracking]`,
		`hand_move_delay_secs = 1.0`,
		`breathing = { parts = ["body", "head"]`,
		`press = { parts = ["left", "right"], duration_secs = 0.1, variant = 1 }`,
		`mouse  = { mouse = 1, gaming = 1 }`,
		"[character.look]\nparts = []",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// left/right are not statically "down" while typing; presses drive them.
	if strings.Contains(out, "left   = { typing") || strings.Contains(out, "right  = { typing") {
		t.Errorf("left/right should not have a static typing mapping:\n%s", out)
	}
}

// TestBuildManifestNormalizesOffsets verifies offsets are scene-derived and
// normalized so the rig starts at (0,0), independent of code.
func TestBuildManifestNormalizesOffsets(t *testing.T) {
	nodes := []node{
		// LeftHandUp center (100,50), size 20x20 -> top-left (90,40).
		{Name: "LeftHandUp", Type: "Sprite2D", Pos: []float64{100, 50}, ZIndex: 6, Texture: "res://l.png", TexSize: []int{20, 20}},
		// Body center (150,120), size 40x40 -> top-left (130,100).
		{Name: "Body", Type: "Sprite2D", Pos: []float64{150, 120}, ZIndex: 0, Texture: "res://b.png", TexSize: []int{40, 40}},
	}
	out := buildManifest(nodes, "test")
	// Desk is injected at (0,148), so min is (0,40): body -> (130,60), left -> (90,0).
	if !strings.Contains(out, "body     = [{ x = 130, y = 60 }]") {
		t.Errorf("body offset not normalized, got:\n%s", out)
	}
	if !strings.Contains(out, "left     = [{ x = 90, y = 0 }]") {
		t.Errorf("left offset not normalized, got:\n%s", out)
	}
}

// TestBuildManifestPartOrder verifies the deliberate layering: desk environment
// at the back, then the character, then the hands last.
func TestBuildManifestPartOrder(t *testing.T) {
	nodes := []node{
		{Name: "LeftHandUp", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 6, Texture: "res://l.png", TexSize: []int{10, 10}},
		{Name: "Eye", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 0, Texture: "res://e.png", TexSize: []int{10, 10}},
		{Name: "Head", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 0, Texture: "res://h.png", TexSize: []int{10, 10}},
		{Name: "Body", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 0, Texture: "res://b.png", TexSize: []int{10, 10}},
		{Name: "Eyelid", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 1, Texture: "res://lid.png", TexSize: []int{10, 10}},
	}
	out := buildManifest(nodes, "test")
	want := `parts_order = ["desk", "body", "head", "eye", "eyelid", "keyboard", "mouse_dev", "left"]`
	if !strings.Contains(out, want) {
		t.Errorf("parts_order mismatch, got:\n%s", out)
	}
}

// TestBuildManifestEmitsAnimations verifies the blink + mouth talk animations
// and the hidden open-eyelid variant.
func TestBuildManifestEmitsAnimations(t *testing.T) {
	nodes := []node{
		{Name: "Body", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 0, Texture: "res://b.png", TexSize: []int{10, 10}},
		{Name: "Eyelid", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 1, Texture: "res://lid.png", TexSize: []int{10, 10}},
		{Name: "Mouth1", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 4, Texture: "res://m1.png", TexSize: []int{10, 10}},
		{Name: "Mouth2", Type: "Sprite2D", Pos: []float64{0, 0}, ZIndex: 4, Texture: "res://m2.png", TexSize: []int{10, 10}},
	}
	out := buildManifest(nodes, "test")
	for _, want := range []string{
		`eyelid   = ["-", "lid.png"]`,
		`blink = { part = "eyelid", closed_variant = 1,`,
		`mouth = { part = "mouth", fps = 10.0 }`,
		`eyelid = { sleep = 1 }`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
