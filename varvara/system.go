package varvara

import (
	"github.com/nf/nux/uxn"
)

type System struct {
	mem   deviceMem
	main  []byte
	m     *uxn.Machine
	state StateFunc
}

func (s *System) Halt() uint16  { return s.mem.short(0x0) }
func (s *System) Red() uint16   { return s.mem.short(0x8) }
func (s *System) Green() uint16 { return s.mem.short(0xa) }
func (s *System) Blue() uint16  { return s.mem.short(0xc) }
func (s *System) Exited() bool  { return s.mem[0xf] != 0 }
func (s *System) ExitCode() int { return int(s.mem[0xf] & 0x7f) }

func (s *System) In(p byte) byte {
	return s.mem[p]
}

func (s *System) Out(p, b byte) {
	s.mem[p] = b
	switch p {
	case 0x3:
		addr := s.mem.short(0x2)
		v := func(offset uint16) uint16 {
			return short(s.main[addr+offset], s.main[addr+offset+1])
		}
		length := int(v(1))
		bank := int(v(3)) * 0x10000
		srcAddr := int(v(5))
		length = min(length, 0x10000-srcAddr)
		if bank >= len(s.main) {
			return
		}
		switch s.main[addr] {
		case 0x00: // fill
			for i := range length {
				s.main[bank+srcAddr+i] = s.main[addr+7]
			}
		case 0x01, 0x02: // copy
			dstBank := int(v(7)) * 0x10000
			dstAddr := int(v(9))
			length = min(length, 0x10000-dstAddr)
			if dstBank < len(s.main) {
				copy(s.main[dstBank+dstAddr:dstBank+dstAddr+length], s.main[bank+srcAddr:bank+srcAddr+length])
			}
		}
	case 0xe:
		panic(uxn.Debug)
	}
}
