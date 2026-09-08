package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/metruzanca/pngtuber/input"
)

const (
	screenW = 192
	screenH = 192
)

// Game is the top-level Ebitengine game. Milestones 1-4 wired the transparent
// window, global input, mic, and the activity state machine; Milestone 5 added
// the rig character; Milestone 6 made the manifest fully data-driven; M7 loads
// the manifest from a user data dir (env/flag/XDG) or the placeholder.
type Game struct {
	cfg   *Config
	char  *Character
	input input.Backend
	sigs  input.ActivitySignals
	mic   *Mic
	act   *Activity

	micLevel   float64
	micTalking bool
	lastMicLog time.Time
	prevKey    int
	prevMouse  int
	state      ActivityState
	lastFrame  time.Time
}

func NewGame(assetsFlag, skinFlag string) (*Game, error) {
	res := ResolveAssets(assetsFlag, skinFlag)
	log.Printf("assets: loading %s", res.Source)
	cfg, err := LoadConfig(res.ManifestPath)
	if err != nil {
		return nil, err
	}

	rig, err := LoadRig(cfg, res.BaseDir)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	g := &Game{
		cfg:       cfg,
		char:      NewCharacter(rig, cfg.Character),
		act:       NewActivity(cfg.Activity, now),
		lastFrame: now,
	}
	g.input = input.NewBackend()
	g.mic = NewMic()
	g.act.SetOnChange(func(s ActivityState) {
		log.Printf("activity: state -> %s", s)
		g.char.SetState(s)
	})
	return g, nil
}

func (g *Game) Update() error {
	now := time.Now()
	dt := now.Sub(g.lastFrame).Seconds()
	g.lastFrame = now

	// Debug: Esc quits.
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	g.pollInput()
	g.updateMic()
	g.updateActivity()
	g.char.Update(dt)
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
// state transitions and a throttled periodic level.
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
// talking state into the state machine. Transitions are logged and forwarded
// to the character via the callback registered in NewGame.
func (g *Game) updateActivity() {
	now := time.Now()
	dk := g.sigs.KeyCount - g.prevKey
	dm := g.sigs.MouseCount - g.prevMouse
	g.prevKey = g.sigs.KeyCount
	g.prevMouse = g.sigs.MouseCount
	g.state = g.act.Tick(now, dk, dm, g.micTalking)
}

func (g *Game) Draw(screen *ebiten.Image) {
	cw, ch := g.canvasSize()
	cx, cy := ebiten.CursorPosition()
	lx, ly := lookOffset(cx, cy, cw, ch, g.cfg.Character.Look.MaxX, g.cfg.Character.Look.MaxY)
	tx, ty := lookOffset(cx, cy, cw, ch, g.cfg.Character.Tracking.MaxX, g.cfg.Character.Tracking.MaxY)
	g.char.Draw(screen, lx, ly, tx, ty)

	talking := "no"
	if g.micTalking {
		talking = "YES"
	}
	ebitenutil.DebugPrint(screen, fmt.Sprintf("%-6s k=%d m=%d mic=%.2f talk=%s",
		g.state, g.sigs.KeyCount, g.sigs.MouseCount, g.micLevel, talking))
}

// canvasSize returns the rig's composition canvas, falling back to the fixed
// default when the manifest has no parts.
func (g *Game) canvasSize() (int, int) {
	if w, h := g.char.rig.Canvas(); w > 0 {
		return w, h
	}
	return screenW, screenH
}

func (g *Game) Layout(outsideW, outsideH int) (int, int) {
	return g.canvasSize()
}

func main() {
	assetsFlag := flag.String("assets", "", "assets directory (overrides PNGTUBER_ASSETS)")
	skinFlag := flag.String("skin", "", "skin name under <assets>/skins (overrides PNGTUBER_SKIN)")
	flag.Parse()

	g, err := NewGame(*assetsFlag, *skinFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer g.input.Close()
	defer g.mic.Close()

	w, h := g.canvasSize()
	ebiten.SetWindowSize(w, h)
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
