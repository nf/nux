package varvara

import (
	"io"
)

const (
	consoleTypeStdin       byte = 0x1
	consoleTypeArgument    byte = 0x2
	consoleTypeEndArgument byte = 0x3
	consoleTypeEnd         byte = 0x4
)

type Console struct {
	Ready <-chan consoleInput

	mem  deviceMem
	args []string

	in       io.Reader
	out, err io.Writer
}

type consoleInput struct {
	b, kind byte
}

func (c *Console) Vector() uint16 { return c.mem.short(0x0) }

func (c *Console) In(p byte) byte {
	return c.mem[p]
}

func (c *Console) setInput(input consoleInput) {
	c.mem[0x2] = input.b
	c.mem[0x7] = input.kind
}

func (c *Console) Out(p, b byte) {
	c.mem[p] = b
	switch p {
	case 0x01:
		if c.Ready == nil {
			ready := make(chan consoleInput)
			go c.readInput(ready)
			c.Ready = ready
		}
	case 0x08:
		c.out.Write([]byte{b})
	case 0x09:
		c.err.Write([]byte{b})
	}
}

func (c *Console) readInput(ready chan<- consoleInput) {
	for i, arg := range c.args {
		for j := range len(arg) {
			ready <- consoleInput{arg[j], consoleTypeArgument}
		}
		kind := consoleTypeEndArgument
		if i == len(c.args)-1 {
			kind = consoleTypeEnd
		}
		ready <- consoleInput{'\n', kind}
	}
	for {
		var b [1]byte
		if _, err := c.in.Read(b[:]); err != nil {
			ready <- consoleInput{'\n', consoleTypeEnd}
			return
		}
		ready <- consoleInput{b[0], consoleTypeStdin}
	}
}
