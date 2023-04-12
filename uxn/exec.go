// Package uxn provides an implementation of a Uxn CPU, called Machine,
// that can be used to execute Uxn bytecode.
package uxn

import (
	"fmt"
)

// Machine is an implementation of a Uxn CPU.
type Machine struct {
	Mem  [64 << 10]byte
	PC   uint16
	Work Stack
	Ret  Stack
	Dev  Device
}

// Device provides access to external systems connected to the Uxn CPU.
type Device interface {
	In(device byte) (value byte)
	InShort(device byte) (value uint16)
	Out(device, value byte)
	OutShort(device byte, value uint16)
}

// Run loads the given rom into Mem at 0x100, sets PC to that same address,
// and executes instructions until it encouters BRK or #010f DEO.
func (m *Machine) Run(rom []byte, logf func(string, ...any)) {
	copy(m.Mem[0x100:], rom)
	m.PC = 0x100
	for m.exec(logf) {
	}
}

func (m *Machine) exec(logf func(string, ...any)) (cont bool) {
	op := Op(m.Mem[m.PC])
	logf("%x\t%v\t%v\t%v\n", m.PC, op, m.Work, m.Ret)
	m.PC++

	switch op {
	case BRK:
		return false
	case JCI, JMI, JSI:
		m.PC += 2
		if op == JCI && m.Work.wrap().Pop() == 0 {
			return true
		}
		if op == JSI {
			m.Ret.wrap().PushShort(m.PC)
		}
		m.PC += uint16(m.Mem[m.PC-2])<<8 + uint16(m.Mem[m.PC-1])
		return true
	}

	var st *stackWrapper
	if op.Return() {
		st = m.Ret.mutate(op.Keep())
	} else {
		st = m.Work.mutate(op.Keep())
	}

	switch op.Base() {
	case LIT:
		st.Push(m.Mem[m.PC])
		m.PC++
		if op.Short() {
			st.Push(m.Mem[m.PC])
			m.PC++
		}
	case JMP, JSR:
		pc := m.PC
		if op.Short() {
			m.PC = st.PopShort()
		} else {
			m.PC += st.PopOffset()
		}
		if op.Base() == JSR {
			m.Ret.wrap().PushShort(pc)
		}
	case JCN:
		var addr uint16
		if op.Short() {
			addr = st.PopShort()
		} else {
			addr = m.PC + st.PopOffset()
		}
		if st.Pop() != 0 {
			m.PC = addr
		}
	case STH:
		var to *stackWrapper
		if op.Return() {
			to = m.Work.wrap()
		} else {
			to = m.Ret.wrap()
		}
		if op.Short() {
			to.PushShort(st.PopShort())
		} else {
			to.Push(st.Pop())
		}
	case LDZ:
		addr := st.Pop()
		st.Push(m.Mem[addr])
		if op.Short() {
			st.Push(m.Mem[addr+1])
		}
	case STZ:
		addr := st.Pop()
		if op.Short() {
			m.Mem[addr+1] = st.Pop()
		}
		m.Mem[addr] = st.Pop()
	case LDR:
		offs := st.PopOffset()
		st.Push(m.Mem[m.PC+offs])
		if op.Short() {
			st.Push(m.Mem[m.PC+offs+1])
		}
	case STR:
		offs := st.PopOffset()
		if op.Short() {
			m.Mem[m.PC+offs+1] = st.Pop()
		}
		m.Mem[m.PC+offs] = st.Pop()
	case LDA:
		addr := st.PopShort()
		st.Push(m.Mem[addr])
		if op.Short() {
			st.Push(m.Mem[addr+1])
		}
	case STA:
		addr := st.PopShort()
		if op.Short() {
			m.Mem[addr+1] = st.Pop()
		}
		m.Mem[addr] = st.Pop()
	case DEI:
		dev := st.Pop()
		if op.Short() {
			st.PushShort(m.Dev.InShort(dev))
		} else {
			st.Push(m.Dev.In(dev))
		}
	case DEO:
		dev := st.Pop()
		if op.Short() {
			m.Dev.OutShort(dev, st.PopShort())
		} else {
			m.Dev.Out(dev, st.Pop())
		}
	case SFT:
		sft := st.Pop()
		left, right := (sft&0xf0)>>4, sft&0x0f
		if op.Short() {
			st.PushShort((st.PopShort() >> right) << left)
		} else {
			st.Push((st.Pop() >> right) << left)
		}
	default:
		if op.Short() {
			execSimple(op, pushPopper[uint16](shortPushPopper{st}))
		} else {
			execSimple(op, pushPopper[byte](st))
		}
	}

	return true
}

func execSimple[T byte | uint16](op Op, s pushPopper[T]) {
	switch op.Base() {
	case INC:
		s.Push(s.Pop() + 1)
	case POP:
		s.Pop()
	case NIP:
		v := s.Pop()
		s.Pop()
		s.Push(v)
	case SWP:
		b, a := s.Pop(), s.Pop()
		s.Push(b)
		s.Push(a)
	case ROT:
		c, b, a := s.Pop(), s.Pop(), s.Pop()
		s.Push(b)
		s.Push(c)
		s.Push(a)
	case DUP:
		v := s.Pop()
		s.Push(v)
		s.Push(v)
	case OVR:
		b, a := s.Pop(), s.Pop()
		s.Push(a)
		s.Push(b)
		s.Push(a)
	case EQU:
		s.PushBool(s.Pop() == s.Pop())
	case NEQ:
		s.PushBool(s.Pop() != s.Pop())
	case GTH:
		s.PushBool(s.Pop() < s.Pop())
	case LTH:
		s.PushBool(s.Pop() > s.Pop())
	case ADD:
		s.Push(s.Pop() + s.Pop())
	case SUB:
		b, a := s.Pop(), s.Pop()
		s.Push(a - b)
	case MUL:
		s.Push(s.Pop() * s.Pop())
	case DIV:
		b, a := s.Pop(), s.Pop()
		s.Push(a / b)
	case AND:
		s.Push(s.Pop() & s.Pop())
	case ORA:
		s.Push(s.Pop() | s.Pop())
	case EOR:
		s.Push(s.Pop() ^ s.Pop())
	default:
		panic(fmt.Errorf("%v not implemented", op))
	}
}

// Nopf is a logf function that does nothing.
func Nopf(string, ...any) {}
