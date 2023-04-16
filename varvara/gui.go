package varvara

import (
	"fmt"
	"image"
	"image/draw"
	"log"
	"time"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

func newGUI(v *Varvara) *gui {
	g := &gui{Varvara: v}
	return g
}

func (v *gui) Run(exit <-chan bool) error {
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

		defer v.release()

		var sz size.Event
		for {
			e := w.NextEvent()

			switch e := e.(type) {
			case update:
			case paint.Event:
			case mouse.Event:
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
					return
				}

			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					return
				}

			case mouse.Event:
				v.mouseX = clampInt16(int(float32(v.size.X) / float32(sz.WidthPx) * e.X))
				v.mouseY = clampInt16(int(float32(v.size.Y) / float32(sz.HeightPx) * e.Y))
				if e.Button >= 1 && e.Button <= 3 {
					v.mouseB[e.Button-1] = e.Direction == mouse.DirPress
				}

			case update:
				select {
				case <-v.guiUpdate:
					if err := v.update(s); err != nil {
						log.Fatalf("update: %v", err)
					}
					v.guiUpdateDone <- true
				default:
					// uxn cpu is busy
				}
				if v.dirty {
					v.tex.Upload(image.Point{}, v.bg, v.bg.Bounds())
					w.Scale(sz.Bounds(), v.tex, v.tex.Bounds(), draw.Src, nil)
					v.tex.Upload(image.Point{}, v.fg, v.fg.Bounds())
					w.Scale(sz.Bounds(), v.tex, v.tex.Bounds(), draw.Over, nil)
					w.Publish()
					v.dirty = false
				}

			case error:
				log.Print(e)
			}
		}
	})
	return nil
}

type gui struct {
	*Varvara

	mouseX, mouseY int16
	mouseB         [3]bool

	size   image.Point
	fg, bg screen.Buffer
	tex    screen.Texture
	ops    int // updated to match v.scr.ops after copying fg/bg
	dirty  bool
}

func (v *gui) update(s screen.Screen) (err error) {
	// Mouse
	v.mouse.Set(v.mouseX, v.mouseY, 0, 0, v.mouseB[0], v.mouseB[1], v.mouseB[2])

	// Screen
	v.size = image.Point{int(v.scr.Width()), int(v.scr.Height())}
	if v.size.X == 0 || v.size.Y == 0 {
		v.size = image.Point{0x100, 0x100}
	}
	if v.tex == nil || v.tex.Size() != v.size {
		v.release()
		v.fg, err = s.NewBuffer(v.size)
		if err != nil {
			return
		}
		v.bg, err = s.NewBuffer(v.size)
		if err != nil {
			return
		}
		v.tex, err = s.NewTexture(v.size)
		if err != nil {
			return
		}
		v.ops = -1
	}
	if o := v.scr.ops; v.ops != o {
		v.ops = o
		if m := v.scr.fg; m != nil && m.w == v.size.X && m.h == v.size.Y {
			copy(v.fg.RGBA().Pix, m.buf)
		}
		if m := v.scr.bg; m != nil && m.w == v.size.X && m.h == v.size.Y {
			copy(v.bg.RGBA().Pix, m.buf)
		}
		v.dirty = true
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
