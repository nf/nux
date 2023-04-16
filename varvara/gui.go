package varvara

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"time"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

func newGUI(v *Varvara) *gui {
	g := &gui{v: v}
	return g
}

func (g *gui) Run(exit <-chan bool) error {
	driver.Main(func(s screen.Screen) {
		w, err := s.NewWindow(&screen.NewWindowOptions{Title: "nux"})
		if err != nil {
			log.Fatal(err)
		}
		defer w.Release()

		type update struct{}
		go func() {
			t := time.NewTicker(time.Second / 60)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					w.Send(update{})
				case <-exit:
					return
				}
			}
		}()

		defer g.release()

		var sz size.Event
		for {
			e := w.NextEvent()

			switch e := e.(type) {
			case paint.Event:
			case key.Event:
			case mouse.Event:
			case update:
			default:
				format := "got %#v\n"
				if _, ok := e.(fmt.Stringer); ok {
					format = "got %v\n"
				}
				log.Printf(format, e)
			}

			select {
			case <-exit:
				return
			default:
			}

			switch e := e.(type) {
			case size.Event:
				sz = e
				if sz.WidthPx+sz.HeightPx == 0 {
					// Window closed.
					return
				}

			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					return
				}

			case key.Event:
				g.ctrl.update(e)

			case mouse.Event:
				g.mouse.X = clampInt16(int(float32(g.size.X) / float32(sz.WidthPx) * e.X))
				g.mouse.Y = clampInt16(int(float32(g.size.Y) / float32(sz.HeightPx) * e.Y))
				if e.Button >= 1 && e.Button <= 3 {
					g.mouse.Button[e.Button-1] = e.Direction == mouse.DirPress
				}

			case update:
				select {
				case <-g.v.guiUpdate:
					if err := g.update(s); err != nil {
						log.Fatalf("update: %v", err)
					}
					g.v.guiUpdateDone <- true
				default:
					// uxn cpu is busy
				}
				if g.dirty {
					g.tex.Upload(image.Point{}, g.bg, g.bg.Bounds())
					w.Scale(sz.Bounds(), g.tex, g.tex.Bounds(), draw.Src, nil)
					g.tex.Upload(image.Point{}, g.fg, g.fg.Bounds())
					w.Scale(sz.Bounds(), g.tex, g.tex.Bounds(), draw.Over, nil)
					w.Publish()
					g.dirty = false
				}

			case error:
				log.Print(e)
			}
		}
	})
	return nil
}

type gui struct {
	v *Varvara

	ctrl  ControllerState
	mouse MouseState

	size   image.Point
	fg, bg screen.Buffer
	tex    screen.Texture
	ops    int // updated to match v.scr.ops after copying fg/bg
	dirty  bool
}

func (g *gui) update(s screen.Screen) (err error) {
	// Buttons
	g.v.cntrl.Set(&g.ctrl)

	// Mouse
	g.v.mouse.Set(&g.mouse)

	// Screen
	g.size = image.Point{int(g.v.scr.Width()), int(g.v.scr.Height())}
	if g.size.X == 0 || g.size.Y == 0 {
		g.size = image.Point{0x100, 0x100}
	}
	if g.tex == nil || g.tex.Size() != g.size {
		g.release()
		g.fg, err = s.NewBuffer(g.size)
		if err != nil {
			return
		}
		g.bg, err = s.NewBuffer(g.size)
		if err != nil {
			return
		}
		g.tex, err = s.NewTexture(g.size)
		if err != nil {
			return
		}
		g.ops = -1
	}
	if o := g.v.scr.ops; g.ops != o {
		g.ops = o

		if m := g.v.scr.fg; m != nil && m.Bounds().Size() == g.size {
			copy(g.fg.RGBA().Pix, m.Pix)
		}
		if m := g.v.scr.bg; m != nil && m.Bounds().Size() == g.size {
			copy(g.bg.RGBA().Pix, m.Pix)
		}
		g.dirty = true
	}
	return
}

func (v *gui) release() {
	if v.tex != nil {
		v.tex.Release()
	}
	if v.fg != nil {
		v.fg.Release()
	}
	if v.bg != nil {
		v.bg.Release()
	}
}

func (s *ControllerState) update(e key.Event) {
	b := e.Direction == key.DirPress || e.Direction == 10
	switch e.Code {
	case key.CodeLeftControl:
		s.A = b
	case key.CodeLeftAlt:
		s.B = b
	case key.CodeLeftShift:
		s.Select = b
	case key.CodeHome:
		s.Start = b
	case key.CodeUpArrow:
		s.Up = b
	case key.CodeDownArrow:
		s.Down = b
	case key.CodeLeftArrow:
		s.Left = b
	case key.CodeRightArrow:
		s.Right = b
	}
	if 0 <= e.Rune && e.Rune < 0x80 {
		k := byte(e.Rune)
		if e.Modifiers&key.ModControl != 0 && 'A'-0x40 <= k && k <= 'Z'-0x40 {
			if e.Modifiers&key.ModShift != 0 {
				k += 0x40
			} else {
				k += 0x60
			}
		}
		s.Key = k
	}
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
