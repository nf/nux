package varvara

type System struct {
	Done chan bool

	mem deviceMem
}

func (s *System) Halt() uint16  { return s.mem.short(0x0) }
func (s *System) Red() uint16   { return s.mem.short(0x8) }
func (s *System) Green() uint16 { return s.mem.short(0xa) }
func (s *System) Blue() uint16  { return s.mem.short(0xc) }
func (s *System) ExitCode() int { return int(s.mem[0xf] & 0x7f) }

func (s *System) In(d byte) byte {
	return s.mem[d]
}

func (s *System) Out(d, b byte) {
	s.mem[d] = b
	switch d {
	case 0xf:
		if b != 0 {
			close(s.Done)
			select {} // Prevent further execution.
		}
	}
}
