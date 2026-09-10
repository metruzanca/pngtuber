// Edit mode: drag rig parts to reposition them and save the offsets back to
// the loaded manifest.toml (Milestone: manual rig tuning).
//
// Controls (edit mode active):
//
//	F2      toggle edit mode
//	Esc     exit edit mode (when active); otherwise quits as usual
//	S       save offsets to the manifest (Ctrl+S also works)
//	drag    left-mouse drag moves the part under the cursor; all of a part's
//	        variants shift together, preserving relative variant offsets.
//
// Saving rewrites only the [character.offsets] section of the manifest file,
// so comments and every other table are preserved.
package app

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// editState tracks the interactive rig editor.
type editState struct {
	active      bool
	dragging    string              // part grabbed by the drag, "" if none
	dragParts   []string            // all parts moved by the drag (the part's group)
	dragStart   image.Point         // mouse position when the drag began
	baseOffsets map[string][]Offset // each dragged part's offsets at drag start
	hovered     string              // part under the cursor (for the overlay)
}

// handleEdit updates edit mode each tick: toggling, dragging, and saving.
func (g *Game) handleEdit() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		g.setEditMode(!g.edit.active)
		return
	}
	if !g.edit.active {
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		if err := g.saveOffsets(); err != nil {
			log.Warnf("edit: save failed: %v", err)
		}
		return
	}

	mx, my := ebiten.CursorPosition()
	if part, ok := g.char.HitPart(mx, my); ok {
		g.edit.hovered = part
	} else {
		g.edit.hovered = ""
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.edit.dragging == "" {
			part, ok := g.char.HitPart(mx, my)
			if ok {
				g.edit.dragging = part
				g.edit.dragParts = g.char.PartGroup(part)
				g.edit.dragStart = image.Pt(mx, my)
				g.edit.baseOffsets = map[string][]Offset{}
				for _, p := range g.edit.dragParts {
					g.edit.baseOffsets[p] = cloneOffsets(g.char.rig.offsets[p])
				}
				log.Infof("edit: dragging %s (+%d linked)", part, len(g.edit.dragParts)-1)
			}
		}
		if g.edit.dragging != "" {
			dx := mx - g.edit.dragStart.X
			dy := my - g.edit.dragStart.Y
			for _, p := range g.edit.dragParts {
				g.char.rig.offsets[p] = shiftedOffsets(g.edit.baseOffsets[p], dx, dy)
			}
		}
	} else {
		g.edit.dragging = ""
		g.edit.dragParts = nil
		g.edit.baseOffsets = nil
	}
}

// setEditMode toggles the editor, pausing transient animations so parts can
// be dragged at their true positions. Exiting saves the current offsets.
func (g *Game) setEditMode(on bool) {
	g.edit.active = on
	g.edit.dragging = ""
	g.edit.baseOffsets = nil
	g.char.SetSteady(on)
	if on {
		log.Infof("edit: mode on — drag parts with the mouse, S to save offsets, Esc to exit")
		return
	}
	if err := g.saveOffsets(); err != nil {
		log.Warnf("edit: exit save failed: %v", err)
	} else {
		log.Infof("edit: mode off — offsets saved")
	}
}

// saveOffsets writes the current rig offsets into the manifest's
// [character.offsets] section, preserving the rest of the file. The built-in
// placeholder is embedded and read-only; installing a skin under the user data
// directory (or copying the placeholder there) makes a skin editable.
func (g *Game) saveOffsets() error {
	if g.embedded {
		return errors.New("the built-in placeholder is read-only; install a skin under the user data dir (see docs/custom-skins.md)")
	}
	if g.manifestPath == "" {
		return errors.New("no manifest path to save to")
	}
	if err := writeOffsets(g.manifestPath, g.char.rig.order, g.char.rig.offsets); err != nil {
		return err
	}
	log.Infof("edit: saved offsets to %s", g.manifestPath)
	return nil
}

// writeOffsets rewrites the [character.offsets] section of a manifest file,
// leaving all other content (comments, other tables) untouched.
func writeOffsets(path string, order []string, offsets map[string][]Offset) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "[character.offsets]" {
			start = i
			break
		}
	}
	if start < 0 {
		return fmt.Errorf("no [character.offsets] section in %s", path)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
			end = i
			break
		}
	}

	var b strings.Builder
	b.WriteString("[character.offsets]\n")
	for _, p := range order {
		offs := offsets[p]
		if len(offs) == 0 {
			continue
		}
		parts := make([]string, len(offs))
		for i, o := range offs {
			parts[i] = fmt.Sprintf("{ x = %d, y = %d }", o.X, o.Y)
		}
		fmt.Fprintf(&b, "%-8s = [%s]\n", p, strings.Join(parts, ", "))
	}
	block := strings.Split(strings.TrimSuffix(b.String(), "\n"), "\n")

	out := make([]string, 0, len(lines)+len(block))
	out = append(out, lines[:start]...) // everything before [character.offsets]
	out = append(out, block...)
	if end < len(lines) {
		out = append(out, "") // blank line before the next section
	}
	out = append(out, lines[end:]...)
	return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
}

func cloneOffsets(src []Offset) []Offset {
	out := make([]Offset, len(src))
	copy(out, src)
	return out
}

func shiftedOffsets(base []Offset, dx, dy int) []Offset {
	out := cloneOffsets(base)
	for i := range out {
		out[i].X += dx
		out[i].Y += dy
	}
	return out
}

// drawEditOverlay renders the edit-mode highlight and hints.
func (g *Game) drawEditOverlay(screen *ebiten.Image) {
	ebitenutil.DebugPrint(screen, "EDIT — drag parts · S save · Esc exit")

	part := g.edit.hovered
	if g.edit.dragging != "" {
		part = g.edit.dragging
	}
	if part == "" {
		return
	}
	// Highlight the whole drag group (e.g. eye + eyelid move together).
	for _, p := range g.char.PartGroup(part) {
		if r, ok := g.char.partRect(p); ok {
			highlightRect(screen, r)
		}
	}
	if r, ok := g.char.partRect(part); ok {
		offs := g.char.rig.offsets[part]
		off := offs[0]
		ebitenutil.DebugPrintAt(screen,
			fmt.Sprintf("%s (%d,%d)", part, off.X, off.Y),
			maxI(8, r.Min.X), maxI(8, r.Max.Y+2))
	}
}

// highlightRect outlines a rectangle with a bright border.
func highlightRect(screen *ebiten.Image, r image.Rectangle) {
	drawThickRect(screen, r.Min.X, r.Min.Y, r.Dx(), r.Dy(), 2)
}

// drawThickRect draws a filled 2px border using a 1x1 white image scaled.
func drawThickRect(screen *ebiten.Image, x, y, w, h, t int) {
	px := ebiten.NewImage(1, 1)
	px.Fill(color.RGBA{0, 255, 255, 255})
	border := func(bx, by, bw, bh int) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(bw), float64(bh))
		op.GeoM.Translate(float64(bx), float64(by))
		screen.DrawImage(px, op)
	}
	border(x, y, w, t)           // top
	border(x, y+h-t, w, t)       // bottom
	border(x, y+t, t, h-2*t)     // left
	border(x+w-t, y+t, t, h-2*t) // right
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
