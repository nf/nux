package varvara

import "github.com/nf/nux/uxn"

type System struct {
	mem deviceMem
}

func (s *System) Halt() uint16  { return s.mem.short(0x0) }
func (s *System) Red() uint16   { return s.mem.short(0x8) }
func (s *System) Green() uint16 { return s.mem.short(0xa) }
func (s *System) Blue() uint16  { return s.mem.short(0xc) }
func (s *System) ExitCode() int { return int(s.mem[0xf] & 0x7f) }

func (s *System) In(p byte) byte {
	return s.mem[p]
}

func (s *System) Out(p, b byte) {
	s.mem[p] = b
	switch p {
	case 0xf:
		if b != 0 {
			panic(uxn.Halt) // Stop execution.
		}
	}
}
