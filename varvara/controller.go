package varvara

type Controller struct {
	inputDevice
}

type ControllerState struct {
	A, B, Select, Start   bool
	Up, Down, Left, Right bool

	Key byte
}

func (c *Controller) Set(s *ControllerState) {
	u := false

	var b byte
	if s.A {
		b |= 0x01
	}
	if s.B {
		b |= 0x02
	}
	if s.Select {
		b |= 0x04
	}
	if s.Start {
		b |= 0x08
	}
	if s.Up {
		b |= 0x10
	}
	if s.Down {
		b |= 0x20
	}
	if s.Left {
		b |= 0x40
	}
	if s.Right {
		b |= 0x80
	}
	u = c.mem.setChanged(0x2, b) || u

	u = c.mem.setChanged(0x3, s.Key) || u

	if u {
		c.updated()
	}
}
