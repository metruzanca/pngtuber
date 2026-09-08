package main

import (
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/metruzanca/pngtuber/input"
)

const (
	screenW = 192
	screenH = 192
)

// Game is the top-level Ebitengine game. Milestone 1: transparent window + test
// sprite. Milestone 2: global input pipeline (evdev -> ActivitySignals).
// Milestone 3: mic capture (pulse -> smoothed MicLevel + talking detection).
// Milestone 4: activity state machine merging input + mic with idle/sleep.
// The character controller plugs in on top in Milestone 5.
type Game struct {
	cfg    *Config
	sheet  *ebiten.Image
	sprite *ebiten.Image
	angle  float64
	input  input.Backend
	sigs   input.ActivitySignals
	mic    *Mic
	act    *Activity

	micLevel   float64
	micTalking bool
	lastMicLog time.Time
	prevKey    int
	prevMouse  int
	state      ActivityState
}

func NewGame() (*Game, error) {
	cfg, err := LoadConfig("assets/character/manifest.toml")
	if err != nil {
		return nil, err
	}

	sheet, _, err := ebitenutil.NewImageFromFile(cfg.Character.Spritesheet)
	if err != nil {
		return nil, fmt.Errorf("load spritesheet: %w", err)
	}

	// Test sprite: a single cell from the sheet. In later milestones this is the
	// per-state frame controller; for now just draw the first cell.
	fw, fh := cfg.Character.FrameSize.Width, cfg.Character.FrameSize.Height
	sprite := sheet.SubImage(image.Rect(0, 0, fw, fh)).(*ebiten.Image)

	g := &Game{
		cfg:    cfg,
		sheet:  sheet,
		sprite: sprite,
	}
	g.input = input.NewBackend()
	g.mic = NewMic()
	g.act = NewActivity(cfg.Activity, time.Now())
	g.act.SetOnChange(func(s ActivityState) {
		log.Printf("activity: state -> %s", s)
	})
	return g, nil
}

func (g *Game) Update() error {
	// Debug: Esc quits.
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	g.pollInput()
	g.updateMic()
	g.updateActivity()

	// Test sprite gently looks toward the cursor (placeholder for cursor
	// tracking in Milestone 5). Screen center -> cursor vector.
	cx, cy := ebiten.CursorPosition()
	g.angle = math.Atan2(float64(cy-screenH/2), float64(cx-screenW/2))
	return nil
}

// pollInput drains the global input backend into ActivitySignals each tick
// and logs a one-line summary whenever activity was observed.
func (g *Game) pollInput() {
	n := g.sigs.Drain(g.input.Signals())
	if n > 0 {
		log.Printf("input: drained %d event(s) this tick — totals key=%d mouse=%d",
			n, g.sigs.KeyCount, g.sigs.MouseCount)
	}
}

// updateMic samples the smoothed loudness into the game state, logs talking
// state transitions and a throttled periodic level, and exposes the result on
// screen.
func (g *Game) updateMic() {
	if g.mic == nil {
		return
	}
	g.micLevel = g.mic.Level()
	talking := g.mic.Talking(g.cfg.Activity.MicThreshold, g.cfg.Activity.MicHysteresis)
	if talking != g.micTalking {
		log.Printf("mic: talking=%v level=%.3f (threshold=%.2f)",
			talking, g.micLevel, g.cfg.Activity.MicThreshold)
	}
	g.micTalking = talking
	if time.Since(g.lastMicLog) >= 2*time.Second {
		log.Printf("mic: level=%.3f talking=%v", g.micLevel, talking)
		g.lastMicLog = time.Now()
	}
}

// updateActivity feeds this tick's per-tick key/mouse deltas plus the mic
// talking state into the state machine. Transitions are logged via the
// callback registered in NewGame.
func (g *Game) updateActivity() {
	now := time.Now()
	dk := g.sigs.KeyCount - g.prevKey
	dm := g.sigs.MouseCount - g.prevMouse
	g.prevKey = g.sigs.KeyCount
	g.prevMouse = g.sigs.MouseCount
	g.state = g.act.Tick(now, dk, dm, g.micTalking)
}

func (g *Game) Draw(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(g.sprite.Bounds().Dx())/2, -float64(g.sprite.Bounds().Dy())/2)
	op.GeoM.Rotate(g.angle * 0.2)
	op.GeoM.Translate(screenW/2, screenH/2)
	screen.DrawImage(g.sprite, op)

	talking := "no"
	if g.micTalking {
		talking = "YES"
	}
	ebitenutil.DebugPrint(screen, fmt.Sprintf("M4: %-6s k=%d m=%d mic=%.2f talk=%s",
		g.state, g.sigs.KeyCount, g.sigs.MouseCount, g.micLevel, talking))
}

func (g *Game) Layout(outsideW, outsideH int) (int, int) {
	return screenW, screenH
}

func main() {
	g, err := NewGame()
	if err != nil {
		log.Fatal(err)
	}
	defer g.input.Close()
	defer g.mic.Close()

	ebiten.SetWindowSize(screenW, screenH)
	ebiten.SetWindowTitle("pngtuber")
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(false)
	ebiten.SetRunnableOnUnfocused(true)

	// Run transparent so OBS Game Capture / PipeWire can capture full alpha.
	// X11 class/instance names make the window show up distinctly in OBS's
	// window list (WM_CLASS) on X11 and in the Wayland portal picker.
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{
		ScreenTransparent: true,
		X11ClassName:      "pngtuber",
		X11InstanceName:   "pngtuber",
	}); err != nil {
		log.Fatal(err)
	}
}
