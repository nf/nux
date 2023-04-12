package main

import "os"

type System struct{}

func (System) In(d byte) byte {
	panic("not implemented")
}

func (System) InShort(d byte) uint16 {
	panic("not implemented")
}

func (System) Out(d, b byte) {
	switch d {
	case 0x0f:
		if b != 0 {
			os.Exit(int(0x7f & b))
		}
	default:
		panic("not implemented")
	}
}

func (System) OutShort(d byte, b uint16) {
	switch d {
	case 0x0f:
		if b != 0 {
			os.Exit(int(0x7f & b))
		}
	default:
		panic("not implemented")
	}
}
