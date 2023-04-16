package varvara

type Screen struct {
	mem  deviceMem
	main []byte
	sys  *System

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

func (s *Screen) setX(x int16)     { s.mem.setShort(0x8, uint16(x)) }
func (s *Screen) setY(y int16)     { s.mem.setShort(0xa, uint16(y)) }
func (s *Screen) setAddr(a uint16) { s.mem.setShort(0xc, a) }

func (s *Screen) In(p byte) byte {
	return s.mem[p]
}

type rgba [4]byte

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

func (s *Screen) Out(p, v byte) {
	s.mem[p] = v
	switch p {
	case 0xe, 0xf:
		// Handled below.
	default:
		return
	}
	s.ops++
	var (
		trans = rgba{0, 0, 0, 0}
		theme = makeTheme(s.sys)
	)
	if s.fg == nil || s.fg.w != int(s.Width()) || s.fg.h != int(s.Height()) {
		s.fg = newImage(s.Width(), s.Height(), trans)
		s.bg = newImage(s.Width(), s.Height(), theme[0])
	}
	var (
		auto = s.Auto()
		x, y = s.X(), s.Y()
		b    = DrawByte(v)
		m    *image
	)
	if b.Foreground() {
		m = s.fg
	} else {
		m = s.bg
	}
	switch p {
	case 0xe: // pixel
		c := theme[b.Color()]
		if b.Fill() {
			var dx, dy int16 = 1, 1
			if b.FlipX() {
				dx = -1
			}
			if b.FlipY() {
				dy = -1
			}
			for x := x; x >= 0 && x < int16(m.w); x += dx {
				for y := y; y >= 0 && y < int16(m.h); y += dy {
					m.set(x, y, c)
				}
			}
		} else {
			m.set(x, y, c)
		}
		if auto.X() {
			s.setX(x + 1)
		}
		if auto.Y() {
			s.setY(y + 1)
		}
	case 0xf: // sprite
		var (
			addr     = s.Addr()
			sx, sy   = x, y
			drawZero = b.Blend() == 0 || b.Blend()%5 != 0
		)
		for i := int(auto.Count()); i >= 0; i-- {
			var (
				spr  = s.main[addr:]
				x, y = sx, sy
			)
			if !b.FlipX() {
				x += 7
			}
			if b.FlipY() {
				y += 7
			}
			for j := 0; j < 8; j++ {
				pxA := spr[j]
				pxB := spr[j+8]
				for i := 0; i < 8; i++ {
					px := pxA & 1
					if b.TwoBit() {
						px |= pxB & 1 << 1
					}
					px = drawBlendingModes[px][b.Blend()]
					if drawZero || px > 0 {
						c := trans
						if !b.Foreground() || px > 0 {
							c = theme[px]
						}
						m.set(x, y, c)
					}
					pxA >>= 1
					pxB >>= 1
					if b.FlipX() {
						x++
					} else {
						x--
					}
				}
				if b.FlipX() {
					x -= 8
				} else {
					x += 8
				}
				if b.FlipY() {
					y--
				} else {
					y++
				}
			}
			if auto.X() {
				sy += 8
			}
			if auto.Y() {
				sx += 8
			}
			if auto.Addr() {
				if b.TwoBit() {
					addr += 0x10
				} else {
					addr += 0x08
				}
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
}

type AutoByte byte

func (b AutoByte) X() bool     { return b&0x01 != 0 }
func (b AutoByte) Y() bool     { return b&0x02 != 0 }
func (b AutoByte) Addr() bool  { return b&0x04 != 0 }
func (b AutoByte) Count() byte { return byte(b >> 4) }

type DrawByte byte

func (b DrawByte) Color() byte      { return byte(b) & 0x03 } // pixel
func (b DrawByte) Blend() byte      { return byte(b) & 0x0f } // sprite
func (b DrawByte) FlipX() bool      { return b&0x10 != 0 }
func (b DrawByte) FlipY() bool      { return b&0x20 != 0 }
func (b DrawByte) Foreground() bool { return b&0x40 != 0 }
func (b DrawByte) Fill() bool       { return b&0x80 != 0 } // pixel
func (b DrawByte) TwoBit() bool     { return b&0x80 != 0 } // sprite

func makeTheme(sys *System) [4]rgba {
	r, g, b := sys.Red(), sys.Green(), sys.Blue()
	return [4]rgba{
		{byte(r & 0xf000 >> 8), byte(g & 0xf000 >> 8), byte(b & 0xf000 >> 8), 0xff},
		{byte(r & 0x0f00 >> 4), byte(g & 0x0f00 >> 4), byte(b & 0x0f00 >> 4), 0xff},
		{byte(r & 0x00f0), byte(g & 0x00f0), byte(b & 0x00f0), 0xff},
		{byte(r & 0x000f << 4), byte(g & 0x000f << 4), byte(b & 0x000f << 4), 0xff},
	}
}

var drawBlendingModes = [4][16]byte{
	{0, 0, 0, 0, 1, 0, 1, 1, 2, 2, 0, 2, 3, 3, 3, 0},
	{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3},
	{1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1},
	{2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2}}
