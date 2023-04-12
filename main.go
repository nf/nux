// Command nux executes Uxn bytecode.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nf/nux/uxn"
)

func main() {
	log.SetPrefix("nux: ")
	log.SetFlags(0)

	debugFlag := flag.Bool("debug", false, "print debugging information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s <program.rom>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}

	rom, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		panic(err)
	}

	logf := uxn.Nopf
	if *debugFlag {
		logf = log.Printf
	}
	(&uxn.Machine{Dev: Device{}}).Run(rom, logf)
}

type Device struct{}

func (Device) In(byte) byte        { panic("device input not implemented") }
func (Device) InShort(byte) uint16 { panic("device input not implemented") }

func (Device) Out(device, v byte) {
	switch device {
	case 0x0f:
		if v == 0x01 {
			os.Exit(0)
		}
		panic(fmt.Errorf("device 0x0f given unknown value %x", v))
	case 0x18:
		os.Stdout.Write([]byte{v})
	}
}

func (Device) OutShort(device byte, v uint16) {
	switch device {
	case 0x0f:
		if v == 0x01 {
			os.Exit(0)
		}
		panic(fmt.Errorf("device 0x0f given unknown value %x", v))
	case 0x18:
		os.Stdout.Write([]byte{byte(v >> 8), byte(v)})
	}
}
