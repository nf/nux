package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/howeyc/fsnotify"
	"github.com/rivo/tview"

	"github.com/nf/nux/uxn"
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

	debug := newDebugView()
	runner := varvara.NewRunner(gui, true, debug.StateFunc)
	debug.r = runner
	log.SetPrefix("")
	log.SetOutput(debug.log)
	go func() {
		if err := debug.Run(); err != nil {
			log.Fatalf("debug: %v", err)
		}
		log.SetOutput(os.Stderr)
		log.SetPrefix("nux: ")
		runner.Debug("exit")
	}()

	romCh := make(chan []byte)
	go func() {
		started := false
		run := time.After(1 * time.Millisecond)
		for {
			select {
			case <-run:
				log.Printf("dev: build %s", filepath.Base(talFile))
				rom, err := devBuild(debug.log, talFile, romFile)
				if err != nil {
					log.Printf("dev: %v", err)
					break
				}
				syms, err := parseSymbols(romFile + ".sym")
				if err != nil {
					log.Printf("dev: reading symbols: %v", err)
					break
				}
				debug.setSymbols(syms)
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

type debugView struct {
	r     *varvara.Runner
	app   *tview.Application
	state *tview.TextView
	log   io.Writer

	mu   sync.Mutex
	syms symbols
}

func newDebugView() *debugView {
	var (
		logView = tview.NewTextView().
			SetMaxLines(1000)
		stateView  = tview.NewTextView()
		inputField = tview.NewInputField()
		flex       = tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(logView, 0, 1, false).
				AddItem(stateView, 3, 1, false).
				AddItem(inputField, 1, 1, true)
		app = tview.NewApplication().SetRoot(flex, true)
	)
	stateView.
		SetTextColor(tcell.ColorBlack).
		SetBackgroundColor(tcell.ColorGrey)
	logView.SetChangedFunc(func() { app.Draw() })

	d := &debugView{
		app:   app,
		state: stateView,
		log:   logView,
	}
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		cmd := inputField.GetText()
		inputField.SetText("")
		if cmd == "exit" {
			app.Stop()
		} else {
			d.r.Debug(cmd)
		}
	})
	return d
}

func (v *debugView) Run() error { return v.app.Run() }

func (v *debugView) StateFunc(m *uxn.Machine) {
	v.mu.Lock()
	syms := v.syms
	v.mu.Unlock()
	var (
		op    = uxn.Op(m.Mem[m.PC])
		pcSym string
		sym   string
	)
	if s := syms.forAddr(m.PC); len(s) > 0 {
		pcSym = s[0].String() + " -> "
	}
	if addr, ok := addrForOp(m); ok {
		switch s := syms.forAddr(addr); len(s) {
		case 0:
			// None.
		case 1:
			sym = s[0].String()
		case 2:
			switch op.Base() {
			case uxn.DEO, uxn.DEI:
				sym = s[0].String()
			default:
				sym = s[1].String()
			}
		default:
			for i, s := range s {
				if i != 0 {
					sym += " "
				}
				sym += s.String()
			}
		}
	}
	msg := fmt.Sprintf("%.4x %- 6s %s%s\nws: %v\nrs: %v\n", m.PC, op, pcSym, sym, m.Work, m.Ret)
	log.Print(msg)
	v.app.QueueUpdateDraw(func() {
		v.state.SetText(msg)
	})
}

func (v *debugView) setSymbols(s symbols) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.syms = s
}
