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

func devMode(gui bool, talFile string) error {
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
		r   = varvara.NewRunner(gui, true)
		vCh = make(chan *varvara.Varvara)
	)
	go func() {
		started := false
		run := time.After(1 * time.Millisecond)
		for {
			select {
			case <-run:
				log.Printf("dev: build %s", filepath.Base(talFile))
				if rom, err := devBuild(os.Stderr, talFile, romFile); err != nil {
					log.Printf("dev: %v", err)
					break
				} else if v := varvara.New(rom); !started {
					log.Printf("dev: start")
					vCh <- v
					started = true
				} else {
					log.Printf("dev: reset")
					r.Reset(v)
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
	code := r.Run((<-vCh))
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
