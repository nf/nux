package varvara

import (
	"github.com/hajimehoshi/ebiten/v2"
)

func newGUI(v *Varvara) *gui {
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	g := &gui{Varvara: v}
	g.update()
	return g
}

func (v *gui) Run() error { return ebiten.RunGame(v) }

type gui struct {
	*Varvara

	w, h   int
	fg, bg *ebiten.Image
	ops    int // updated to match v.scr.ops after copying fg/bg
}

func (v *gui) update() {
	// Controller
	v.cntrl.SetButtons(
		ebiten.IsKeyPressed(ebiten.KeyControl), // A
		ebiten.IsKeyPressed(ebiten.KeyAlt),     // B
		ebiten.IsKeyPressed(ebiten.KeyShift),   // Select
		ebiten.IsKeyPressed(ebiten.KeyHome),    // Start
		ebiten.IsKeyPressed(ebiten.KeyUp),
		ebiten.IsKeyPressed(ebiten.KeyDown),
		ebiten.IsKeyPressed(ebiten.KeyLeft),
		ebiten.IsKeyPressed(ebiten.KeyRight))

	// Mouse
	mx, my := ebiten.CursorPosition()
	wx, wy := ebiten.Wheel()
	v.mouse.Set(
		clampInt16(mx), clampInt16(my),
		clampInt16(int(wx*10)), clampInt16(int(wy*10)),
		ebiten.IsMouseButtonPressed(ebiten.MouseButton0),
		ebiten.IsMouseButtonPressed(ebiten.MouseButton1),
		ebiten.IsMouseButtonPressed(ebiten.MouseButton2))

	// Screen
	v.w, v.h = int(v.scr.Width()), int(v.scr.Height())
	if v.w == 0 {
		v.w = 160
	}
	if v.h == 0 {
		v.h = 120
	}
	if v.fg == nil || v.fg.Bounds().Dx() != v.w || v.fg.Bounds().Dy() != v.h {
		v.fg = ebiten.NewImage(v.w, v.h)
		v.bg = ebiten.NewImage(v.w, v.h)
		v.ops = -1
	}
	if o := v.scr.ops; v.ops != o {
		v.ops = o
		if m := v.scr.fg; m != nil && m.w == v.w && m.h == v.h {
			v.fg.WritePixels(m.buf)
		}
		if m := v.scr.bg; m != nil && m.w == v.w && m.h == v.h {
			v.bg.WritePixels(m.buf)
		}
	}
}

func (v *gui) Update() error {
	select {
	case <-v.sys.Done:
		return ebiten.Termination
	case <-v.guiUpdate:
		// This is the only safe time to access Varvara state;
		// at all other times it may be mutated by the program.
		v.update()
		v.guiUpdateDone <- true
		return nil
	default:
		// cpu is busy with other things.
		return nil
	}
}

func (v *gui) Draw(screen *ebiten.Image) {
	screen.DrawImage(v.bg, nil)
	screen.DrawImage(v.fg, nil)
}

func (v *gui) Layout(outerWidth, outerHeight int) (screenWidth, screenHeight int) {
	return v.w, v.h
}

func clampInt16(v int) int16 {
	const max, min = 32767, -32768
	switch {
	case v > max:
		return max
	case v < min:
		return min
	default:
		return int16(v)
	}
}
