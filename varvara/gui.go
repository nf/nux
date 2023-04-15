package varvara

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
		v.bg.Fill(v.theme[0])
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
		var (
			op = &v.pending[i]
			m  *ebiten.Image
		)
		if op.Foreground() {
			m = v.fg
		} else {
			m = v.bg
		}
		if op.Sprite {
			var (
				x, y     = op.X, op.Y
				drawZero = op.Blend() == 0 || op.Blend()%5 != 0
			)
			if !op.FlipX() {
				x += 7
			}
			if op.FlipY() {
				y += 7
			}
			for j := 0; j < 8; j++ {
				pxA := op.SpriteData[j]
				pxB := op.SpriteData[j+8]
				for i := 0; i < 8; i++ {
					px := pxA & 1
					if op.TwoBit() {
						px |= pxB & 1 << 1
					}
					px = drawBlendingModes[px][op.Blend()]
					if drawZero || px > 0 {
						var c color.Color = color.Transparent
						if !op.Foreground() || px > 0 {
							c = v.theme[px]
						}
						m.Set(int(x), int(y), c)
					}
					pxA >>= 1
					pxB >>= 1
					if op.FlipX() {
						x++
					} else {
						x--
					}
				}
				if op.FlipX() {
					x -= 8
				} else {
					x += 8
				}
				if op.FlipY() {
					y--
				} else {
					y++
				}
			}
		} else { // pixel
			c := v.theme[op.Color()]
			if op.Fill() {
				var dx, dy int16 = 1, 1
				if op.FlipX() {
					dx = -1
				}
				if op.FlipY() {
					dy = -1
				}
				for x := op.X; x >= 0 && int(x) < v.width; x += dx {
					for y := op.Y; y >= 0 && int(y) < v.height; y += dy {
						m.Set(int(x), int(y), c)
					}
				}
			} else {
				m.Set(int(op.X), int(op.Y), c)
			}
		}
	}
	v.pending = v.pending[:0]
	screen.DrawImage(v.bg, nil)
	screen.DrawImage(v.fg, nil)
}

func (v *gui) Layout(outerWidth, outerHeight int) (screenWidth, screenHeight int) {
	return v.width, v.height
}
