// Command nux executes Uxn ROMs on a Varvara machine.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/nf/nux/varvara"
)

func main() {
	log.SetPrefix("nux: ")
	log.SetFlags(0)

	var (
		cliFlag   = flag.Bool("cli", false, "disable GUI features")
		devFlag   = flag.Bool("dev", false, "enable developer mode (live re-build and run an untxal program)")
		debugFlag = flag.Bool("debug", false, "enable debugger (implies -dev)")

		cpuProfileFlag = flag.String("cpu_profile", "", "write CPU profile to `file`")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-cli] <program.rom | program.tal>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "       %s [-cli] <-dev | -debug> <program.tal>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}

	if *devFlag || *debugFlag {
		if err := devMode(!*cliFlag, *debugFlag, flag.Arg(0)); err != nil {
			log.Fatal(err)
		}
		return
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

	code, err := run(flag.Arg(0), !*cliFlag)

	if f := cpuProfile; f != nil {
		pprof.StopCPUProfile()
		f.Close()
	}

	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

func run(romFile string, guiEnabled bool) (int, error) {
	var (
		rom []byte
		err error
	)
	if filepath.Ext(romFile) == ".tal" {
		tmp, err := os.MkdirTemp("", "nux-build-*")
		if err != nil {
			return 0, err
		}
		defer os.RemoveAll(tmp)

		talFile := romFile
		romFile = filepath.Join(tmp, filepath.Base(talFile)+".rom")
		rom, err = devBuild(os.Stderr, talFile, romFile)
	} else {
		rom, err = os.ReadFile(romFile)
	}
	if err != nil {
		return 0, err
	}

	r := varvara.NewRunner(guiEnabled, false, nil)
	code := r.Run(rom)

	return code, nil
}
