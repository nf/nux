package uxn

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNewMachine(t *testing.T) {
	for _, c := range []struct {
		romSize int
	}{
		{0x00000},
		{0x00001},
		{0x0feff},
		{0x0ff00},
		{0x0ff01},
		{0x1ff00},
	} {
		t.Run(fmt.Sprintf("%.5x", c.romSize), func(t *testing.T) {
			m := NewMachine(bytes.Repeat([]byte{1}, c.romSize))
			for i := range m.Mem {
				w := byte(0)
				if i >= 0x100 && i < 0x100+c.romSize {
					w = 1
				}
				if g := m.Mem[i]; g != w {
					t.Errorf("Mem[%.5x] == %.2x, want %.2x", i, g, w)
				}
			}
		})
	}
}

func TestExec(t *testing.T) {
	c := newExecTestCase
	for i, c := range []*execTestCase{
		c(INC).work(1, 2).want().work(1, 3),
		c(INC).work(0xff).want().work(0),
		c(INCk).work(1, 2).want().work(1, 2, 3),
		c(INC2).work(1, 2).want().work(1, 3),
		c(INC2).work(0, 0xff).want().work(1, 0),
		c(INC2).work(0xff, 0xff).want().work(0, 0),
		c(INC2k).work(1, 2).want().work(1, 2, 1, 3),
		c(INC2k).work(0, 0xff).want().work(0, 0xff, 1, 0),

		c(POP).work(1, 2).want().work(1),
		c(POPk).work(1, 2).want().work(1, 2),
		c(POP2).work(1, 2),
		c(POP2k).work(1, 2).want().work(1, 2),

		c(NIP).work(1, 2).want().work(2),
		c(NIP2).work(1, 2, 3, 4).want().work(3, 4),
		c(NIP2k).work(1, 2, 3, 4).want().work(1, 2, 3, 4, 3, 4),

		c(SWP).work(1, 2).want().work(2, 1),
		c(SWPk).work(1, 2).want().work(1, 2, 2, 1),
		c(SWP2).work(1, 2, 3, 4).want().work(3, 4, 1, 2),

		c(ROT).work(1, 2, 3).want().work(2, 3, 1),
		c(ROTk).work(1, 2, 3).want().work(1, 2, 3, 2, 3, 1),
		c(ROT2).work(1, 2, 3, 4, 5, 6).want().work(3, 4, 5, 6, 1, 2),
		c(ROT2k).work(1, 2, 3, 4, 5, 6).want().work(1, 2, 3, 4, 5, 6, 3, 4, 5, 6, 1, 2),

		c(DUP).work(1, 2).want().work(1, 2, 2),
		c(DUPk).work(1).want().work(1, 1, 1),
		c(DUP2).work(1, 2).want().work(1, 2, 1, 2),

		c(OVR).work(1, 2).want().work(1, 2, 1),
		c(OVRk).work(1, 2).want().work(1, 2, 1, 2, 1),
		c(OVR2).work(1, 2, 3, 4).want().work(1, 2, 3, 4, 1, 2),
		c(OVR2k).work(1, 2, 3, 4).want().work(1, 2, 3, 4, 1, 2, 3, 4, 1, 2),

		c(EQU).work(42, 42).want().work(1),
		c(EQU).work(1, 2).want().work(0),
		c(EQUk).work(42, 42).want().work(42, 42, 1),
		c(EQUk).work(1, 2).want().work(1, 2, 0),
		c(EQU2).work(1, 2, 1, 2).want().work(1),
		c(EQU2).work(1, 2, 3, 4).want().work(0),
		c(EQU2k).work(1, 2, 1, 2).want().work(1, 2, 1, 2, 1),
		c(EQU2k).work(1, 2, 3, 4).want().work(1, 2, 3, 4, 0),

		c(NEQ).work(42, 42).want().work(0),
		c(NEQ).work(1, 2).want().work(1),
		c(NEQk).work(42, 42).want().work(42, 42, 0),
		c(NEQk).work(1, 2).want().work(1, 2, 1),
		c(NEQ2).work(1, 2, 1, 2).want().work(0),
		c(NEQ2).work(1, 2, 3, 4).want().work(1),
		c(NEQ2k).work(1, 2, 1, 2).want().work(1, 2, 1, 2, 0),
		c(NEQ2k).work(1, 2, 3, 4).want().work(1, 2, 3, 4, 1),

		c(GTH).work(1, 2).want().work(0),
		c(GTH).work(1, 1).want().work(0),
		c(GTH).work(2, 1).want().work(1),
		c(GTHk).work(2, 1).want().work(2, 1, 1),
		c(GTH2).work(1, 2, 1, 3).want().work(0),
		c(GTH2).work(1, 2, 1, 2).want().work(0),
		c(GTH2).work(1, 3, 1, 2).want().work(1),
		c(GTH2k).work(1, 3, 1, 2).want().work(1, 3, 1, 2, 1),

		c(LTH).work(1, 2).want().work(1),
		c(LTH).work(1, 1).want().work(0),
		c(LTH).work(2, 1).want().work(0),
		c(LTHk).work(2, 1).want().work(2, 1, 0),
		c(LTH2).work(1, 2, 1, 3).want().work(1),
		c(LTH2).work(1, 2, 1, 2).want().work(0),
		c(LTH2).work(1, 3, 1, 2).want().work(0),
		c(LTH2k).work(1, 3, 1, 2).want().work(1, 3, 1, 2, 0),

		c(ADD).work(1, 2).want().work(3),
		c(SUB).work(3, 2).want().work(1),
		c(MUL).work(2, 3).want().work(6),
		c(DIV).work(6, 3).want().work(2),
		c(AND).work(0x99, 0xb8).want().work(0x98),
		c(ORA).work(0x36, 0x63).want().work(0x77),
		c(EOR).work(0x31, 0x13).want().work(0x22),

		c(JCI).mem(0x101, 2, 2).work(0).want().pc(0x103),
		c(JCI).mem(0x101, 7, 5).work(1).want().pc(0x808),

		c(JMI).mem(0x101, 7, 5).want().pc(0x808),

		c(JSI).mem(0x101, 7, 5).want().ret(1, 3).pc(0x808),

		c(LIT).mem(0x101, 1).want().work(1).pc(0x102),
		c(LIT2).mem(0x101, 1, 2).want().work(1, 2).pc(0x103),

		c(JMP).work(1).want().pc(0x102),
		c(JMPk).work(1).want().work(1).pc(0x102),
		c(JMP).work(rel(-2)).want().pc(0xff),
		c(JMP2).work(3, 4).want().pc(0x304),
		c(JMP2k).work(3, 4).want().work(3, 4).pc(0x304),

		c(JCN).work(0, 4).want(),
		c(JCN).work(1, 4).want().pc(0x105),
		c(JCNk).work(1, 4).want().work(1, 4).pc(0x105),
		c(JCN2).work(0, 2, 7).want(),
		c(JCN2).work(1, 2, 7).want().pc(0x207),
		c(JCN2k).work(1, 2, 7).want().work(1, 2, 7).pc(0x207),

		c(JSR).work(4).want().ret(1, 1).pc(0x105),
		c(JSRk).work(4).want().work(4).ret(1, 1).pc(0x105),
		c(JSR2).work(2, 7).want().ret(1, 1).pc(0x207),
		c(JSR2k).work(2, 7).want().work(2, 7).ret(1, 1).pc(0x207),

		c(STH).work(7).want().ret(7),
		c(STHr).ret(7).want().work(7),
		c(STHk).work(7).want().work(7).ret(7),
		c(STH2).work(7, 8).want().ret(7, 8),
		c(STH2r).ret(7, 8).want().work(7, 8),
		c(STH2k).work(7, 8).want().work(7, 8).ret(7, 8),

		c(LDZ).mem(0x71, 0x42).work(0x71).want().work(0x42),
		c(LDZ2).mem(0x71, 0x42, 0x69).work(0x71).want().work(0x42, 0x69),
		c(STZ).work(0x42, 0x71).want().mem(0x71, 0x42),
		c(STZ2).work(0x42, 0x69, 0x71).want().mem(0x71, 0x42, 0x69),

		c(LDR).mem(0xf1, 0x42).work(rel(-16)).want().work(0x42),
		c(LDR2).mem(0xf1, 0x42, 0x69).work(rel(-16)).want().work(0x42, 0x69),

		c(STR).work(0x42, rel(-16)).want().mem(0xf1, 0x42),
		c(STR2).work(0x42, 0x69, rel(-16)).want().mem(0xf1, 0x42, 0x69),

		c(LDA).mem(0x109, 0x42).work(0x01, 0x09).want().work(0x42),
		c(LDA2).mem(0x109, 0x42, 0x69).work(0x01, 0x09).want().work(0x42, 0x69),

		c(STA).work(0x42, 0x1, 0x09).want().mem(0x109, 0x42),
		c(STA2).work(0x42, 0x69, 0x1, 0x09).want().mem(0x109, 0x42, 0x69),

		c(SFT).work(9, 0x21).want().work(16),
		c(SFT2).work(1, 9, 0x21).want().work(2, 16),

		c(DIV).work(1, 2, 0).want().work(1, 0, byte(DIV), byte(DivideByZero)).
			error(HaltError{HaltCode: DivideByZero, Op: DIV, Addr: 0x100}),
		c(POP).work().want().work(1, 0, byte(POP), byte(Underflow)).
			error(HaltError{HaltCode: Underflow, Op: POP, Addr: 0x100}),
		c(POP2).work(42).want().work(1, 0, byte(POP2), byte(Underflow)).
			error(HaltError{HaltCode: Underflow, Op: POP2, Addr: 0x100}),
		c(POP2k).work(42).want().work(1, 0, byte(POP2k), byte(Underflow)).
			error(HaltError{HaltCode: Underflow, Op: POP2k, Addr: 0x100}),
		c(DUP).work(bytes.Repeat([]byte{7}, 255)...).want().
			work(1, 0, byte(DUP), byte(Overflow)).
			error(HaltError{HaltCode: Overflow, Op: DUP, Addr: 0x100}),
		c(DUP2).work(bytes.Repeat([]byte{7}, 254)...).want().
			work(1, 0, byte(DUP2), byte(Overflow)).
			error(HaltError{HaltCode: Overflow, Op: DUP2, Addr: 0x100}),
	} {
		t.Run(fmt.Sprintf("%s_%d", Op(c.m.Mem[0x100]), i), func(t *testing.T) {
			if err := c.m.exec(Nopf); err != c.err {
				t.Fatalf("got error %v, want %v", err, c.err)
			}
			if g, w := c.m.Work, c.w.Work; !stackEq(g, w) {
				t.Errorf("work stack is\n\t%v\nwant\n\t%v", g, w)
			}
			if g, w := c.m.Ret, c.w.Ret; !stackEq(g, w) {
				t.Errorf("return stack is %v, want %v", g, w)
			}
			if g, w := c.m.Mem, c.w.Mem; g != w {
				for i := 0; i < len(g) && i < len(w); i++ {
					if g[i] != w[i] {
						t.Errorf("memory[%.4x] = %.2x, want %.2x", i, g[i], w[i])
					}
				}
			}
			if g, w := c.m.PC, c.w.PC; g != w {
				t.Errorf("PC is %x, want %x", g, w)
			}
		})
	}
}

type execTestCase struct {
	m, w *Machine
	err  error
	set  *Machine
}

func newExecTestCase(op Op) *execTestCase {
	c := &execTestCase{}
	c.m = NewMachine([]byte{byte(op)})
	c.w = NewMachine([]byte{byte(op)})
	c.w.PC++
	c.set = c.m
	return c
}

func (c *execTestCase) work(bytes ...byte) *execTestCase {
	setStack(&c.set.Work, bytes)
	return c
}

func (c *execTestCase) ret(bytes ...byte) *execTestCase {
	setStack(&c.set.Ret, bytes)
	return c
}

func (c *execTestCase) mem(addr uint16, bytes ...byte) *execTestCase {
	copy(c.set.Mem[addr:], bytes)
	if c.set == c.m {
		copy(c.w.Mem[addr:], bytes)
	}
	return c
}

func (c *execTestCase) pc(addr uint16) *execTestCase {
	c.set.PC = addr
	return c
}

func (c *execTestCase) want() *execTestCase {
	c.set = c.w
	return c
}

func (c *execTestCase) error(err error) *execTestCase {
	c.err = err
	return c
}

func setStack(s *Stack, bytes []byte) {
	for i, b := range bytes {
		s.Bytes[i] = b
	}
	s.Ptr = byte(len(bytes))
}

func stackEq(a, b Stack) bool {
	ac := Stack{Ptr: a.Ptr}
	bc := Stack{Ptr: b.Ptr}
	copy(ac.Bytes[:], a.Bytes[:a.Ptr])
	copy(bc.Bytes[:], b.Bytes[:a.Ptr])
	return ac == bc
}

func rel(i int8) byte { return byte(i) }
