package varvara

type Screen struct {
	mem  deviceMem
	main []byte  // sprite data
	sys  *System // r, g, b

	fg, bg *image
	ops    int // total count of draw operations
}

func (s *Screen) Vector() uint16 { return s.mem.short(0x0) }
func (s *Screen) Width() uint16  { return s.mem.short(0x2) }
func (s *Screen) Height() uint16 { return s.mem.short(0x4) }
func (s *Screen) Auto() AutoByte { return AutoByte(s.mem[0x6]) }
func (s *Screen) X() int16       { return int16(s.mem.short(0x8)) }
func (s *Screen) Y() int16       { return int16(s.mem.short(0xa)) }
func (s *Screen) Addr() uint16   { return s.mem.short(0xc) }

func (s *Screen) setWidth(w uint16)  { s.mem.setShort(0x2, w) }
func (s *Screen) setHeight(h uint16) { s.mem.setShort(0x4, h) }
func (s *Screen) setX(x int16)       { s.mem.setShort(0x8, uint16(x)) }
func (s *Screen) setY(y int16)       { s.mem.setShort(0xa, uint16(y)) }
func (s *Screen) setAddr(a uint16)   { s.mem.setShort(0xc, a) }

type AutoByte byte

func (b AutoByte) X() bool     { return b&0x01 != 0 }
func (b AutoByte) Y() bool     { return b&0x02 != 0 }
func (b AutoByte) Addr() bool  { return b&0x04 != 0 }
func (b AutoByte) Count() int8 { return int8(b >> 4) }

type drawOp byte

func (b drawOp) Color() byte      { return byte(b) & 0x03 } // pixel only
func (b drawOp) Blend() byte      { return byte(b) & 0x0f } // sprite only
func (b drawOp) FlipX() bool      { return b&0x10 != 0 }
func (b drawOp) FlipY() bool      { return b&0x20 != 0 }
func (b drawOp) Foreground() bool { return b&0x40 != 0 }
func (b drawOp) Fill() bool       { return b&0x80 != 0 } // pixel only
func (b drawOp) TwoBit() bool     { return b&0x80 != 0 } // sprite only

func (s *Screen) In(p byte) byte {
	return s.mem[p]
}

func (s *Screen) Out(p, v byte) {
	s.mem[p] = v

	switch p {
	default:
		return
	case 0xe:
		s.drawPixel(drawOp(v))
	case 0xf:
		s.drawSprite(drawOp(v))
	}
	s.ops++
}

func (s *Screen) imageFor(op drawOp) (*image, [4]rgba) {
	r, g, b := s.sys.Red(), s.sys.Green(), s.sys.Blue()
	theme := [4]rgba{
		{byte(r & 0xf000 >> 8), byte(g & 0xf000 >> 8), byte(b & 0xf000 >> 8), 0xff},
		{byte(r & 0x0f00 >> 4), byte(g & 0x0f00 >> 4), byte(b & 0x0f00 >> 4), 0xff},
		{byte(r & 0x00f0), byte(g & 0x00f0), byte(b & 0x00f0), 0xff},
		{byte(r & 0x000f << 4), byte(g & 0x000f << 4), byte(b & 0x000f << 4), 0xff},
	}
	if s.fg == nil || s.fg.w != int(s.Width()) || s.fg.h != int(s.Height()) {
		s.fg = newImage(s.Width(), s.Height(), transparent)
		s.bg = newImage(s.Width(), s.Height(), theme[0])
	}
	if op.Foreground() {
		return s.fg, theme
	} else {
		return s.bg, theme
	}
}

func (s *Screen) drawPixel(op drawOp) {
	var (
		m, theme = s.imageFor(op)
		c        = theme[op.Color()]
	)
	if op.Fill() {
		var dx, dy int16 = 1, 1
		if op.FlipX() {
			dx = -1
		}
		if op.FlipY() {
			dy = -1
		}
		for x := s.X(); 0 <= x && x < int16(m.w); x += dx {
			for y := s.Y(); 0 <= y && y < int16(m.h); y += dy {
				m.set(x, y, c)
			}
		}
	} else {
		m.set(s.X(), s.Y(), c)
	}
	if s.Auto().X() {
		s.setX(s.X() + 1)
	}
	if s.Auto().Y() {
		s.setY(s.Y() + 1)
	}
}

func (s *Screen) drawSprite(op drawOp) {
	var (
		m, theme = s.imageFor(op)
		auto     = s.Auto()
		addr     = s.Addr()
		sx, sy   = s.X(), s.Y() // sprite top-left
		// drawZero reports whether this blending mode should draw
		// color zero; if false, pixels of color zero are not set.
		drawZero = op.Blend() == 0 || op.Blend()%5 != 0
		sprite   = s.main[addr:]
	)
	for i := auto.Count(); i >= 0; i-- {
		var (
			x, y         = sx, sy
			dx, dy int16 = 1, 1
		)
		if !op.FlipX() {
			x += 7
			dx = -1
		}
		if op.FlipY() {
			y += 7
			dy = -1
		}
		for j := 0; j < 8; j++ {
			pxA, pxB := sprite[j], sprite[j+8]
			for i := 0; i < 8; i++ {
				px := pxA & 1
				pxA >>= 1
				if op.TwoBit() {
					px |= pxB & 1 << 1
					pxB >>= 1
				}
				px = drawBlendingModes[px][op.Blend()]
				if drawZero || px > 0 {
					c := transparent
					if !op.Foreground() || px > 0 {
						c = theme[px]
					}
					m.set(x, y, c)
				}
				x += dx
			}
			x += -dx * 8
			y += dy
		}
		if auto.X() {
			sy += 8
		}
		if auto.Y() {
			sx += 8
		}
		if auto.Addr() {
			if op.TwoBit() {
				addr += 0x10
			} else {
				addr += 0x08
			}
			sprite = s.main[addr:]
		}
	}
	if auto.X() {
		s.setX(s.X() + 8)
	}
	if auto.Y() {
		s.setY(s.Y() + 8)
	}
	if auto.Addr() {
		s.setAddr(addr)
	}
}

type rgba [4]byte

var transparent = rgba{0, 0, 0, 0}

type image struct {
	w, h int
	buf  []byte
}

func newImage(w, h uint16, c rgba) *image {
	m := &image{int(w), int(h), make([]byte, int(w)*int(h)*4)}
	for b := m.buf; len(b) > 0; b = b[4:] {
		copy(b, c[:])
	}
	return m
}

func (m *image) set(x, y int16, c rgba) {
	if 0 <= x && int(x) < m.w && 0 <= y && int(y) < m.h {
		copy(m.buf[(int(y)*m.w+int(x))*4:], c[:])
	}
}

var drawBlendingModes = [4][16]byte{
	{0, 0, 0, 0, 1, 0, 1, 1, 2, 2, 0, 2, 3, 3, 3, 0},
	{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3},
	{1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1},
	{2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2}}
