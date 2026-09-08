// Character rig composition and state->part animation controller (Milestone 5).
//
// Parts are composited PNGs (body, head, eye, eyelid, hands, mouth) loaded
// from the manifest with offsets/z-order. Each part can have several variants
// (e.g. hand up/down, mouth-open frames); the controller picks a variant per
// activity state and animates variant sequences by accumulated elapsed time so
// the manifest's fps is honored regardless of the game's TPS. The head/eye
// parts track the cursor via a capped look offset.
package main

import (
	"fmt"
	_ "image/png"
	"math"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	// blinkInterval is the seconds between blinks; blinkDuration how long the
	// eyelid stays closed during a blink.
	blinkInterval = 3.0
	blinkDuration = 0.15
	// mouthFPS drives the talk mouth-open frames while talking.
	mouthFPS = 10.0
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
	offsets map[string]Offset
	look    LookConfig
}

// LoadRig loads every part variant from the manifest, resolving relative
// filenames against baseDir.
func LoadRig(cfg *Config, baseDir string) (*Rig, error) {
	r := &Rig{
		order:   cfg.Character.PartsOrder,
		parts:   make(map[string][]*ebiten.Image, len(cfg.Character.Parts)),
		offsets: cfg.Character.Offsets,
		look:    cfg.Character.Look,
	}
	for name, files := range cfg.Character.Parts {
		for _, f := range files {
			img, _, err := ebitenutil.NewImageFromFile(filepath.Join(baseDir, f))
			if err != nil {
				return nil, fmt.Errorf("load part %q (%s): %w", name, f, err)
			}
			r.parts[name] = append(r.parts[name], img)
		}
	}
	return r, nil
}

// Character composes the rig parts and switches variants by activity state.
type Character struct {
	rig         *Rig
	state       ActivityState
	mouth       frameLoop
	timeToBlink float64
	blinkT      float64
	blinking    bool
}

// NewCharacter creates a Character in the Idle state.
func NewCharacter(rig *Rig) *Character {
	return &Character{
		rig:         rig,
		state:       Idle,
		timeToBlink: blinkInterval,
		mouth:       frameLoop{fps: mouthFPS, nframes: len(rig.parts["mouth"])},
	}
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
	c.timeToBlink = blinkInterval
}

// Update advances the animation clocks by dt seconds (real elapsed time).
func (c *Character) Update(dt float64) {
	c.mouth.update(dt)
	if c.state == Sleep {
		// Eyelid is forced closed by variantIndex; no blinking needed.
		return
	}
	c.timeToBlink -= dt
	if c.timeToBlink <= 0 {
		c.blinking = true
		c.blinkT += dt
		if c.blinkT >= blinkDuration {
			c.blinking = false
			c.timeToBlink = blinkInterval
			c.blinkT = 0
		}
	}
}

// variantIndex returns which variant image of a part to show for the current
// state. Variant 0 is the default/calm variant for every part.
func (c *Character) variantIndex(part string) int {
	switch part {
	case "left": // 0=up, 1=down
		if c.state == Typing || c.state == Gaming {
			return 1
		}
	case "right": // 0=up, 1=down
		if c.state == Mouse || c.state == Gaming {
			return 1
		}
	case "eyelid": // 0=open, 1=closed
		if c.state == Sleep || c.blinking {
			return 1
		}
	case "mouth": // 0=closed, 1..n talk frames
		if c.state == Talking {
			return c.mouth.index()
		}
		return 0
	}
	return 0
}

// Draw composes the parts in manifest order with their offsets, applying the
// cursor look offset to the head and eye parts.
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
		off := c.rig.offsets[part]
		op := &ebiten.DrawImageOptions{}
		lx, ly := 0.0, 0.0
		if part == "head" || part == "eye" {
			lx, ly = lookX, lookY
		}
		op.GeoM.Translate(float64(off.X)+lx, float64(off.Y)+ly)
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
