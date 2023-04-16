package varvara

type Mouse struct {
	Ready <-chan bool

	mem   deviceMem
	ready chan bool
}

func (m *Mouse) Vector() uint16 { return m.mem.short(0x0) }

func (m *Mouse) set(x, y, scrollX, scrollY int16, b1, b2, b3 bool) {
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
		select {
		case m.ready <- true:
		default:
		}
	}
}

func (m *Mouse) In(p byte) byte {
	if m.ready == nil {
		m.ready = make(chan bool, 1)
		m.Ready = m.ready
	}
	return m.mem[p]
}

func (m *Mouse) Out(p, b byte) {
	m.mem[p] = b
}
