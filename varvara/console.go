package varvara

import (
	"log"
	"os"
)

type Console struct {
	Ready <-chan bool

	mem   deviceMem
	input <-chan byte
}

func (c *Console) Vector() uint16 { return c.mem.short(0x0) }

func (c *Console) In(p byte) byte {
	switch p {
	case 0x2:
		select {
		case b := <-c.input:
			c.mem[p] = b
			return b
		default:
		}
	}
	return c.mem[p]
}

func (c *Console) Out(p, b byte) {
	c.mem[p] = b
	switch p {
	case 0x01:
		if c.input == nil {
			var (
				input = make(chan byte, 1)
				ready = make(chan bool)
			)
			go readInput(input, ready)
			c.input, c.Ready = input, ready
		}
	case 0x08:
		os.Stdout.Write([]byte{b})
	case 0x09:
		os.Stderr.Write([]byte{b})
	}
}

func readInput(input chan<- byte, ready chan<- bool) {
	for {
		var b [1]byte
		if _, err := os.Stdin.Read(b[:]); err != nil {
			log.Printf("reading stdin: %v", err)
			return
		}
		input <- b[0]
		ready <- true
	}
}
