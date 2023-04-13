package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

type gui struct {
	*Varvara

	theme         [4]color.RGBA
	width, height int
	pending       []DrawOp
	fg, bg        *ebiten.Image
}

func (v *gui) update() {
	var (
		r = v.sys.Red()
		g = v.sys.Green()
		b = v.sys.Blue()
	)
	v.theme = [4]color.RGBA{
		{R: uint8(r & 0xf000 >> 8), G: uint8(g & 0xf000 >> 8), B: uint8(b & 0xf000 >> 8), A: 0xff},
		{R: uint8(r & 0x0f00 >> 4), G: uint8(g & 0x0f00 >> 4), B: uint8(b & 0x0f00 >> 4), A: 0xff},
		{R: uint8(r & 0x00f0), G: uint8(g & 0x00f0), B: uint8(b & 0x00f0), A: 0xff},
		{R: uint8(r & 0x000f << 4), G: uint8(g & 0x000f << 4), B: uint8(b & 0x000f << 4), A: 0xff},
	}
	v.width = int(v.scr.Width())
	v.height = int(v.scr.Height())
	if v.width == 0 {
		v.width = 160
	}
	if v.height == 0 {
		v.height = 120
	}
	if v.fg == nil || v.fg.Bounds().Dx() != v.width || v.fg.Bounds().Dy() != v.height {
		v.fg = ebiten.NewImage(v.width, v.height)
		v.bg = ebiten.NewImage(v.width, v.height)
	}
	v.pending = append(v.pending, v.scr.pending...)
	v.scr.pending = v.scr.pending[:0]
}

func (v *gui) Update() error {
	select {
	case <-v.sys.Done:
		return ebiten.Termination
	case <-v.guiUpdate:
	}

	// This is the only safe time to access Varvara state;
	// at all other times it may be mutated by the program.
	v.update()

	v.guiUpdateDone <- true
	return nil
}

func (v *gui) Draw(screen *ebiten.Image) {
	for i := range v.pending {
		op := &v.pending[i]

		var (
			m     *ebiten.Image
			x, y  = int(op.X), int(op.Y)
			flipX = op.Byte&0x10 != 0
			flipY = op.Byte&0x20 != 0
		)
		if op.Byte&0x40 == 0 {
			m = v.bg
		} else {
			m = v.fg
		}
		if op.Sprite {
			if !flipX {
				x += 7
			}
			if flipY {
				y += 7
			}
			for j := 0; j < 8; j++ {
				pxA := op.SpriteData[j]
				pxB := op.SpriteData[j+8]
				for i := 0; i < 8; i++ {
					c := pxA & 0x1
					if op.Byte&0x80 != 0 {
						c += pxB & 0x1 << 1
					}
					c = drawBlendingModes[c][op.Byte&0x0f]
					m.Set(x, y, v.theme[c])
					pxA >>= 1
					pxB >>= 1
					if flipX {
						x++
					} else {
						x--
					}
				}
				if flipX {
					x -= 8
				} else {
					x += 8
				}
				if flipY {
					y--
				} else {
					y++
				}
			}
		} else { // pixel
			// TODO: fill
			m.Set(x, y, v.theme[op.Byte&0x3])
		}
	}
	v.pending = v.pending[:0]
	screen.DrawImage(v.bg, nil)
	screen.DrawImage(v.fg, nil)
}

func (v *gui) Layout(outerWidth, outerHeight int) (screenWidth, screenHeight int) {
	return v.width, v.height
}
