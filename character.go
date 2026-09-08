// Character rig composition and state->part animation controller (Milestone 5).
//
// Parts are composited PNGs (body, head, eye, eyelid, hands, mouth) loaded
// from the manifest with offsets/z-order. Each part can have several variants
// (e.g. hand up/down, mouth-open frames). The per-state variant mapping and
// the two dynamic behaviors (mouth talk animation, blink) are all
// data-driven via the manifest (Milestone 6): [character.variants] and
// [character.animations], so editing the TOML changes behavior without
// recompiling. Variant sequences animate by accumulated elapsed time so the
// manifest's fps is honored regardless of the game's TPS. The head/eye parts
// track the cursor via a capped look offset.
package main

import (
	"fmt"
	_ "image/png"
	"math"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// frameLoop cycles through a frame sequence by accumulated elapsed time, so
// the manifest fps is accurate regardless of TPS. nframes==0 or 1 stays on
// frame 0.
type frameLoop struct {
	fps     float64
	elapsed float64
	nframes int
}

func (f *frameLoop) reset() { f.elapsed = 0 }

func (f *frameLoop) update(dt float64) { f.elapsed += dt }

// index returns the current frame, looping at nframes.
func (f *frameLoop) index() int {
	if f.nframes <= 1 {
		return 0
	}
	return int(f.elapsed*f.fps) % f.nframes
}

// Rig holds the loaded part images, composition order/offsets and look config.
type Rig struct {
	order   []string
	parts   map[string][]*ebiten.Image
	offsets map[string][]Offset
	look    LookConfig
	canvasW int
	canvasH int
}

// Canvas returns the composition canvas size in pixels (the rig's bounding
// box), used to size the window to fit any skin.
func (r *Rig) Canvas() (int, int) { return r.canvasW, r.canvasH }

// hiddenVariant marks a part variant that is not drawn (e.g. an open eyelid
// that is only shown when closed).
const hiddenVariant = "-"

// LoadRig loads every part variant from the manifest, resolving relative
// filenames against baseDir. The hiddenVariant marker ("-") maps to a nil
// image that Draw skips. Offsets are normalized so the rig starts at (0,0)
// and the canvas is sized to the parts' bounding box.
func LoadRig(cfg *Config, baseDir string) (*Rig, error) {
	r := &Rig{
		order:   cfg.Character.PartsOrder,
		parts:   make(map[string][]*ebiten.Image, len(cfg.Character.Parts)),
		offsets: cfg.Character.Offsets,
		look:    cfg.Character.Look,
	}
	for name, files := range cfg.Character.Parts {
		for _, f := range files {
			if f == hiddenVariant {
				r.parts[name] = append(r.parts[name], nil)
				continue
			}
			img, _, err := ebitenutil.NewImageFromFile(filepath.Join(baseDir, f))
			if err != nil {
				return nil, fmt.Errorf("load part %q (%s): %w", name, f, err)
			}
			r.parts[name] = append(r.parts[name], img)
		}
	}
	r.normalize()
	return r, nil
}

// normalize shifts all offsets so the rig starts at (0,0) and computes the
// canvas size from the parts' bounding box.
func (r *Rig) normalize() {
	minX, minY := math.MaxInt, math.MaxInt
	maxX, maxY := math.MinInt, math.MinInt
	for name, offs := range r.offsets {
		for i, off := range offs {
			imgs := r.parts[name]
			if i >= len(imgs) || imgs[i] == nil {
				continue
			}
			w, h := imgs[i].Bounds().Dx(), imgs[i].Bounds().Dy()
			minX = min(minX, off.X)
			minY = min(minY, off.Y)
			maxX = max(maxX, off.X+w)
			maxY = max(maxY, off.Y+h)
		}
	}
	if maxX < minX {
		return
	}
	for name := range r.offsets {
		for i := range r.offsets[name] {
			r.offsets[name][i].X -= minX
			r.offsets[name][i].Y -= minY
		}
	}
	r.canvasW = maxX - minX
	r.canvasH = maxY - minY
}

// Character composes the rig parts and switches variants by activity state.
type Character struct {
	rig      *Rig
	state    ActivityState
	variants map[string]map[string]int
	anim     AnimationsConfig
	mouth    frameLoop

	timeToBlink float64
	blinkT      float64
	blinking    bool
}

// NewCharacter creates a Character in the Idle state, driven by the manifest's
// variant mapping and animation configuration.
func NewCharacter(rig *Rig, cc CharacterConfig) *Character {
	mouthFPS := cc.Animations.Mouth.FPS
	if mouthFPS <= 0 {
		mouthFPS = 10
	}
	c := &Character{
		rig:      rig,
		state:    Idle,
		variants: cc.Variants,
		anim:     cc.Animations,
		mouth:    frameLoop{fps: mouthFPS, nframes: len(rig.parts[cc.Animations.Mouth.Part])},
	}
	if c.anim.Blink.IntervalSecs <= 0 {
		c.anim.Blink.IntervalSecs = 3.0
	}
	if c.anim.Blink.DurationSecs <= 0 {
		c.anim.Blink.DurationSecs = 0.15
	}
	c.timeToBlink = c.anim.Blink.IntervalSecs
	return c
}

// SetState switches the active activity state, resetting animated variants.
func (c *Character) SetState(s ActivityState) {
	if c.state == s {
		return
	}
	c.state = s
	c.mouth.reset()
	c.blinking = false
	c.blinkT = 0
	c.timeToBlink = c.anim.Blink.IntervalSecs
}

// Update advances the animation clocks by dt seconds (real elapsed time).
func (c *Character) Update(dt float64) {
	c.mouth.update(dt)
	if c.anim.Blink.Part == "" || c.state == Sleep {
		// No blink configured, or asleep (eyelid forced closed via variants).
		return
	}
	c.timeToBlink -= dt
	if c.timeToBlink <= 0 {
		c.blinking = true
		c.blinkT += dt
		if c.blinkT >= c.anim.Blink.DurationSecs {
			c.blinking = false
			c.timeToBlink = c.anim.Blink.IntervalSecs
			c.blinkT = 0
		}
	}
}

// variantIndex returns which variant image of a part to show for the current
// state. Resolution order: the manifest's per-state [character.variants]
// mapping, then the dynamic behaviors (mouth talk animation, blink). Variant 0
// is the default/calm variant for every part.
func (c *Character) variantIndex(part string) int {
	if m, ok := c.variants[part]; ok {
		if idx, ok := m[c.state.String()]; ok {
			return idx
		}
	}
	if part == c.anim.Mouth.Part && c.state == Talking {
		return c.mouth.index()
	}
	if part == c.anim.Blink.Part && c.blinking {
		return c.anim.Blink.ClosedVariant
	}
	return 0
}

// Draw composes the parts in manifest order with their offsets, applying the
// cursor look offset to the head and eye parts. Hidden variants (nil) are
// skipped.
func (c *Character) Draw(screen *ebiten.Image, lookX, lookY float64) {
	for _, part := range c.rig.order {
		imgs := c.rig.parts[part]
		if len(imgs) == 0 {
			continue
		}
		idx := c.variantIndex(part)
		if idx < 0 || idx >= len(imgs) {
			idx = 0
		}
		if imgs[idx] == nil {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		offs := c.rig.offsets[part]
		if len(offs) > 0 {
			off := offs[0]
			if idx < len(offs) {
				off = offs[idx]
			}
			lx, ly := 0.0, 0.0
			if part == "head" || part == "eye" {
				lx, ly = lookX, lookY
			}
			op.GeoM.Translate(float64(off.X)+lx, float64(off.Y)+ly)
		}
		screen.DrawImage(imgs[idx], op)
	}
}

// lookOffset maps a cursor position to a capped look offset (px) for the
// head/eye parts, so the character appears to track the mouse.
func lookOffset(cx, cy, w, h int, maxX, maxY float64) (float64, float64) {
	dx := float64(cx - w/2)
	dy := float64(cy - h/2)
	if dx == 0 && dy == 0 {
		return 0, 0
	}
	d := math.Hypot(dx, dy)
	return dx / d * maxX, dy / d * maxY
}
