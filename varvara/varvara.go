// Package varvara implements the Varvara computing stack.
package varvara

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nf/nux/uxn"
)

func Run(rom []byte, enableGUI bool, logf func(string, ...any)) (exitCode int) {
	m := uxn.NewMachine(rom)
	v := &Varvara{
		guiUpdate:     make(chan bool),
		guiUpdateDone: make(chan bool),
	}
	v.sys.Done = make(chan bool)
	v.scr.main = m.Mem[:]
	v.fileA.main = m.Mem[:]
	v.fileB.main = m.Mem[:]
	m.Dev = v

	var g *gui
	if enableGUI {
		g = &gui{Varvara: v}
		g.update()
	}

	vector := uint16(0x100)
	go func() {
		for {
			if err := m.ExecVector(vector, logf); err != nil {
				if _, ok := err.(uxn.HaltError); ok {
					if vector = v.sys.Halt(); vector > 0 {
						continue
					}
				}
				log.Fatalf("uxn.Machine.ExecVector: %v", err)
			}
			select {
			case <-v.con.Ready:
				vector = v.con.Vector()
			case v.guiUpdate <- true:
				<-v.guiUpdateDone
				vector = v.scr.Vector()
			}
		}
	}()

	if enableGUI {
		if err := ebiten.RunGame(g); err != nil {
			log.Fatalf("ebiten.RunGame: %v", err)
		}
	} else {
		<-v.sys.Done
	}

	return v.sys.ExitCode()
}

type Varvara struct {
	sys   System
	con   Console
	scr   Screen
	fileA File
	fileB File

	guiUpdate     chan bool
	guiUpdateDone chan bool
}

func (v *Varvara) In(p byte) byte {
	dev := p & 0xf0
	p &= 0xf
	switch dev {
	case 0x00:
		return v.sys.In(p)
	case 0x10:
		return v.con.In(p)
	case 0x20:
		return v.scr.In(p)
	case 0xa0:
		return v.fileA.In(p)
	case 0xb0:
		return v.fileB.In(p)
	default:
		return 0 // Unimplemented device.
	}
}

func (v *Varvara) InShort(p byte) uint16 {
	return short(v.In(p), v.In(p+1))
}

func (v *Varvara) Out(p, b byte) {
	dev := p & 0xf0
	p &= 0xf
	switch dev {
	case 0x00:
		v.sys.Out(p, b)
	case 0x10:
		v.con.Out(p, b)
	case 0x20:
		v.scr.Out(p, b)
	case 0xa0:
		v.fileA.Out(p, b)
	case 0xb0:
		v.fileB.Out(p, b)
	}
}

func (v *Varvara) OutShort(p byte, b uint16) {
	v.Out(p, byte(b>>8))
	v.Out(p+1, byte(b))
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
