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
//
// The character keeps two states: `raw` is the activity machine's current
// state (set instantly); `hands` is the *committed* state that drives the hand
// layout. The committed state only changes after `hand_move_delay_secs` of
// stability, so the mouse hand doesn't flicker between the mouse and keyboard
// during gaming pauses. Mouth (speech) and eyelid (sleep/blink) react to the
// raw state immediately.
type Character struct {
	rig      *Rig
	raw      ActivityState
	hands    ActivityState
	variants map[string]map[string]int
	anim     AnimationsConfig
	tracking TrackingConfig
	mouth    frameLoop

	handTimer   float64
	breathPhase float64
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
		raw:      Idle,
		hands:    Idle,
		variants: cc.Variants,
		anim:     cc.Animations,
		tracking: cc.Tracking,
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

// SetState records the activity machine's current state. The committed hand
// state lags behind per hand_move_delay_secs; mouth/eyelid react instantly.
func (c *Character) SetState(s ActivityState) {
	if c.raw == s {
		return
	}
	c.raw = s
	c.mouth.reset()
}

// Update advances the animation clocks by dt seconds (real elapsed time).
func (c *Character) Update(dt float64) {
	c.mouth.update(dt)
	if c.anim.Breathing.PeriodSecs > 0 {
		c.breathPhase += dt
	}

	// Hand-layout debounce: sleep entry/exit is instant; otherwise the hands
	// only relocate after the raw state has been stable for the delay.
	delay := c.anim.HandMoveDelaySecs
	switch {
	case c.raw == Sleep || c.hands == Sleep:
		c.hands = c.raw
		c.handTimer = 0
	case c.raw != c.hands:
		c.handTimer += dt
		if delay <= 0 || c.handTimer >= delay {
			c.hands = c.raw
			c.handTimer = 0
		}
	default:
		c.handTimer = 0
	}

	if c.anim.Blink.Part == "" || c.raw == Sleep {
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

// variantIndex returns which variant image of a part to show. Resolution
// order: the manifest's per-state [character.variants] mapping against the
// committed hand state (raw state for the mouth and eyelid parts), then the
// dynamic behaviors (mouth talk animation, blink). Variant 0 is the
// default/calm variant for every part.
func (c *Character) variantIndex(part string) int {
	if part == c.anim.Mouth.Part && c.raw == Talking {
		return c.mouth.index()
	}
	state := c.hands
	if part == c.anim.Blink.Part {
		state = c.raw // eyelid responds to sleep instantly
	}
	if m, ok := c.variants[part]; ok {
		if idx, ok := m[state.String()]; ok {
			return idx
		}
	}
	if part == c.anim.Blink.Part && c.blinking {
		return c.anim.Blink.ClosedVariant
	}
	return 0
}

// Draw composes the parts in manifest order with their offsets, applying the
// cursor look offset to the head and eye parts, the breathing scale, and the
// mouse tracking offset to tracked parts while the mouse is active. Hidden
// variants (nil) are skipped.
func (c *Character) Draw(screen *ebiten.Image, lookX, lookY, trackX, trackY float64) {
	breathe := map[string]bool{}
	for _, p := range c.anim.Breathing.Parts {
		breathe[p] = true
	}
	track := map[string]bool{}
	for _, p := range c.tracking.Parts {
		track[p] = true
	}
	mouseActive := c.hands == Mouse || c.hands == Gaming

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
		offs := c.rig.offsets[part]
		if len(offs) == 0 {
			continue
		}
		off := offs[0]
		if idx < len(offs) {
			off = offs[idx]
		}

		op := &ebiten.DrawImageOptions{}
		w, h := imgs[idx].Bounds().Dx(), imgs[idx].Bounds().Dy()
		if breathe[part] && c.anim.Breathing.PeriodSecs > 0 {
			scale := 1 + c.anim.Breathing.Amplitude*math.Sin(2*math.Pi*c.breathPhase/c.anim.Breathing.PeriodSecs)
			op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(float64(w)/2, float64(h)/2)
		}
		lx, ly := 0.0, 0.0
		if part == "head" || part == "eye" {
			lx, ly = lookX, lookY
		}
		tx, ty := 0.0, 0.0
		if track[part] && mouseActive {
			tx, ty = trackX, trackY
		}
		op.GeoM.Translate(float64(off.X)+lx+tx, float64(off.Y)+ly+ty)
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
