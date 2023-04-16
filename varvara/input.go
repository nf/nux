package varvara

type inputDevice struct {
	Ready <-chan bool

	mem   deviceMem
	ready chan bool
}

func (d *inputDevice) Vector() uint16 { return d.mem.short(0x0) }
func (d *inputDevice) In(p byte) byte { return d.mem[p] }
func (d *inputDevice) Out(p, b byte) {
	if p&0x1 == p && d.ready == nil {
		d.ready = make(chan bool, 1)
		d.Ready = d.ready
	}
	d.mem[p] = b
}

func (d *inputDevice) updated() {
	select {
	case d.ready <- true:
	default:
	}
}
