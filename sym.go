package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

type symbol struct {
	addr  uint16
	label string
}

func (s symbol) String() string { return fmt.Sprintf("%s (%.4x)", s.label, s.addr) }

type symbols struct {
	byAddr  []symbol
	byLabel []symbol
}

func (s *symbols) forAddr(addr uint16) []symbol {
	ss := s.byAddr
	i := sort.Search(len(ss), func(i int) bool {
		return ss[i].addr >= addr
	})
	j := i
	for ; j < len(ss) && ss[j].addr == addr; j++ {
	}
	return ss[i:j]
}

func (s *symbols) withLabel(label string) []symbol {
	ss := s.byLabel
	i := sort.Search(len(ss), func(i int) bool {
		return ss[i].label >= label
	})
	j := i
	for ; j < len(ss) && ss[j].label == label; j++ {
	}
	return ss[i:j]
}

func (s *symbols) withLabelPrefix(p string) []symbol {
	ss := s.byLabel
	i := sort.Search(len(ss), func(i int) bool {
		return ss[i].label >= p
	})
	j := i
	for ; j < len(ss) && strings.HasPrefix(ss[j].label, p); j++ {
	}
	return ss[i:j]
}

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

func (sym *symbols) resolve(t string) []symbol {
	if i, err := strconv.ParseUint(t, 16, 16); err == nil {
		return []symbol{{addr: uint16(i)}}
	}
	t, wild := strings.CutSuffix(t, "*")
	if wild {
		return sym.withLabelPrefix(t)
	}
	return sym.withLabel(t)
}
