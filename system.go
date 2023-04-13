package main

type System struct {
	Done chan bool

	mem deviceMem
}

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
