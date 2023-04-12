package main

import "github.com/nf/nux/uxn"

func NewVarvara(m *uxn.Machine) *Varvara {
	v := &Varvara{}
	v.fileA.main = m.Mem[:]
	v.fileB.main = m.Mem[:]
	return v
}

type Varvara struct {
	sys   System
	con   Console
	fileA File
	fileB File
}

func (v *Varvara) In(d byte) byte {
	dev := d & 0xf0
	d &= 0xf
	switch dev {
	case 0x00:
		return v.sys.In(d)
	case 0x10:
		return v.con.In(d)
	case 0xa0:
		return v.fileA.In(d)
	case 0xb0:
		return v.fileB.In(d)
	default:
		panic("device not implemented")
	}
}

func (v *Varvara) InShort(d byte) uint16 {
	return short(v.In(d), v.In(d+1))
}

func (v *Varvara) Out(d, b byte) {
	dev := d & 0xf0
	d &= 0xf
	switch dev {
	case 0x00:
		v.sys.Out(d, b)
	case 0x10:
		v.con.Out(d, b)
	case 0xa0:
		v.fileA.Out(d, b)
	case 0xb0:
		v.fileB.Out(d, b)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) OutShort(d byte, b uint16) {
	v.Out(d, byte(b>>8))
	v.Out(d+1, byte(b))
}

func (v *Varvara) Next() uint16 {
	return <-v.con.Next()
}

type deviceMem [16]byte

func (m *deviceMem) short(addr byte) uint16 {
	return short(m[addr], m[addr+1])
}

func (m *deviceMem) setShort(addr byte, v uint16) {
	m[addr] = byte(v >> 8)
	m[addr+1] = byte(v)
}

func short(hi, lo byte) uint16 {
	return uint16(hi)<<8 + uint16(lo)
}
