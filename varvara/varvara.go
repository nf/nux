// Package varvara implements the Varvara computing stack.
package varvara

import (
	"io"
	"log"
	"os"
	"sync/atomic"

	"github.com/nf/nux/uxn"
)

type Runner struct {
	gui   bool
	dev   bool
	state StateFunc

	swap     chan []byte
	swapDone chan bool
	debug    chan debugOp

	stdout, stderr io.Writer
}

type StateFunc func(*uxn.Machine, StateKind)

type StateKind byte

const (
	ClearState StateKind = iota
	HaltState
	PauseState
	BreakState
	DebugState
	QuietState
)

func NewRunner(enableGUI, devMode bool, state StateFunc) *Runner {
	if state == nil {
		state = func(*uxn.Machine, StateKind) {}
	}
	return &Runner{
		gui:      enableGUI,
		dev:      devMode,
		state:    state,
		swap:     make(chan []byte),
		swapDone: make(chan bool),
		debug:    make(chan debugOp),

		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

type debugOp struct {
	cmd  string
	addr uint16
}

func (r *Runner) SetOutput(w io.Writer) {
	r.stdout = w
	r.stderr = w
}

func (r *Runner) Debug(cmd string, addr uint16) { r.debug <- debugOp{cmd, addr} }

func (r *Runner) Swap(rom []byte) {
	if !r.dev {
		panic("Reset called while not running in dev mode")
	}
	r.swap <- rom
	<-r.swapDone
}

func (r *Runner) Run(rom []byte) (exitCode int) {
	var v *Varvara
	newV := func() {
		prev := v
		v = New(rom, r.state, r.stdout, r.stderr)
		if prev != nil {
			v.breakAddr, v.debugAddr = prev.breakAddr, prev.debugAddr
		}
		v.state(v.m, ClearState)
	}
	newV()
	var (
		g    = NewGUI(v)
		exit = make(chan bool)
	)
	go func() {
		var (
			execErr = make(chan error)
			running = false
		)
		exec := func() {
			if running {
				return
			}
			running = true
			go func() { execErr <- v.Exec(g) }()
			if r.dev {
				log.Printf("uxn: started")
			}
		}
		halt := func() {
			if !running {
				return
			}
			running = false
			v.Halt()
			if err := <-execErr; err != nil {
				log.Printf("uxn: stopped: %v", err)
			} else {
				log.Printf("uxn: stopped")
			}
		}
		exec()
		for {
			select {
			case rom = <-r.swap:
				halt()
				newV()
				g.Swap(v)
				exec()
				r.swapDone <- true
			case err := <-execErr:
				running = false
				if r.dev {
					if err != nil {
						log.Printf("uxn: stopped: %v", err)
					} else {
						log.Printf("uxn: stopped")
					}
				} else {
					close(exit)
					return
				}
			case op := <-r.debug:
				switch op.cmd {
				case "halt":
					halt()
				case "reset":
					halt()
					newV()
					g.Swap(v)
					exec()
				case "step":
					v.Step()
				case "cont":
					v.Continue()
				case "break":
					v.SetBreak(op.addr)
				case "debug":
					v.SetDebug(op.addr)
				case "exit":
					halt()
					close(exit)
					return
				}
			}
		}
	}()
	if r.gui {
		// If the GUI is enabled then Run will drive the GUI and the
		// screen vector until exit is closed.
		if err := g.Run(exit); err != nil {
			log.Fatalf("gui: %v", err)
		}
	} else {
		<-exit
	}
	return v.sys.ExitCode()
}

type Varvara struct {
	m     *uxn.Machine
	sys   System
	con   Console
	scr   Screen
	cntrl Controller
	mouse Mouse
	fileA File
	fileB File
	time  Datetime

	state StateFunc

	paused    int32
	breakAddr int32
	debugAddr int32
	halted    bool
	halt      chan bool
	cont      chan bool
}

func New(rom []byte, state StateFunc, stdout, stderr io.Writer) *Varvara {
	m := uxn.NewMachine(rom)
	v := &Varvara{
		m:     m,
		state: state,
		halt:  make(chan bool),
		cont:  make(chan bool),
	}
	m.Dev = v
	v.sys.main = m.Mem[:]
	v.sys.m = m
	v.sys.state = state
	v.con.in = os.Stdin
	v.con.out = stdout
	v.con.err = stderr
	v.scr.main = m.Mem[:]
	v.scr.sys = &v.sys
	v.scr.setWidth(0x100)
	v.scr.setHeight(0x100)
	v.fileA.main = m.Mem[:]
	v.fileB.main = m.Mem[:]
	return v
}

func (v *Varvara) Halt() {
	if !v.halted {
		close(v.halt)
		v.halted = true
	}
	atomic.StoreInt32(&v.paused, 1)
}

func (v *Varvara) Continue() {
	atomic.StoreInt32(&v.paused, 0)
	select {
	case v.cont <- true:
	default:
	}
}

func (v *Varvara) Step() {
	atomic.StoreInt32(&v.paused, 1)
	select {
	case v.cont <- true:
	default:
	}
}

func (v *Varvara) SetBreak(addr uint16) {
	atomic.StoreInt32(&v.breakAddr, int32(addr))
}

func (v *Varvara) SetDebug(addr uint16) {
	atomic.StoreInt32(&v.debugAddr, int32(addr))
}

func (v *Varvara) Exec(g *GUI) error {
	defer v.state(v.m, HaltState)
	for {
		clear, quiet := false, true
		for {
			wait := false
			switch {
			case atomic.LoadInt32(&v.paused) != 0:
				v.state(v.m, PauseState)
				wait = true
			case uint16(atomic.LoadInt32(&v.breakAddr)) == v.m.PC:
				v.state(v.m, BreakState)
				wait = true
			case uint16(atomic.LoadInt32(&v.debugAddr)) == v.m.PC:
				v.state(v.m, DebugState)
				// Don't send quiet state because we already
				// sent debug state, and we want the watches to
				// reflect the debug state.
				quiet = false
			}
			if wait {
				select {
				case <-v.halt:
					return nil
				case <-v.cont:
				}
				// Send the clear state after we resume.
				clear, quiet = true, false
			}
			if err := v.m.Exec(); err == uxn.ErrBRK {
				break
			} else if err != nil {
				h, ok := err.(uxn.HaltError)
				if ok && h.HaltCode == uxn.Debug {
					v.state(v.m, DebugState)
					continue
				}
				if ok {
					if h.HaltCode == uxn.Halt {
						return nil
					}
					if vec := v.sys.Halt(); vec > 0 {
						v.m.Work.Ptr = 4
						v.m.Work.Bytes[0] = byte(h.Addr >> 8)
						v.m.Work.Bytes[1] = byte(h.Addr)
						v.m.Work.Bytes[2] = byte(h.Op)
						v.m.Work.Bytes[3] = byte(h.HaltCode)
						v.m.Ret.Ptr = 0
						v.m.PC = vec
						continue
					}
				}
				return err
			}
		}
		if quiet {
			v.state(v.m, QuietState)
		} else if clear {
			v.state(v.m, ClearState)
		}

		var vector uint16
		for vector == 0 {
			select {
			case <-v.con.Ready:
				vector = v.con.Vector()
			case <-v.cntrl.Ready:
				vector = v.cntrl.Vector()
			case <-v.mouse.Ready:
				vector = v.mouse.Vector()
			case g.Update <- true:
				<-g.UpdateDone
				vector = v.scr.Vector()
			case <-v.halt:
				return nil
			}
		}
		v.m.PC = vector
	}
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
