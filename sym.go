package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

type symbols struct {
	byAddr  []symbol
	byLabel []symbol
}

func (s *symbols) forAddr(addr uint16) (ss []symbol) {
	i := sort.Search(len(s.byAddr), func(i int) bool {
		return s.byAddr[i].addr >= addr
	})
	for ; i < len(s.byAddr); i++ {
		if s.byAddr[i].addr == addr {
			ss = append(ss, s.byAddr[i])
		}
	}
	return ss
}

func (s *symbols) withLabelPrefix(p string) (ss []symbol) {
	i := sort.Search(len(s.byLabel), func(i int) bool {
		return s.byLabel[i].label >= p
	})
	for ; i < len(s.byLabel); i++ {
		if strings.HasPrefix(s.byLabel[i].label, p) {
			ss = append(ss, s.byLabel[i])
		}
	}
	return ss
}

type symbol struct {
	addr  uint16
	label string
}

func (s symbol) String() string { return fmt.Sprintf("%s (%.4x)", s.label, s.addr) }

func parseSymbols(symFile string) (*symbols, error) {
	b, err := os.ReadFile(symFile)
	if err != nil {
		return nil, err
	}
	var ss []symbol
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
	var syms symbols
	sort.SliceStable(ss, func(i, j int) bool {
		return ss[i].addr < ss[j].addr
	})
	syms.byAddr = append([]symbol(nil), ss...)
	sort.SliceStable(ss, func(i, j int) bool {
		return ss[i].label < ss[j].label
	})
	syms.byLabel = append([]symbol(nil), ss...)
	return &syms, nil
}

func (syms symbols) stateMsg(m *uxn.Machine, k varvara.StateKind) string {
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
	if addr, ok := m.OpAddr(m.PC); ok {
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
	kind := "       "
	switch k {
	case varvara.BreakState:
		kind = "[break]"
	case varvara.DebugState:
		kind = "[debug]"
	case varvara.PauseState:
		kind = "[pause]"
	case varvara.HaltState:
		kind = "[HALT!]"
	}
	return fmt.Sprintf("%.4x %- 6s %s %s%s\nws: %v\nrs: %v\n",
		m.PC, op, kind, pcSym, sym, m.Work, m.Ret)
}

func (sym *symbols) resolve(t string) (symbol, bool) {
	if i, err := strconv.ParseUint(t, 16, 16); err == nil {
		return symbol{addr: uint16(i)}, true
	}
	for _, s := range sym.withLabelPrefix(t) {
		if s.label == t {
			return s, true
		}
	}
	return symbol{}, false
}
