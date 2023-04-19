package varvara

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/math/f64"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
)

const debugGUI = false

func NewGUI(v *Varvara) *GUI {
	up, done := make(chan bool), make(chan bool)
	return &GUI{
		Update: up, doUpdate: up,
		UpdateDone: done, updateDone: done,
		v: v,
	}
}

type GUI struct {
	Update     chan<- bool
	UpdateDone <-chan bool

	doUpdate   <-chan bool
	updateDone chan<- bool

	v *Varvara // only touch this in the update method!

	ctrl  ControllerState
	mouse MouseState

	// Screen
	wsize    size.Event
	size     image.Point
	fg, bg   screen.Buffer
	tex      screen.Texture
	xform    f64.Aff3 // from varvara buffer to window buffer
	xformInv f64.Aff3
	ops      int // updated to match v.scr.ops after copying fg/bg
}

type updateEvent struct{}

var errCloseGUI = errors.New("close GUI")

func (g *GUI) Run(exit <-chan bool) (err error) {
	defer close(g.updateDone)
	driver.Main(func(s screen.Screen) {
		var w screen.Window
		w, err = s.NewWindow(&screen.NewWindowOptions{Title: "nux"})
		if err != nil {
			return
		}
		defer w.Release()
		defer g.release()

		go func() {
			t := time.NewTicker(time.Second / 60)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					w.Send(updateEvent{})
				case <-exit:
					return
				}
			}
		}()

		for err == nil {
			select {
			case <-exit:
				return
			default:
			}
			err = g.handle(s, w, w.NextEvent())
		}
	})
	if err == errCloseGUI {
		err = nil
	}
	return
}

func (g *GUI) handle(s screen.Screen, w screen.Window, e any) error {
	if debugGUI {
		switch e := e.(type) {
		case paint.Event:
		case updateEvent:
		default:
			format := "got %#v\n"
			if _, ok := e.(fmt.Stringer); ok {
				format = "got %v\n"
			}
			log.Printf(format, e)
		}
	}

	switch e := e.(type) {
	case size.Event:
		g.wsize = e
		if e.WidthPx+e.HeightPx == 0 {
			// Window closed.
			return errCloseGUI
		}
		if g.bg != nil {
			g.updateTransform()
		}

	case lifecycle.Event:
		if e.To == lifecycle.StageDead {
			return errCloseGUI
		}

	case key.Event:
		g.handleKey(e)

	case mouse.Event:
		g.handleMouse(e)

	case updateEvent:
		select {
		case <-g.doUpdate:
			err := g.update(s)
			g.updateDone <- true
			if err != nil {
				return fmt.Errorf("update: %v", err)
			}

		default:
			// uxn cpu is busy
		}
		g.paint(w)

	case error:
		log.Printf("gui: %v", e)
	}
	return nil
}

// update synchronizes state between gui and Varvara.
// It must only be called when the Varvara CPU is not executing.
func (g *GUI) update(s screen.Screen) (err error) {
	g.v.cntrl.Set(&g.ctrl)
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
		g.updateTransform()
	}
	if o := g.v.scr.ops; g.ops != o {
		if m := g.v.scr.fg; m != nil && m.Bounds().Size() == g.size {
			copy(g.fg.RGBA().Pix, m.Pix)
		}
		if m := g.v.scr.bg; m != nil && m.Bounds().Size() == g.size {
			copy(g.bg.RGBA().Pix, m.Pix)
		}
		g.ops = o
	}
	return
}

func (g *GUI) updateTransform() {
	g.xform = paintTransform(g.wsize.Bounds(), g.bg.Bounds())
	g.xformInv = invert(g.xform)
}

func (g *GUI) release() {
	if g.tex != nil {
		g.tex.Release()
	}
	if g.fg != nil {
		g.fg.Release()
	}
	if g.bg != nil {
		g.bg.Release()
	}
}

// paint draws bg and fg to the given window.
func (g *GUI) paint(w screen.Window) {
	w.Fill(g.wsize.Bounds(), color.RGBA{0, 0, 0, 0}, draw.Src)
	if g.bg != nil { // fg, tex, and xform must also be set
		g.tex.Upload(image.Point{}, g.bg, g.bg.Bounds())
		w.Draw(g.xform, g.tex, g.tex.Bounds(), draw.Src, nil)
		g.tex.Upload(image.Point{}, g.fg, g.fg.Bounds())
		w.Draw(g.xform, g.tex, g.tex.Bounds(), draw.Over, nil)
	}
	w.Publish()
}

// paintTransform returns the affine transform that maps the pixels in the
// source to the largest rectangle that fits inside the destination.
func paintTransform(dst, src image.Rectangle) f64.Aff3 {
	var (
		wx, wy = float64(dst.Dx()), float64(dst.Dy())
		sx, sy = float64(src.Dx()), float64(src.Dy())
		wr     = float64(wx) / float64(wy)
		sr     = float64(sx) / float64(sy)
		dx, dy float64
	)
	if wr > sr {
		dx, dy = wy*sr, wy
	} else {
		dx, dy = wx, wx/sr
	}
	return f64.Aff3{
		dx / sx, 0, (wx - dx) / 2,
		0, dy / sy, (wy - dy) / 2,
	}
}

func (g *GUI) handleKey(e key.Event) {
	var (
		s = &g.ctrl
		b = e.Direction == key.DirPress || e.Direction == 10
	)
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

func (g *GUI) handleMouse(e mouse.Event) {
	if g.bg == nil {
		// Screen not initialized; can't compute mouse x/y.
		return
	}
	var (
		m  = &g.mouse
		sx = float64(e.X)
		sy = float64(e.Y)
		t  = g.xformInv
	)
	m.X = clampInt16(t[0]*sx + t[1]*sy + t[2])
	m.Y = clampInt16(t[3]*sx + t[4]*sy + t[5])
	if e.Button >= 1 && e.Button <= 3 && e.Direction != mouse.DirNone {
		m.Button[e.Button-1] = e.Direction == mouse.DirPress
	}
}

func clampInt16(v float64) int16 {
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

func invert(m f64.Aff3) f64.Aff3 {
	m00 := +m[3*1+1]
	m01 := -m[3*0+1]
	m02 := +m[3*1+2]*m[3*0+1] - m[3*1+1]*m[3*0+2]
	m10 := -m[3*1+0]
	m11 := +m[3*0+0]
	m12 := +m[3*1+0]*m[3*0+2] - m[3*1+2]*m[3*0+0]

	det := m00*m11 - m10*m01

	return f64.Aff3{
		m00 / det,
		m01 / det,
		m02 / det,
		m10 / det,
		m11 / det,
		m12 / det,
	}
}
