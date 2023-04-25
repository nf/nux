package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/howeyc/fsnotify"

	"github.com/nf/nux/varvara"
)

func devMode(enableGUI, enableDebug bool, talFile string) error {
	talFile = filepath.Clean(talFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Watch(filepath.Dir(talFile)); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "nux-dev-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	romFile := filepath.Join(tmp, filepath.Base(talFile)+".rom")

	var (
		runner *varvara.Runner
		debug  *Debugger
	)
	if enableDebug {
		debug = NewDebugger()
		runner = varvara.NewRunner(enableGUI, true, debug.StateFunc)
		runner.SetOutput(debug.Log)
		debug.Runner = runner

		log.SetPrefix("")
		log.SetOutput(debug.log)
		go func() {
			if err := debug.Run(); err != nil {
				log.Fatalf("debug: %v", err)
			}
			log.SetOutput(os.Stderr)
			log.SetPrefix("nux: ")
			runner.Debug("exit", 0)
		}()
	} else {
		runner = varvara.NewRunner(enableGUI, true, nil)
	}

	romCh := make(chan []byte)
	go func() {
		started := false
		run := time.After(1 * time.Millisecond)
		for {
			select {
			case <-run:
				log.Printf("dev: build %s", filepath.Base(talFile))
				var out io.Writer = os.Stderr
				if debug != nil {
					out = debug.Log
				}
				rom, err := devBuild(out, talFile, romFile)
				if err != nil {
					log.Printf("dev: %v", err)
					break
				}
				if debug != nil {
					syms, err := parseSymbols(romFile + ".sym")
					if err != nil {
						log.Printf("dev: reading symbols: %v", err)
						break
					}
					debug.SetSymbols(syms)
				}
				if !started {
					log.Printf("dev: start")
					romCh <- rom
					started = true
				} else {
					log.Printf("dev: reset")
					runner.Swap(rom)
				}
			case ev := <-watcher.Event:
				if ev.Name == talFile && !ev.IsAttrib() {
					run = time.After(100 * time.Millisecond)
				}
			case err := <-watcher.Error:
				log.Printf("dev: watcher: %v", err)
			}
		}
	}()
	code := runner.Run((<-romCh))
	return fmt.Errorf("dev: exit code: %d", code)
}

func devBuild(out io.Writer, talFile, romFile string) ([]byte, error) {
	cmd := exec.Command("uxnasm", talFile, romFile)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("uxnasm: %v", err)
	}
	return os.ReadFile(romFile)
}
