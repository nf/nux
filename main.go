// Command nux executes Uxn ROMs on a Varvara machine.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

func main() {
	log.SetPrefix("nux: ")
	log.SetFlags(0)

	var (
		cpuProfileFlag = flag.String("cpu_profile", "", "write CPU profile to `file`")
		guiFlag        = flag.Bool("gui", false, "enable GUI features")
		devFlag        = flag.Bool("dev", false, "enable developer mode (live re-build and run an untxal program)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s <program.rom>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s -dev <program.tal>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}

	if *devFlag {
		if err := devMode(*guiFlag, flag.Arg(0)); err != nil {
			log.Fatal(err)
		}
		return
	}

	rom, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	var cpuProfile io.Closer
	if prof := *cpuProfileFlag; prof != "" {
		f, err := os.Create(prof)
		if err != nil {
			log.Fatalf("creating CPU profile file: %v", err)
		}
		pprof.StartCPUProfile(f)
		cpuProfile = f
	}

	r := varvara.NewRunner(*guiFlag, false, func(*uxn.Machine, bool) {})
	code := r.Run(rom)

	if f := cpuProfile; f != nil {
		pprof.StopCPUProfile()
		f.Close()
	}

	os.Exit(code)
}
