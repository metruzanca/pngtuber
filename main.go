package main

import (
	"flag"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/metruzanca/pngtuber/internal/app"
)

// version is injected at build time via goreleaser ldflags (default "dev").
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	assetsFlag := flag.String("assets", "", "assets directory (overrides PNGTUBER_ASSETS)")
	skinFlag := flag.String("skin", "", "skin name under <assets>/skins (overrides PNGTUBER_SKIN)")
	debugFlag := flag.Bool("debug", false, "show the in-window status overlay and verbose input/mic logs")
	versionFlag := flag.Bool("version", false, "print the pngtuber version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("pngtuber %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	log.Infof("pngtuber %s — input-reactive desktop avatar", version)
	if *debugFlag {
		log.SetLevel(log.DebugLevel)
	}

	g, err := app.NewGame(*assetsFlag, *skinFlag, *debugFlag)
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	w, h := g.CanvasSize()
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
