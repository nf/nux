package main

import (
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/howeyc/fsnotify"

	"github.com/nf/nux/varvara"
)

//go:embed drifblim.rom
var drifblimROM []byte

func devMode(enableGUI, enableDebug bool, talFile string) error {
	talFile, err := filepath.Abs(talFile)
	if err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	watchedFiles := make(map[string]bool)
	watchedDirs := make(map[string]bool)
	watchFiles := func(files []string, replace bool) {
		if replace {
			watchedFiles = make(map[string]bool)
		}
		for _, name := range files {
			name, err := filepath.Abs(filepath.FromSlash(name))
			if err != nil {
				log.Printf("dev: watch %s: %v", name, err)
				continue
			}
			watchedFiles[name] = true
			dir := filepath.Dir(name)
			if watchedDirs[dir] {
				continue
			}
			if err := watcher.Watch(dir); err != nil {
				log.Printf("dev: watch %s: %v", dir, err)
				continue
			}
			watchedDirs[dir] = true
		}
	}
	watchedFiles[talFile] = true
	talDir := filepath.Dir(talFile)
	if err := watcher.Watch(talDir); err != nil {
		return err
	}
	watchedDirs[talDir] = true
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
				rom, files, err := devBuildFiles(out, talFile, romFile)
				watchFiles(files, err == nil)
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
				name, err := filepath.Abs(ev.Name)
				if err == nil && watchedFiles[name] && !ev.IsAttrib() {
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
	rom, _, err := devBuildFiles(out, talFile, romFile)
	return rom, err
}

func devBuildFiles(out io.Writer, talFile, romFile string) ([]byte, []string, error) {
	tmp, err := os.MkdirTemp(".", ".nux-build-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmp)

	buildROM := filepath.Join(tmp, filepath.Base(romFile))
	r := varvara.NewRunner(false, false, nil)
	r.SetOutput(out)
	r.SetArgs(filepath.ToSlash(talFile), filepath.ToSlash(buildROM))
	var files []string
	r.SetFileReadFunc(func(name string) { files = append(files, name) })
	if code := r.Run(drifblimROM); code != 0 {
		return nil, files, fmt.Errorf("drifblim: exit code: %d", code)
	}

	rom, err := os.ReadFile(buildROM)
	if err != nil {
		return nil, files, err
	}
	if err := os.WriteFile(romFile, rom, 0644); err != nil {
		return nil, files, err
	}
	sym, err := os.ReadFile(buildROM + ".sym")
	if err != nil {
		return nil, files, err
	}
	if err := os.WriteFile(romFile+".sym", sym, 0644); err != nil {
		return nil, files, err
	}
	return rom, files, nil
}
