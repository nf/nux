package varvara

type Mouse struct {
	inputDevice
}

type MouseState struct {
	X, Y             int16
	ScrollX, ScrollY int16
	Button           [3]bool
}

func (m *Mouse) Set(s *MouseState) {
	u := false

	u = m.mem.setShortChanged(0x2, uint16(s.X)) || u
	u = m.mem.setShortChanged(0x4, uint16(s.Y)) || u
	u = m.mem.setShortChanged(0xa, uint16(s.ScrollX)) || u
	u = m.mem.setShortChanged(0xc, uint16(s.ScrollY)) || u

	var b byte
	if s.Button[0] {
		b |= 0x1
	}
	if s.Button[1] {
		b |= 0x2
	}
	if s.Button[2] {
		b |= 0x4
	}
	u = m.mem.setChanged(0x6, b) || u

	if u {
		m.updated()
	}
}
