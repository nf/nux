package varvara

import (
	"github.com/hajimehoshi/ebiten/v2"
)

type gui struct {
	*Varvara

	w, h   int
	fg, bg *ebiten.Image
}

func (v *gui) update() {
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
	}
	if v.scr.fg != nil {
		v.fg.WritePixels(v.scr.fg.buf)
	}
	if v.scr.bg != nil {
		v.bg.WritePixels(v.scr.bg.buf)
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
