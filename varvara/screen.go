package varvara

type Screen struct {
	mem  deviceMem
	main []byte

	pending []DrawOp
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

func (s *Screen) Out(p, b byte) {
	s.mem[p] = b
	switch p {
	case 0xe: // pixel
		var (
			auto = s.Auto()
			x, y = s.X(), s.Y()
		)
		op := DrawOp{DrawByte: DrawByte(b), X: x, Y: y}
		s.pending = append(s.pending, op)
		if auto.X() {
			s.setX(x + 1)
		}
		if auto.Y() {
			s.setY(y + 1)
		}
	case 0xf: // sprite
		var (
			b      = DrawByte(b)
			auto   = s.Auto()
			addr   = s.Addr()
			x, y   = s.X(), s.Y()
			sx, sy = x, y
		)
		for i := int(auto.Count()); i >= 0; i-- {
			op := DrawOp{DrawByte: b, X: sx, Y: sy, Sprite: true}
			copy(op.SpriteData[:], s.main[addr:])
			s.pending = append(s.pending, op)
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
			s.setX(x + 8)
		}
		if auto.Y() {
			s.setY(y + 8)
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

// DrawOp represents a screen draw operation, painting either pixels or
// sprites. If a sprite operation, Sprite is true and SpriteData holds
// the sprite data at the time the sprite byte was written.
// If a pixel operation, Sprite will be false and SpriteData undefined.
type DrawOp struct {
	DrawByte
	X, Y       int16
	Sprite     bool
	SpriteData [16]byte
}

var drawBlendingModes = [4][16]byte{
	{0, 0, 0, 0, 1, 0, 1, 1, 2, 2, 0, 2, 3, 3, 3, 0},
	{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3},
	{1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1},
	{2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2}}
