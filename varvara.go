package main

type Varvara struct {
	sys System
	con Console
}

func (v *Varvara) In(d byte) byte {
	switch d & 0x10 {
	case 0x00:
		return v.sys.In(d)
	case 0x10:
		return v.con.In(d)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) InShort(d byte) uint16 {
	switch d & 0x10 {
	case 0x00:
		return v.sys.InShort(d)
	case 0x10:
		return v.con.InShort(d)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) Out(d, b byte) {
	switch d & 0x10 {
	case 0x00:
		v.sys.Out(d, b)
	case 0x10:
		v.con.Out(d, b)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) OutShort(d byte, b uint16) {
	switch d & 0xF0 {
	case 0x00:
		v.sys.OutShort(d, b)
	case 0x10:
		v.con.OutShort(d, b)
	default:
		panic("not implemented")
	}
}

func (v *Varvara) Next() uint16 {
	return <-v.con.Next()
}
