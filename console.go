package main

import (
	"log"
	"os"
)

type Console struct {
	vector chan<- uint16
	input  <-chan byte
	next   <-chan uint16
}

func (c *Console) In(d byte) byte {
	switch d {
	case 0x12:
		select {
		case b := <-c.input:
			return b
		default:
			return 0
		}
	default:
		panic("not implemented")
	}
}

func (v *Console) InShort(byte) uint16 { panic("not implemented") }

func (c *Console) Out(d, b byte) {
	switch d {
	case 0x18:
		os.Stdout.Write([]byte{b})
	case 0x19:
		os.Stderr.Write([]byte{b})
	default:
		panic("not implemented")
	}
}

func (c *Console) OutShort(d byte, b uint16) {
	switch d {
	case 0x10:
		if c.vector == nil {
			var (
				vector = make(chan uint16)
				input  = make(chan byte, 1)
				next   = make(chan uint16)
			)
			go readInput(vector, input, next)
			c.vector, c.input, c.next = vector, input, next
		}
		c.vector <- b
	case 0x18:
		os.Stdout.Write([]byte{byte(b >> 8), byte(b)})
	case 0x19:
		os.Stderr.Write([]byte{byte(b >> 8), byte(b)})
	default:
		panic("not implemented")
	}
}

func (c *Console) Next() <-chan uint16 { return c.next }

func readInput(vector <-chan uint16, input chan<- byte, next chan<- uint16) {
	read := make(chan byte)
	go func() {
		for {
			var b [1]byte
			if _, err := os.Stdin.Read(b[:]); err != nil {
				log.Printf("reading stdin: %v", err)
				return
			}
			read <- b[0]
		}
	}()
	var (
		vec = <-vector
		val byte
	)
	for {
		select {
		case vec = <-vector:
			continue
		case val = <-read:
		}
	sendVal:
		select {
		case vec = <-vector:
			goto sendVal
		case input <- val:
		}
	sendVec:
		select {
		case vec = <-vector:
			goto sendVec
		case next <- vec:
		}
	}
}
