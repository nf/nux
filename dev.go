package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/howeyc/fsnotify"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

func devMode(gui bool, talFile string) error {
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

	vCh := make(chan *varvara.Varvara)
	go func() {
		var (
			v   *varvara.Varvara
			run = time.After(1 * time.Millisecond)
		)
		for {
			select {
			case <-run:
				log.Printf("dev: build %s", filepath.Base(talFile))
				if rom, err := devBuild(talFile, romFile); err != nil {
					log.Printf("dev: %v", err)
				} else if v == nil {
					log.Printf("dev: start")
					v = varvara.New(rom)
					vCh <- v
				} else {
					log.Printf("dev: reset")
					v = v.Reset(rom)
				}
			case ev := <-watcher.Event:
				if ev.Name == filepath.Base(talFile) && !ev.IsAttrib() {
					run = time.After(100 * time.Millisecond)
				}
			case err := <-watcher.Error:
				log.Printf("watcher: %v", err)
			}
		}
	}()
	code := (<-vCh).Run(gui, true, uxn.Nopf)
	return fmt.Errorf("dev: exit code: %d", code)
}

func devBuild(talFile, romFile string) ([]byte, error) {
	cmd := exec.Command("uxnasm", talFile, romFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("uxnasm: %v", err)
	}
	return os.ReadFile(romFile)
}
