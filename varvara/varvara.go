// Package varvara implements the Varvara computing stack.
package varvara

import (
	"log"

	"github.com/nf/nux/uxn"
)

func Run(rom []byte, enableGUI bool, logf func(string, ...any)) (exitCode int) {
	m := uxn.NewMachine(rom)
	v := &Varvara{}
	v.scr.main = m.Mem[:]
	v.scr.sys = &v.sys
	v.scr.setWidth(0x100)
	v.scr.setHeight(0x100)
	v.fileA.main = m.Mem[:]
	v.fileB.main = m.Mem[:]
	m.Dev = v

	halt := make(chan bool)

	var g *gui
	if enableGUI {
		v.guiUpdate = make(chan bool)
		v.guiUpdateDone = make(chan bool)
		g = newGUI(v)
	}

	vector := uint16(0x100)
	go func() {
		for {
			if err := m.ExecVector(vector, logf); err != nil {
				if h, ok := err.(uxn.HaltError); ok {
					if h.HaltCode == uxn.Halt {
						close(halt)
						return
					}
					if vector = v.sys.Halt(); vector > 0 {
						continue
					}
				}
				log.Fatalf("uxn.Machine.ExecVector: %v", err)
			}
			for vector = 0; vector == 0; {
				select {
				case <-v.con.Ready:
					vector = v.con.Vector()
				case <-v.cntrl.Ready:
					vector = v.cntrl.Vector()
				case <-v.mouse.Ready:
					vector = v.mouse.Vector()
				case v.guiUpdate <- true:
					<-v.guiUpdateDone
					vector = v.scr.Vector()
				}
			}
		}
	}()

	if g != nil {
		// If the GUI is enabled then Run will drive the GUI and the
		// screen vector until halt is closed.
		if err := g.Run(halt); err != nil {
			log.Fatalf("gui: %v", err)
		}
	} else {
		<-halt
	}

	return v.sys.ExitCode()
}

type Varvara struct {
	sys   System
	con   Console
	scr   Screen
	cntrl Controller
	mouse Mouse
	fileA File
	fileB File
	time  Datetime

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
	case 0x80:
		return v.cntrl.In(p)
	case 0x90:
		return v.mouse.In(p)
	case 0xa0:
		return v.fileA.In(p)
	case 0xb0:
		return v.fileB.In(p)
	case 0xc0:
		return v.time.In(p)
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
	case 0x80:
		v.cntrl.Out(p, b)
	case 0x90:
		v.mouse.Out(p, b)
	case 0xa0:
		v.fileA.Out(p, b)
	case 0xb0:
		v.fileB.Out(p, b)
	case 0xc0:
		v.time.Out(p, b)
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

func (m *deviceMem) setChanged(addr, v byte) bool {
	if v == m[addr] {
		return false
	}
	m[addr] = v
	return true
}

func (m *deviceMem) setShortChanged(addr byte, v uint16) bool {
	if v == m.short(addr) {
		return false
	}
	m.setShort(addr, v)
	return true
}

func short(hi, lo byte) uint16 {
	return uint16(hi)<<8 + uint16(lo)
}
