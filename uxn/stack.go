package uxn

import (
	"fmt"
	"strings"
)

// Stack implements a Uxn CPU stack.
type Stack struct {
	Bytes [255]byte
	Ptr   byte
}

func (s *Stack) Peek() (byte, bool) {
	if s.Ptr == 0 {
		return 0, false
	}
	return s.Bytes[s.Ptr-1], true
}

func (s *Stack) PeekOffset() (uint16, bool) {
	b, ok := s.Peek()
	return uint16(int8(b)), ok
}

func (s *Stack) PeekShort() (uint16, bool) {
	if s.Ptr < 2 {
		return 0, false
	}
	return short(s.Bytes[s.Ptr-2], s.Bytes[s.Ptr-1]), true
}

func (s *Stack) wrap() *stackWrapper { return s.keep(false) }

func (s *Stack) keep(keep bool) *stackWrapper {
	return &stackWrapper{Stack: s, keep: keep}
}

type stackWrapper struct {
	*Stack
	keep   bool
	popped byte
	pushed bool
}

func (s *stackWrapper) Pop() byte {
	if s.pushed {
		panic("internal error: Pop after Push in StackWrapper")
	}
	if s.Ptr-s.popped == 0 {
		panic(Underflow)
	}
	if s.keep {
		s.popped++
	} else {
		s.Ptr--
	}
	return s.Bytes[s.Ptr-s.popped]
}

func (s *stackWrapper) Push(v byte) {
	if s.Ptr == 255 {
		panic(Overflow)
	}
	s.Bytes[s.Ptr] = v
	s.Ptr++
	s.pushed = true
}

func (s *stackWrapper) PopShort() uint16 {
	return uint16(s.Pop()) + uint16(s.Pop())<<8
}

func (s *stackWrapper) PushShort(v uint16) {
	s.Push(byte(v >> 8))
	s.Push(byte(v))
}

func (s *stackWrapper) PopOffset() uint16 {
	return uint16(int8(s.Pop()))
}

func (s *stackWrapper) PushBool(b bool) {
	if b {
		s.Push(1)
	} else {
		s.Push(0)
	}
}

type shortPushPopper struct {
	*stackWrapper
}

func (s shortPushPopper) Pop() uint16   { return s.PopShort() }
func (s shortPushPopper) Push(v uint16) { s.PushShort(v) }

type pushPopper[T byte | uint16] interface {
	Pop() T
	Push(T)
	PushBool(bool)
}

var (
	_ pushPopper[byte]   = &stackWrapper{}
	_ pushPopper[uint16] = shortPushPopper{}
)

func (s Stack) String() string {
	var b strings.Builder
	b.WriteByte('(')
	for _, v := range s.Bytes[:s.Ptr] {
		b.WriteByte(' ')
		fmt.Fprintf(&b, "%.2x", v)
	}
	b.WriteByte(' ')
	b.WriteByte(')')
	return b.String()
}
