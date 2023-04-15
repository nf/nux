// Command nux executes Uxn ROMs on a Varvara machine.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

func main() {
	log.SetPrefix("nux: ")
	log.SetFlags(0)

	var (
		debugFlag = flag.Bool("debug", false, "print debugging information")
		guiFlag   = flag.Bool("gui", false, "enable GUI features")
	)

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
		log.Fatal(err)
	}

	logf := uxn.Nopf
	if *debugFlag {
		logf = log.Printf
	}
	os.Exit(varvara.Run(rom, *guiFlag, logf))
}
