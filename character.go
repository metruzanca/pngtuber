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
	"image"
	_ "image/png"
	"io/fs"
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
// filenames against baseDir on the OS filesystem. The hiddenVariant marker
// ("-") maps to a nil image that Draw skips. Offsets are normalized so the rig
// starts at (0,0) and the canvas is sized to the parts' bounding box.
func LoadRig(cfg *Config, baseDir string) (*Rig, error) {
	return loadRig(cfg, func(f string) (*ebiten.Image, error) {
		img, _, err := ebitenutil.NewImageFromFile(filepath.Join(baseDir, f))
		return img, err
	})
}

// LoadRigFS loads the rig from an fs.FS (used for the embedded placeholder);
// baseDir is relative to the root of the FS.
func LoadRigFS(fsys fs.FS, cfg *Config, baseDir string) (*Rig, error) {
	return loadRig(cfg, func(f string) (*ebiten.Image, error) {
		rc, err := fsys.Open(filepath.Join(baseDir, f))
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		img, _, err := ebitenutil.NewImageFromReader(rc)
		return img, err
	})
}

// LoadRigFrom loads the rig from res, reading from the embedded placeholder
// FS when res.FS is non-nil, otherwise from the OS filesystem.
func LoadRigFrom(res AssetDirs, cfg *Config) (*Rig, error) {
	if res.FS != nil {
		return LoadRigFS(res.FS, cfg, res.BaseDir)
	}
	return LoadRig(cfg, res.BaseDir)
}

// loadRig is the shared rig loader: it builds the parts table by calling load
// for each part variant filename.
func loadRig(cfg *Config, load func(string) (*ebiten.Image, error)) (*Rig, error) {
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
			img, err := load(f)
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
	rig        *Rig
	raw        ActivityState
	hands      ActivityState
	variants   map[string]map[string]int
	anim       AnimationsConfig
	tracking   TrackingConfig
	look       LookConfig
	visibility VisibilityConfig
	groups     map[string][]string
	mouth      frameLoop

	handTimer   float64
	breathPhase float64
	timeToBlink float64
	blinkT      float64
	blinking    bool

	// press animation state
	pressIndex int
	pressTimer map[string]float64
	steady     bool // edit mode: pause breathing for stable dragging

	// mouthOn is the live mic "in threshold" flag: the lip flap stops the
	// instant audio drops below the threshold, independent of the held Talking
	// state and mic hysteresis.
	mouthOn bool

	// mouse hand tracking (driven by global relative motion, focus-independent)
	mouseOffX   float64
	mouseOffY   float64
	mouseActive bool
	mouseIdle   float64
}

// NewCharacter creates a Character in the Idle state, driven by the manifest's
// variant mapping and animation configuration.
func NewCharacter(rig *Rig, cc CharacterConfig) *Character {
	mouthFPS := cc.Animations.Mouth.FPS
	if mouthFPS <= 0 {
		mouthFPS = 10
	}
	c := &Character{
		rig:        rig,
		raw:        Idle,
		hands:      Idle,
		variants:   cc.Variants,
		anim:       cc.Animations,
		tracking:   cc.Tracking,
		look:       cc.Look,
		visibility: cc.Visibility,
		groups:     cc.Groups,
		pressTimer: map[string]float64{},
		mouth:      frameLoop{fps: mouthFPS, nframes: len(rig.parts[cc.Animations.Mouth.Part])},
	}
	if c.anim.Blink.IntervalSecs <= 0 {
		c.anim.Blink.IntervalSecs = 3.0
	}
	if c.anim.Blink.DurationSecs <= 0 {
		c.anim.Blink.DurationSecs = 0.15
	}
	if c.anim.Press.DurationSecs <= 0 {
		c.anim.Press.DurationSecs = 0.1
	}
	if c.anim.Press.Variant <= 0 {
		c.anim.Press.Variant = 1
	}
	if c.tracking.Sensitivity <= 0 {
		c.tracking.Sensitivity = 0.02
	}
	c.timeToBlink = c.anim.Blink.IntervalSecs
	return c
}

// SetSteady pauses transient animations (breathing, mouse tracking) for edit
// mode so parts can be dragged at their true positions.
func (c *Character) SetSteady(steady bool) {
	c.steady = steady
	if steady {
		c.mouseOffX = 0
		c.mouseOffY = 0
	}
}

// UpdateMouse integrates global relative mouse motion into the tracked parts'
// offset while the mouse is active, otherwise the offset springs back to rest.
//
// The mouse hand turns ON instantly with any mouse motion (or a Mouse/Gaming
// state) and turns OFF only after hand_move_delay_secs of none — so it appears
// the moment the real mouse moves (even mid-typing) instead of waiting for the
// debounced committed layout, while still avoiding flicker during brief gaming
// pauses.
func (c *Character) UpdateMouse(dx, dy int, dt float64) {
	if dx != 0 || dy != 0 || c.raw == Mouse || c.raw == Gaming {
		c.mouseIdle = 0
		c.mouseActive = true
	} else {
		c.mouseIdle += dt
		delay := c.anim.HandMoveDelaySecs
		if delay <= 0 {
			delay = 0.5
		}
		if c.mouseIdle >= delay {
			c.mouseActive = false
		}
	}

	if len(c.tracking.Parts) == 0 {
		return
	}
	if c.mouseActive {
		c.mouseOffX += float64(dx) * c.tracking.Sensitivity
		c.mouseOffY += float64(dy) * c.tracking.Sensitivity
		c.mouseOffX = clampF(c.mouseOffX, -c.tracking.MaxX, c.tracking.MaxX)
		c.mouseOffY = clampF(c.mouseOffY, -c.tracking.MaxY, c.tracking.MaxY)
		return
	}
	// Mouse hand is resting: ease back to center.
	decay := 1 - 8*dt
	if decay < 0 {
		decay = 0
	}
	c.mouseOffX *= decay
	c.mouseOffY *= decay
	if math.Abs(c.mouseOffX) < 0.01 {
		c.mouseOffX = 0
	}
	if math.Abs(c.mouseOffY) < 0.01 {
		c.mouseOffY = 0
	}
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// stretchGeoM builds the transform that keeps a part's top row fixed at its
// base position and stretches it so its bottom edge reaches (dx, dy) — a
// perspective-style reach toward the mouse. It maps IMAGE coordinates to the
// canvas: translate the image top-center to the origin, apply a horizontal
// skew (sx = dx/h) + vertical scale (sy = 1 + dy/h), then translate to the
// part's base top-center. The whole top row is pinned; the bottom row slides
// uniformly by (dx, dy).
func stretchGeoM(off Offset, w, h int, dx, dy float64) ebiten.GeoM {
	sx := dx / float64(h)
	sy := 1 + dy/float64(h)
	if sy < 0.05 {
		sy = 0.05
	}
	var m ebiten.GeoM
	m.Translate(-float64(w)/2, 0)
	m.Skew(sx, 0)
	m.Scale(1, sy)
	m.Translate(float64(off.X)+float64(w)/2, float64(off.Y))
	return m
}

// Press records key events: each one taps the next press part (alternating)
// down for press.duration_secs.
func (c *Character) Press(keys int) {
	if keys <= 0 || len(c.anim.Press.Parts) == 0 {
		return
	}
	const maxPerTick = 8
	if keys > maxPerTick {
		keys = maxPerTick
	}
	for i := 0; i < keys; i++ {
		c.pressIndex = (c.pressIndex + 1) % len(c.anim.Press.Parts)
		part := c.anim.Press.Parts[c.pressIndex]
		c.pressTimer[part] = c.anim.Press.DurationSecs
	}
}

// SetState records the activity machine's current state. The committed hand
// state lags behind per hand_move_delay_secs; mouth/eyelid react instantly.
func (c *Character) SetState(s ActivityState) {
	if c.raw == s {
		return
	}
	c.raw = s
}

// SetTalking updates the live mic "in threshold" flag that drives the mouth
// animation. The lip flap starts as soon as the level is at/above the
// threshold and stops immediately when it drops below — it does not wait for
// the activity state machine's held Talking state or mic hysteresis.
func (c *Character) SetTalking(on bool) {
	if on && !c.mouthOn {
		c.mouth.reset()
	}
	c.mouthOn = on
}

// Update advances the animation clocks by dt seconds (real elapsed time).
func (c *Character) Update(dt float64) {
	c.mouth.update(dt)
	if !c.steady && c.anim.Breathing.PeriodSecs > 0 {
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

	// Decay press timers.
	for part, t := range c.pressTimer {
		t -= dt
		if t <= 0 {
			delete(c.pressTimer, part)
		} else {
			c.pressTimer[part] = t
		}
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
// order: the press animation (hands tapping), then the manifest's per-state
// [character.variants] mapping against the committed hand state (raw state for
// the mouth and eyelid parts), then the dynamic behaviors (mouth talk
// animation, blink). Variant 0 is the default/calm variant for every part.
func (c *Character) variantIndex(part string) int {
	if c.pressTimer[part] > 0 {
		return c.anim.Press.Variant
	}
	if part == c.anim.Mouth.Part && c.mouthOn {
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

// partRect returns the drawn rectangle (top-left origin) of a part's current
// variant, or ok=false if the part has no visible image.
func (c *Character) partRect(part string) (image.Rectangle, bool) {
	imgs := c.rig.parts[part]
	if len(imgs) == 0 {
		return image.Rectangle{}, false
	}
	idx := c.variantIndex(part)
	if idx < 0 || idx >= len(imgs) || imgs[idx] == nil {
		return image.Rectangle{}, false
	}
	offs := c.rig.offsets[part]
	if len(offs) == 0 {
		return image.Rectangle{}, false
	}
	off := offs[0]
	if idx < len(offs) {
		off = offs[idx]
	}
	b := imgs[idx].Bounds()
	return image.Rect(off.X, off.Y, off.X+b.Dx(), off.Y+b.Dy()), true
}

// HitPart returns the top-most part under (x, y), in draw order.
func (c *Character) HitPart(x, y int) (string, bool) {
	for i := len(c.rig.order) - 1; i >= 0; i-- {
		part := c.rig.order[i]
		if r, ok := c.partRect(part); ok && x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y {
			return part, true
		}
	}
	return "", false
}

// PartGroup returns the parts that move together with part when dragged in
// edit mode (the manifest [character.groups] containing it, or just itself).
func (c *Character) PartGroup(part string) []string {
	for _, members := range c.groups {
		for _, m := range members {
			if m == part {
				return members
			}
		}
	}
	return []string{part}
}

// partVisible reports whether a part should be drawn. In edit mode everything
// is visible so both hand poses can be positioned; otherwise the mouse hand
// and the keyboard hand (the same physical hand) are mutually exclusive based
// on the tracked mouse activity (instant-on, delayed-off).
func (c *Character) partVisible(part string) bool {
	if c.steady {
		return true
	}
	for _, p := range c.visibility.MouseHand {
		if p == part {
			return c.mouseActive
		}
	}
	for _, p := range c.visibility.KeyboardHand {
		if p == part {
			return !c.mouseActive
		}
	}
	return true
}

// ShiftPart moves every variant of a part by (dx, dy).
func (c *Character) ShiftPart(part string, dx, dy int) {
	for i := range c.rig.offsets[part] {
		c.rig.offsets[part][i].X += dx
		c.rig.offsets[part][i].Y += dy
	}
}

// Draw composes the parts in manifest order with their offsets, applying the
// cursor look offset to configured parts, the breathing scale, and the mouse
// tracking offset (from UpdateMouse) to tracked parts while the mouse is
// active. Hidden variants (nil) are skipped.
func (c *Character) Draw(screen *ebiten.Image, lookX, lookY float64) {
	breathe := map[string]bool{}
	for _, p := range c.anim.Breathing.Parts {
		breathe[p] = true
	}
	track := map[string]bool{}
	for _, p := range c.tracking.Parts {
		track[p] = true
	}
	stretch := map[string]bool{}
	for _, p := range c.tracking.Stretch {
		stretch[p] = true
	}
	look := map[string]bool{}
	for _, p := range c.look.Parts {
		look[p] = true
	}

	for _, part := range c.rig.order {
		if !c.partVisible(part) {
			continue
		}
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
		if breathe[part] && !c.steady && c.anim.Breathing.PeriodSecs > 0 {
			scale := 1 + c.anim.Breathing.Amplitude*math.Sin(2*math.Pi*c.breathPhase/c.anim.Breathing.PeriodSecs)
			op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(float64(w)/2, float64(h)/2)
		}
		lx, ly := 0.0, 0.0
		if look[part] {
			lx, ly = lookX, lookY
		}
		tx, ty := 0.0, 0.0
		switch {
		case track[part] && c.mouseActive && !c.steady && stretch[part]:
			// The hand keeps its top row fixed and stretches toward the mouse.
			op.GeoM.Concat(stretchGeoM(off, w, h, c.mouseOffX, c.mouseOffY))
		case track[part] && c.mouseActive && !c.steady:
			tx, ty = c.mouseOffX, c.mouseOffY
			op.GeoM.Translate(float64(off.X)+lx+tx, float64(off.Y)+ly+ty)
		default:
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
