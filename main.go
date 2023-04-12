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
	(&uxn.Machine{Dev: &Varvara{}}).Run(rom, logf)
}
