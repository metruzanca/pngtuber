package main

import (
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenW = 192
	screenH = 192
)

// Game is the top-level Ebitengine game. Milestone 1: transparent window + test
// sprite. Subsystems (input, mic, activity, character controller) plug in here
// in later milestones.
type Game struct {
	cfg    *Config
	sheet  *ebiten.Image
	sprite *ebiten.Image
	angle  float64
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

	return &Game{
		cfg:    cfg,
		sheet:  sheet,
		sprite: sprite,
	}, nil
}

func (g *Game) Update() error {
	// Debug: Esc quits.
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	// Test sprite gently looks toward the cursor (placeholder for cursor
	// tracking in Milestone 5). Screen center -> cursor vector.
	cx, cy := ebiten.CursorPosition()
	g.angle = math.Atan2(float64(cy-screenH/2), float64(cx-screenW/2))
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(g.sprite.Bounds().Dx())/2, -float64(g.sprite.Bounds().Dy())/2)
	op.GeoM.Rotate(g.angle * 0.2)
	op.GeoM.Translate(screenW/2, screenH/2)
	screen.DrawImage(g.sprite, op)

	ebitenutil.DebugPrint(screen, "M1: transparent window")
}

func (g *Game) Layout(outsideW, outsideH int) (int, int) {
	return screenW, screenH
}

func main() {
	g, err := NewGame()
	if err != nil {
		log.Fatal(err)
	}

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
