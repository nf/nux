package main

import "github.com/nf/nux/uxn"

func NewVarvara(m *uxn.Machine) *Varvara {
	v := &Varvara{}
	v.fileA.mem = m.Mem[:]
	v.fileB.mem = m.Mem[:]
	return v
}

type Varvara struct {
	sys   System
	con   Console
	fileA File
	fileB File
}

func (v *Varvara) In(d byte) byte {
	switch d & 0xf0 {
	case 0x00:
		return v.sys.In(d)
	case 0x10:
		return v.con.In(d)
	case 0xa0:
		return v.fileA.In(d)
	case 0xb0:
		return v.fileB.In(d)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) InShort(d byte) uint16 {
	switch d & 0xf0 {
	case 0x00:
		return v.sys.InShort(d)
	case 0x10:
		return v.con.InShort(d)
	case 0xa0:
		return v.fileA.InShort(d)
	case 0xb0:
		return v.fileB.InShort(d)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) Out(d, b byte) {
	switch d & 0xf0 {
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
	switch d & 0xf0 {
	case 0x00:
		v.sys.OutShort(d, b)
	case 0x10:
		v.con.OutShort(d, b)
	case 0xa0:
		v.fileA.OutShort(d, b)
	case 0xb0:
		v.fileB.OutShort(d, b)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) Next() uint16 {
	return <-v.con.Next()
}
