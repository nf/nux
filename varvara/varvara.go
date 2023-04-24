// Package varvara implements the Varvara computing stack.
package varvara

import (
	"log"
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
}

type StateFunc func(m *uxn.Machine, halt bool)

func NewRunner(enableGUI, devMode bool, state StateFunc) *Runner {
	r := &Runner{
		gui:      enableGUI,
		dev:      devMode,
		state:    state,
		swap:     make(chan []byte),
		swapDone: make(chan bool),
		debug:    make(chan debugOp),
	}
	return r
}

type debugOp struct {
	cmd  string
	addr uint16
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
	var (
		v    = New(rom, r.state)
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
			log.Printf("unx: started")
		}
		halt := func() {
			if running {
				v.Halt()
				if err := <-execErr; err != nil {
					log.Printf("uxn: stopped: %v", err)
				} else {
					log.Printf("uxn: stopped")
				}
				running = false
			}
		}
		exec()
		for {
			select {
			case rom = <-r.swap:
				halt()
				bp := v.breakpoint
				v = New(rom, r.state)
				v.breakpoint = bp
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
				case "halt", "h":
					halt()
				case "reset", "r":
					halt()
					bp := v.breakpoint
					v = New(rom, r.state)
					v.breakpoint = bp
					g.Swap(v)
					exec()
				case "pause", "p":
					v.Pause()
				case "step", "s":
					v.Step()
				case "cont", "c":
					v.Continue()
				case "bp":
					v.SetBreakpoint(op.addr)
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

	interrupt  int32
	breakpoint int32
	halted     bool
	halt       chan bool
	cont       chan bool
}

func New(rom []byte, state StateFunc) *Varvara {
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
	v.Pause()
}

func (v *Varvara) Pause() {
	atomic.StoreInt32(&v.interrupt, 1)
}

func (v *Varvara) Continue() {
	atomic.StoreInt32(&v.interrupt, 0)
	select {
	case v.cont <- true:
	default:
	}
}

func (v *Varvara) Step() {
	atomic.StoreInt32(&v.interrupt, 1)
	select {
	case v.cont <- true:
	default:
	}
}

func (v *Varvara) SetBreakpoint(addr uint16) {
	atomic.StoreInt32(&v.breakpoint, int32(addr))
}

func (v *Varvara) Exec(g *GUI) error {
	for {
		for {
			if uint16(atomic.LoadInt32(&v.breakpoint)) == v.m.PC ||
				atomic.LoadInt32(&v.interrupt) != 0 {
				v.state(v.m, false)
				select {
				case <-v.halt:
					return nil
				case <-v.cont:
				}
			}
			if err := v.m.Exec(); err == uxn.ErrBRK {
				break
			} else if err != nil {
				v.state(v.m, true)
				if h, ok := err.(uxn.HaltError); ok {
					if h.HaltCode == uxn.Halt {
						return nil
					}
					if vec := v.sys.Halt(); vec > 0 {
						v.m.PC = vec
						continue
					}
				}
				return err
			}
		}
		v.state(nil, false)

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

type backlog struct {
	entries []logEntry
	n       int
}

type logEntry struct {
	format string
	args   []any
}

const maxBacklog = 100

func (b *backlog) LazyPrintf(format string, args ...any) {
	if b.n < len(b.entries) {
		b.entries[b.n] = logEntry{format, args}
	} else {
		b.entries = append(b.entries, logEntry{format, args})
	}
	b.n = (b.n + 1) % maxBacklog
}

func (b *backlog) Emit() {
	if len(b.entries) == 0 {
		return
	}
	for i := b.n; ; i++ {
		i %= len(b.entries)
		log.Printf(b.entries[i].format, b.entries[i].args...)
		if (i+1)%maxBacklog == b.n {
			break
		}
	}
}

func (b *backlog) Reset() {
	b.entries = b.entries[:0]
	b.n = 0
}
