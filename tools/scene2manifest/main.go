// scene2manifest generates a pngtuber manifest.toml from a Godot scene dump
// (Milestone 7 / §5.8). It consumes the JSON produced by tools/dump_scene.gd:
//
//	go run ./tools/scene2manifest -dump nodes.json -skin alien_cat -out manifest.toml
//
// The dump is an array of Sprite2D nodes with name, position (center), z_index
// and texture. It maps the BitBuddy coworker node-naming convention to rig
// parts (Body→body, LeftHandUp/Down→left variants, MouthN→mouth frames, ...),
// converts center positions to top-left offsets, normalizes the rig to the
// (0,0) origin, orders parts by z_index, and emits parts/offsets plus the
// standard per-state variant mapping and animations. Part offsets are exact —
// derived from the scene, not hand-tuned.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// node is one Sprite2D from the dump.
type node struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Pos     []float64 `json:"position"`
	ZIndex  int       `json:"z_index"`
	Texture string    `json:"texture"`
	TexSize []int     `json:"texture_size"`
}

// variant is one image file of a rig part with its composition offset.
type variant struct {
	file   string
	offset [2]int
	z      int
	ord    int
	src    string // originating scene node name
}

// deskLayout places the shared desk environment (desk surface, keyboard, mouse
// device) around the character. These are static single-variant parts, shared
// across skins; the offsets were chosen to sit under the coworker scene's hand
// anchor points (see the README/docs). Copy the referenced PNGs into each skin
// dir alongside the generated manifest.
var deskLayout = []struct {
	part, file string
	x, y       int
}{
	{"desk", "desk_1.png", 0, 148},
	{"keyboard", "keyboard_1.png", 85, 78},
	{"mouse_dev", "mouse_1.png", 126, 68},
}

// mapNode maps a scene node name to a rig part + variant index (0-based, in
// the order the variants will appear in the manifest). Variant 0 is the
// default/resting variant: for the arms the resting pose is the "_down" sprite
// (hands resting on the desk/keyboard), so "_down" maps to variant 0 and the
// "_up" sprite to variant 1 (used by the press animation while typing).
func mapNode(name string) (part string, ord int, ok bool) {
	switch {
	case name == "Body", name == "BodyNoMouth":
		// BodyNoMouth (no baked-in mouth) is preferred when the skin renders a
		// separate mouth part; the preference is applied in setVariant.
		return "body", 0, true
	case name == "Head":
		return "head", 0, true
	case name == "Eye":
		return "eye", 0, true
	case name == "Eyelid":
		return "eyelid", 0, true
	case strings.HasPrefix(name, "Mouth"):
		n, err := strconv.Atoi(strings.TrimPrefix(name, "Mouth"))
		if err != nil {
			return "", 0, false
		}
		return "mouth", n - 1, true
	case name == "LeftHandUp":
		return "left", 1, true
	case name == "LeftHandDown":
		return "left", 0, true
	case name == "RightHandUp":
		return "right", 1, true
	case name == "RightHandDown":
		return "right", 0, true
	case name == "MouseHandUp":
		return "mouse", 1, true
	case name == "MouseHandDown":
		return "mouse", 0, true
	}
	return "", 0, false
}

// setVariant stores a variant, preferring BodyNoMouth over the plain Body when
// a skin provides both.
func setVariant(parts map[string]map[int]variant, part string, ord int, v variant) {
	if parts[part] == nil {
		parts[part] = map[int]variant{}
	}
	existing, ok := parts[part][ord]
	if !ok || (v.src == "BodyNoMouth" && existing.src != "BodyNoMouth") {
		parts[part][ord] = v
	}
}

func main() {
	dumpPath := flag.String("dump", "", "node dump JSON from tools/dump_scene.gd")
	outPath := flag.String("out", "manifest.toml", "output manifest path")
	skin := flag.String("skin", "", "skin name (used as [character] skin)")
	flag.Parse()

	if *dumpPath == "" {
		log.Fatal("need -dump <nodes.json>")
	}
	data, err := os.ReadFile(*dumpPath)
	if err != nil {
		log.Fatal(err)
	}
	var nodes []node
	if err := json.Unmarshal(data, &nodes); err != nil {
		log.Fatal(err)
	}

	manifest := buildManifest(nodes, *skin)
	if err := os.WriteFile(*outPath, []byte(manifest), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%d parts)", *outPath, len(partsIn(nodes)))
}

// partsIn counts distinct parts produced from a node dump (for logging).
func partsIn(nodes []node) map[string]struct{} {
	out := map[string]struct{}{}
	for _, n := range nodes {
		if p, _, ok := mapNode(n.Name); ok {
			out[p] = struct{}{}
		}
	}
	return out
}

// buildManifest turns a scene dump into a manifest.toml string.
func buildManifest(nodes []node, skin string) string {
	// part -> variant index -> variant
	parts := map[string]map[int]variant{}
	for _, n := range nodes {
		if n.Type != "Sprite2D" || n.Texture == "" || len(n.Pos) < 2 || len(n.TexSize) < 2 {
			continue
		}
		part, ord, ok := mapNode(n.Name)
		if !ok {
			continue
		}
		off := [2]int{
			int(math.Round(n.Pos[0] - float64(n.TexSize[0])/2)),
			int(math.Round(n.Pos[1] - float64(n.TexSize[1])/2)),
		}
		setVariant(parts, part, ord, variant{
			file:   filepath.Base(n.Texture),
			offset: off,
			z:      n.ZIndex,
			ord:    ord,
			src:    n.Name,
		})
	}

	// Inject the shared desk environment parts (single variant each).
	for _, d := range deskLayout {
		setVariant(parts, d.part, 0, variant{
			file:   d.file,
			offset: [2]int{d.x, d.y},
			z:      -100,
			ord:    0,
			src:    d.part,
		})
	}

	if len(parts) == 0 {
		return ""
	}

	// Normalize offsets so the rig starts at (0,0).
	minX, minY := math.MaxInt, math.MaxInt
	for _, vs := range parts {
		for _, v := range vs {
			minX = min(minX, v.offset[0])
			minY = min(minY, v.offset[1])
		}
	}
	for _, vs := range parts {
		for k, v := range vs {
			v.offset[0] -= minX
			v.offset[1] -= minY
			vs[k] = v
		}
	}

	// parts_order: deliberate layering — desk environment at the back, then
	// the character, then the keyboard/mouse device in front of the character,
	// then the hands on top of the input devices.
	prefOrder := []string{"desk", "body", "head", "eye", "eyelid", "mouth", "keyboard", "mouse_dev", "mouse", "right", "left"}
	order := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range prefOrder {
		if parts[p] != nil && !seen[p] {
			order = append(order, p)
			seen[p] = true
		}
	}
	for p := range parts {
		if !seen[p] {
			order = append(order, p)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# pngtuber manifest generated by tools/scene2manifest from the\n")
	fmt.Fprintf(&b, "# skin's coworker_*.scn rig scene. Offsets are scene-derived.\n\n")
	fmt.Fprintf(&b, "[character]\nskin = %q\n", skin)
	fmt.Fprintf(&b, "parts_order = [%s]\n\n", quoteList(order))

	fmt.Fprintf(&b, "[character.parts]\n")
	for _, p := range order {
		vs := parts[p]
		nframes := len(vs)
		if p == "eyelid" {
			nframes = 2 // variant 0 hidden (open), variant 1 = closed lid
		}
		files := make([]string, 0, nframes)
		for i := 0; i < nframes; i++ {
			if p == "eyelid" {
				if i == 0 {
					files = append(files, "-")
				} else {
					files = append(files, vs[0].file)
				}
			} else {
				files = append(files, vs[i].file)
			}
		}
		fmt.Fprintf(&b, "%-8s = [%s]\n", p, quoteList(files))
	}

	fmt.Fprintf(&b, "\n[character.offsets]\n")
	for _, p := range order {
		vs := parts[p]
		nframes := len(vs)
		if p == "eyelid" {
			nframes = 2
		}
		offs := make([]string, 0, nframes)
		for i := 0; i < nframes; i++ {
			off := vs[0].offset
			if p != "eyelid" {
				off = vs[i].offset
			}
			offs = append(offs, fmt.Sprintf("{ x = %d, y = %d }", off[0], off[1]))
		}
		fmt.Fprintf(&b, "%-8s = [%s]\n", p, strings.Join(offs, ", "))
	}

	hasEyelid := parts["eyelid"] != nil
	fmt.Fprintf(&b, "\n[character.groups]\n")
	if parts["eye"] != nil && hasEyelid {
		// The eye and eyelid sit together; edit mode drags them as one unit.
		fmt.Fprintf(&b, "eyes = [\"eye\", \"eyelid\"]\n")
	}

	fmt.Fprintf(&b, "\n[character.visibility]\n")
	if parts["mouse"] != nil {
		fmt.Fprintf(&b, "mouse_hand = [\"mouse\"]\n")
	}
	if parts["left"] != nil {
		fmt.Fprintf(&b, "keyboard_hand = [\"left\"]\n")
	}

	fmt.Fprintf(&b, "\n[character.variants]\n")
	if hasEyelid {
		fmt.Fprintf(&b, "eyelid = { sleep = 1 }\n")
	}

	fmt.Fprintf(&b, "\n[character.animations]\n")
	fmt.Fprintf(&b, "mouth = { part = \"mouth\", fps = 10.0 }\n")
	fmt.Fprintf(&b, "press = { parts = [\"left\", \"right\"], duration_secs = 0.1, variant = 1 }\n")
	fmt.Fprintf(&b, "breathing = { parts = [\"body\", \"head\"], period_secs = 4.0, amplitude = 0.015 }\n")
	fmt.Fprintf(&b, "hand_move_delay_secs = 0.5\n")
	if hasEyelid {
		fmt.Fprintf(&b, "blink = { part = \"eyelid\", closed_variant = 1, interval_secs = 3.0, duration_secs = 0.15 }\n")
	} else {
		fmt.Fprintf(&b, "blink = { part = \"\", closed_variant = 0, interval_secs = 0, duration_secs = 0 }\n")
	}

	fmt.Fprintf(&b, "\n[character.tracking]\n")
	if parts["mouse"] != nil {
		fmt.Fprintf(&b, "parts = [\"mouse\", \"mouse_dev\"]\nstretch = [\"mouse\"]\n")
	} else {
		fmt.Fprintf(&b, "parts = [\"mouse_dev\"]\nstretch = []\n")
	}
	fmt.Fprintf(&b, "max_x = 20\nmax_y = 12\nsensitivity = 0.02\n")
	fmt.Fprintf(&b, "\n[character.look]\nparts = []\nmax_x = 10\nmax_y = 10\n")
	fmt.Fprintf(&b, "\n[activity]\nidle_after_secs = 5.0\nsleep_after_secs = 120.0\nmic_threshold = 0.08\nmic_hysteresis = 0.5\ngaming_window_secs = 1.0\n")
	return b.String()
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = strconv.Quote(s)
	}
	return strings.Join(quoted, ", ")
}
