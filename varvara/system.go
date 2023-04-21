package varvara

import (
	"log"

	"github.com/nf/nux/uxn"
)

type System struct {
	mem  deviceMem
	main []byte
	m    *uxn.Machine
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
	case 0x3:
		switch addr := s.mem.short(0x2); s.main[addr] {
		case 0x01: // copy
			var (
				v = func() uint16 {
					addr += 2
					return short(s.main[addr-1], s.main[addr])
				}
				size    = int(v())
				src     = int(v()%0x10) * 0x10000
				srcAddr = v()
				dst     = int(v()%0x10) * 0x10000
				dstAddr = v()
			)
			for i := 0; i < size; i++ {
				s.main[dst+int(dstAddr+uint16(i))] = s.main[src+int(srcAddr+uint16(i))]
			}
		}
	case 0xe:
		log.Printf("%x\t%v\t%v\n", s.m.PC, s.m.Work, s.m.Ret)
	case 0xf:
		if b != 0 {
			panic(uxn.Halt) // Stop execution.
		}
	}
}
