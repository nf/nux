package main

import "os"

type System struct{}

func (System) In(d byte) byte {
	return 0
}

func (System) InShort(d byte) uint16 {
	return 0
}

func (System) Out(d, b byte) {
	d &= 0xf
	switch d {
	case 0xf:
		if b != 0 {
			os.Exit(int(0x7f & b))
		}
	}
}
