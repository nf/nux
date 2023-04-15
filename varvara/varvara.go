// Package varvara implements the Varvara computing stack.
package varvara

import (
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nf/nux/uxn"
)

func Run(rom []byte, enableGUI bool, logf func(string, ...any)) {
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
	m.ExecVector(0x100, logf)
	var g *gui
	if enableGUI {
		g = &gui{Varvara: v}
		g.update()
	}
	go func() {
		for {
			select {
			case <-v.con.Ready:
				m.ExecVector(v.con.Vector(), logf)
			case v.guiUpdate <- true:
				<-v.guiUpdateDone
				if addr := v.scr.Vector(); addr != 0 { // FIXME
					m.ExecVector(addr, logf)
				}
			}
		}
	}()
	if enableGUI {
		if err := ebiten.RunGame(g); err != nil {
			log.Fatalf("ebiten: %v", err)
		}
	} else {
		<-v.sys.Done
	}
	os.Exit(v.sys.ExitCode())
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

func (v *Varvara) In(d byte) byte {
	dev := d & 0xf0
	d &= 0xf
	switch dev {
	case 0x00:
		return v.sys.In(d)
	case 0x10:
		return v.con.In(d)
	case 0x20:
		return v.scr.In(d)
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
	case 0x20:
		v.scr.Out(d, b)
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
