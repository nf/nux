package main

type Screen struct {
	mem  deviceMem
	main []byte

	pending []DrawOp
}

func (s *Screen) Vector() uint16 { return s.mem.short(0x0) }
func (s *Screen) Width() uint16  { return s.mem.short(0x2) }
func (s *Screen) Height() uint16 { return s.mem.short(0x4) }
func (s *Screen) Auto() byte     { return s.mem[0x6] }
func (s *Screen) X() uint16      { return s.mem.short(0x8) }
func (s *Screen) Y() uint16      { return s.mem.short(0xa) }
func (s *Screen) Addr() uint16   { return s.mem.short(0xc) }

func (s *Screen) setX(x uint16)    { s.mem.setShort(0x8, x) }
func (s *Screen) setY(y uint16)    { s.mem.setShort(0xa, y) }
func (s *Screen) setAddr(a uint16) { s.mem.setShort(0xc, a) }

func (s *Screen) In(d byte) byte { return s.mem[d] }

func (s *Screen) Out(d, b byte) {
	s.mem[d] = b
	switch d {
	case 0xe: // pixel
		var (
			auto = s.Auto()
			x, y = s.X(), s.Y()
		)
		s.pending = append(s.pending, DrawOp{X: x, Y: y, Byte: b})
		if auto&0x1 != 0 { // x
			s.setX(x + 1)
		}
		if auto&0x2 != 0 { // y
			s.setY(y + 1)
		}
	case 0xf: // sprite
		var (
			auto = s.Auto()
			x, y = s.X(), s.Y()
			addr = s.Addr()
		)
		for n := int8(auto >> 4); n >= 0; n-- {
			op := DrawOp{X: x, Y: y, Byte: b, Sprite: true}
			copy(op.SpriteData[:], s.main[addr:])
			s.pending = append(s.pending, op)
			if auto&0x1 != 0 { // x
				x += 8
			}
			if auto&0x2 != 0 { // y
				y += 8
			}
			if auto&0x4 != 0 { // addr
				if b&0x80 == 0 {
					addr += 0x08
				} else {
					addr += 0x10
				}
			}
		}
		s.setX(x)
		s.setY(y)
		s.setAddr(addr)
	}
}

type DrawOp struct {
	X, Y       uint16
	Byte       byte
	Sprite     bool
	SpriteData [16]byte
}

var drawBlendingModes = [4][16]byte{
	{0, 0, 0, 0, 1, 0, 1, 1, 2, 2, 0, 2, 3, 3, 3, 0},
	{0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3, 0, 1, 2, 3},
	{1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1, 1, 2, 3, 1},
	{2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2, 2, 3, 1, 2}}
