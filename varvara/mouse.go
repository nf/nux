package varvara

type Mouse struct {
	inputDevice
}

func (m *Mouse) Set(x, y, scrollX, scrollY int16, b1, b2, b3 bool) {
	var u bool

	u = m.mem.setShortChanged(0x2, uint16(x)) || u
	u = m.mem.setShortChanged(0x4, uint16(y)) || u
	u = m.mem.setShortChanged(0xa, uint16(scrollX)) || u
	u = m.mem.setShortChanged(0xc, uint16(scrollY)) || u

	var b byte
	if b1 {
		b |= 0x1
	}
	if b2 {
		b |= 0x2
	}
	if b3 {
		b |= 0x4
	}
	u = m.mem.setChanged(0x6, b) || u

	if u {
		m.updated()
	}
}
