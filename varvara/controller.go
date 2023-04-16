package varvara

type Controller struct {
	inputDevice
}

func (c *Controller) SetButtons(a, b, slct, strt, up, down, left, right bool) {
	var v byte
	if a {
		v |= 0x01
	}
	if b {
		v |= 0x02
	}
	if slct {
		v |= 0x04
	}
	if strt {
		v |= 0x08
	}
	if up {
		v |= 0x10
	}
	if down {
		v |= 0x20
	}
	if left {
		v |= 0x40
	}
	if right {
		v |= 0x80
	}
	if c.mem.setChanged(0x2, v) {
		c.updated()
	}
}

func (c *Controller) SetKey(k byte) {
	if c.mem.setChanged(0x3, k) {
		c.updated()
	}
}
