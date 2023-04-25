// Package uxn provides an implementation of a Uxn CPU, called Machine,
// that can be used to execute Uxn bytecode.
package uxn

import (
	"errors"
	"fmt"
)

// Machine is an implementation of a Uxn CPU.
type Machine struct {
	Mem  [0x100000]byte
	PC   uint16
	Work Stack
	Ret  Stack
	Dev  Device
}

// Device provides access to external systems connected to the Uxn CPU.
type Device interface {
	In(port byte) (value byte)
	InShort(port byte) (value uint16)
	Out(port, value byte)
	OutShort(port byte, value uint16)
}

// NewMachine returns a Uxn CPU loaded with the given rom at 0x100.
// If the rom is larger than 0xfeff bytes then Mem is grown to accommodate it,
// even though the Uxn CPU cannot directly address the extra memory.
func NewMachine(rom []byte) *Machine {
	m := &Machine{PC: 0x100}
	copy(m.Mem[0x100:], rom)
	return m
}

var ErrBRK = errors.New("BRK")

// Exec executes the intsruction at m.PC. It returns ErrBRK if that instruction
// is BRK, and otherwise only returns a non-nil error if it encounters a halt
// condition.
func (m *Machine) Exec() (err error) {
	var (
		op   = Op(m.Mem[m.PC])
		opPC = m.PC
	)
	defer func() {
		if e := recover(); e != nil {
			if code, ok := e.(HaltCode); ok {
				err = HaltError{
					Addr:     opPC,
					Op:       op,
					HaltCode: code,
				}
			} else {
				panic(e)
			}
		}
	}()

	m.PC++

	switch op {
	case BRK:
		return ErrBRK
	case JCI, JMI, JSI:
		m.PC += 2
		if op == JCI && m.Work.wrap().Pop() == 0 {
			return nil
		}
		if op == JSI {
			m.Ret.wrap().PushShort(m.PC)
		}
		m.PC += short(m.Mem[m.PC-2], m.Mem[m.PC-1])
		return nil
	}

	var st *stackWrapper
	if op.Return() {
		st = m.Ret.keep(op.Keep())
	} else {
		st = m.Work.keep(op.Keep())
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
		port := st.Pop()
		if op.Short() {
			st.PushShort(m.Dev.InShort(port))
		} else {
			st.Push(m.Dev.In(port))
		}
	case DEO:
		port := st.Pop()
		if op.Short() {
			m.Dev.OutShort(port, st.PopShort())
		} else {
			m.Dev.Out(port, st.Pop())
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

	return nil
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
		if b == 0 {
			panic(DivideByZero)
		}
		s.Push(a / b)
	case AND:
		s.Push(s.Pop() & s.Pop())
	case ORA:
		s.Push(s.Pop() | s.Pop())
	case EOR:
		s.Push(s.Pop() ^ s.Pop())
	default:
		panic(fmt.Errorf("internal error: %v not implemented", op))
	}
}

// OpAddr returns the address associated with the operation at addr, either
// from memory or the stack, and reports whether the operation has an
// associated address (for example, JMP does while POP does not), and whether
// there are enough bytes on the stack to provide an address (ie, it returns
// false if executing the instruction would trigger a stack underflow).
func (m *Machine) OpAddr(addr uint16) (uint16, bool) {
	switch op := Op(m.Mem[addr]); op.Base() {
	case JCI, JMI, JSI:
		return m.PC + short(m.Mem[m.PC+1], m.Mem[m.PC+2]) + 3, true
	case JMP, JCN, JSR, LDR, STR, LDA, STA, LDZ, STZ, DEI, DEO:
		var st *Stack
		if op.Return() {
			st = &m.Ret
		} else {
			st = &m.Work
		}
		switch op.Base() {
		case JMP, JCN, JSR:
			if op.Short() { // addr16 abs
				return st.PeekShort()
			} else { // addr8 rel
				offs, ok := st.PeekOffset()
				return m.PC + offs + 1, ok
			}
		case LDR, STR: // addr8 rel
			offs, ok := st.PeekOffset()
			return m.PC + offs + 1, ok
		case LDA, STA: // addr16 abs
			return st.PeekShort()
		case LDZ, STZ, DEI, DEO: // addr8 zero, device8
			addr, ok := st.Peek()
			return uint16(addr), ok
		}
	}
	return 0, false
}

// HaltError is returned by Exec if execution is halted by
// the program for some reason.
type HaltError struct {
	HaltCode
	Op   Op
	Addr uint16
}

func (e HaltError) Error() string {
	return fmt.Sprintf("%s executing %s at %.4x", e.HaltCode, e.Op, e.Addr)
}

// HaltCode signifies the type of condition that halted execution.
type HaltCode byte

const (
	Halt         HaltCode = 0x00
	Underflow    HaltCode = 0x01
	Overflow     HaltCode = 0x02
	DivideByZero HaltCode = 0x03

	// Debug is treated differently by Exec, which will still return a
	// HaltError but the stacks are left unchanged and the program may
	// cotinue to execute normally.
	Debug HaltCode = 0xff
)

func (c HaltCode) String() string {
	if s, ok := map[HaltCode]string{
		Halt:         "halt",
		Underflow:    "stack underflow",
		Overflow:     "stack overflow",
		DivideByZero: "division by zero",
	}[c]; ok {
		return s
	}
	return fmt.Sprintf("unknown (%.2x)", byte(c))
}

func short(hi, lo byte) uint16 {
	return uint16(hi)<<8 + uint16(lo)
}
