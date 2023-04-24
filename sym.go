package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/nf/nux/uxn"
)

type symbols []symbol

func (s symbols) forAddr(addr uint16) (ss []symbol) {
	i := sort.Search(len(s), func(i int) bool { return s[i].addr >= addr })
	for ; i < len(s); i++ {
		if s[i].addr == addr {
			ss = append(ss, s[i])
		}
	}
	return ss
}

type symbol struct {
	addr  uint16
	label string
}

func (s symbol) String() string { return fmt.Sprintf("%s (%.4x)", s.label, s.addr) }

func parseSymbols(symFile string) (symbols, error) {
	b, err := os.ReadFile(symFile)
	if err != nil {
		return nil, err
	}
	var ss symbols
	for len(b) > 0 {
		if len(b) < 3 {
			return nil, fmt.Errorf("invalid symbol at end of file %q", b)
		}
		s := symbol{addr: uint16(b[0])<<8 + uint16(b[1])}
		b = b[2:]
		i := bytes.IndexByte(b, 0)
		if i < 0 {
			return nil, fmt.Errorf("invalid symbol label at %.4x %q", s.addr, b)
		}
		s.label = string(b[:i])
		b = b[i+1:]
		ss = append(ss, s)
	}
	sort.SliceStable(ss, func(i, j int) bool {
		return ss[i].addr < ss[j].addr
	})
	return ss, nil
}

func addrForOp(m *uxn.Machine) (uint16, bool) {
	switch op := uxn.Op(m.Mem[m.PC]); op.Base() {
	case uxn.JCI, uxn.JMI, uxn.JSI:
		return m.PC + uint16(m.Mem[m.PC+1])<<8 + uint16(m.Mem[m.PC+2]) + 3, true
	case uxn.JMP, uxn.JCN, uxn.JSR,
		uxn.LDR, uxn.STR,
		uxn.LDA, uxn.STA,
		uxn.LDZ, uxn.STZ,
		uxn.DEI, uxn.DEO:

		var st *uxn.StackWrapper
		if op.Return() {
			st = m.Ret.Mutate(true)
		} else {
			st = m.Work.Mutate(true)
		}
		switch op.Base() {
		case uxn.JMP, uxn.JCN, uxn.JSR:
			if op.Short() {
				// addr16 abs
				return st.PopShort(), true
			} else {
				// addr8 rel
				return m.PC + st.PopOffset() + 1, true
			}
		case uxn.LDR, uxn.STR: // addr8 rel
			return m.PC + st.PopOffset() + 1, true
		case uxn.LDA, uxn.STA: // addr16 rel
			return st.PopShort(), true
		case uxn.LDZ, uxn.STZ: // addr8 zero
			return uint16(st.Pop()), true
		case uxn.DEI, uxn.DEO: // device8
			return uint16(st.Pop()), true
		}
	}
	return 0, false
}

func (syms symbols) stateMsg(m *uxn.Machine) string {
	if m == nil {
		return ""
	}
	var (
		op    = uxn.Op(m.Mem[m.PC])
		pcSym string
		sym   string
	)
	if s := syms.forAddr(m.PC); len(s) > 0 {
		pcSym = s[0].String() + " -> "
	}
	if addr, ok := addrForOp(m); ok {
		switch s := syms.forAddr(addr); len(s) {
		case 0:
			// None.
		case 1:
			sym = s[0].String()
		case 2:
			switch op.Base() {
			case uxn.DEO, uxn.DEI:
				sym = s[0].String()
			default:
				sym = s[1].String()
			}
		default:
			for i, s := range s {
				if i != 0 {
					sym += " "
				}
				sym += s.String()
			}
		}
	}
	return fmt.Sprintf("%.4x %- 6s %s%s\nws: %v\nrs: %v\n",
		m.PC, op, pcSym, sym, m.Work, m.Ret)
}

func (syms symbols) resolve(s string) uint16 {
	if i, err := strconv.ParseUint(s, 16, 16); err == nil {
		return uint16(i)
	}
	for _, sym := range syms {
		if s == sym.label {
			return sym.addr
		}
	}
	return 0
}
