package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		runner.Debug("exit", 0)
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
	r *varvara.Runner

	log   *tview.TextView
	watch *tview.TextView
	state *tview.TextView
	input *tview.InputField
	cols  *tview.Flex
	rows  *tview.Flex
	app   *tview.Application

	dbg, brk *symbol

	mu      sync.Mutex
	syms    *symbols
	watches []watch
}

type watch struct {
	symbol
	short bool
}

func (d *debugView) symbols() *symbols {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.syms
}

func (d *debugView) setSymbols(s *symbols) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.syms = s
}

func newDebugView() *debugView {
	d := &debugView{
		log: tview.NewTextView().
			SetMaxLines(1000),
		watch: tview.NewTextView().
			SetWrap(false).
			SetTextAlign(tview.AlignRight),
		state: tview.NewTextView().
			SetWrap(false),
		input: tview.NewInputField(),
		cols:  tview.NewFlex(),
		rows: tview.NewFlex().
			SetDirection(tview.FlexRow),
		app: tview.NewApplication(),
	}
	d.log.SetChangedFunc(func() { d.app.Draw() })
	d.watch.SetBackgroundColor(tcell.ColorDarkBlue)
	d.state.SetBackgroundColor(tcell.ColorDarkGrey)
	d.cols.
		AddItem(d.watch, 0, 1, false).
		AddItem(d.log, 0, 2, false)
	d.rows.
		AddItem(d.cols, 0, 1, false).
		AddItem(d.state, 3, 0, false).
		AddItem(d.input, 1, 0, true)
	d.app.SetRoot(d.rows, true)

	d.input.SetAutocompleteFunc(func(t string) (entries []string) {
		if cmd, arg, ok := strings.Cut(t, " "); ok {
			switch cmd {
			case "b", "break", "d", "debug", "w", "w2", "watch", "watch2":
				for _, s := range d.symbols().withLabelPrefix(arg) {
					entries = append(entries, cmd+" "+s.label)
				}
			}
		}
		return
	})
	d.input.SetAutocompletedFunc(func(t string, index, src int) bool {
		if src != tview.AutocompletedNavigate {
			d.input.SetText(t)
		}
		return src == tview.AutocompletedEnter || src == tview.AutocompletedClick
	})
	d.input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		cmd := d.input.GetText()
		if cmd == "" {
			return
		}
		d.input.SetText("")
		if cmd == "exit" {
			d.app.Stop()
			return
		}
		if cmd, arg, ok := strings.Cut(cmd, " "); ok {
			switch cmd {
			case "b", "break", "d", "debug":
				s, ok := d.symbols().resolve(arg)
				if !ok {
					log.Printf("invalid addr %q", arg)
					return
				}
				d.r.Debug(cmd, s.addr)
				switch cmd[0] {
				case 'b':
					d.brk = &s
					log.Printf("set break %.4x", s.addr)
				case 'd':
					d.dbg = &s
					log.Printf("set debug %.4x", s.addr)
				}
				return
			case "w", "w2", "watch", "watch2":
				s, ok := d.symbols().resolve(arg)
				if !ok {
					log.Printf("invalid address %q", arg)
					return
				}
				d.mu.Lock()
				d.watches = append(d.watches,
					watch{symbol: s, short: strings.HasSuffix(cmd, "2")})
				d.mu.Unlock()
				log.Printf("watching %.4x", s.addr)
				return
			}
		}
		d.r.Debug(cmd, 0)
		switch cmd[0] {
		case 'b':
			d.brk = nil
			log.Print("cleared break")
		case 'd':
			d.dbg = nil
			log.Print("cleared debug")
		}
	})
	return d
}

func (d *debugView) Run() error { return d.app.Run() }

func (d *debugView) StateFunc(m *uxn.Machine, k varvara.StateKind) {
	var (
		watch = d.watchContent(m)
		state string
	)
	if k != varvara.ClearState && k != varvara.QuietState {
		state = stateMsg(d.symbols(), m, k)
	}
	d.app.QueueUpdateDraw(func() {
		switch k {
		case varvara.DebugState, varvara.ClearState:
			d.state.SetTextColor(tcell.ColorBlack)
			d.state.SetBackgroundColor(tcell.ColorDarkGrey)
		case varvara.BreakState:
			d.state.SetTextColor(tcell.ColorYellow)
			d.state.SetBackgroundColor(tcell.ColorDarkBlue)
		case varvara.PauseState:
			d.state.SetTextColor(tcell.ColorWhite)
			d.state.SetBackgroundColor(tcell.ColorDarkBlue)
		case varvara.HaltState:
			d.state.SetTextColor(tcell.ColorWhite)
			d.state.SetBackgroundColor(tcell.ColorDarkRed)
		}
		d.watch.SetText(watch)
		if k != varvara.QuietState {
			d.state.SetText(state)
		}
	})
}

func stateMsg(syms symbols, m *uxn.Machine, k varvara.StateKind) string {
	var (
		op    = uxn.Op(m.Mem[m.PC])
		pcSym string
		sym   string
	)
	if s := syms.forAddr(m.PC); len(s) > 0 {
		pcSym = s[0].String() + " -> "
	}
	if addr, ok := m.OpAddr(m.PC); ok {
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
	kind := "       "
	switch k {
	case varvara.BreakState:
		kind = "[break]"
	case varvara.DebugState:
		kind = "[debug]"
	case varvara.PauseState:
		kind = "[pause]"
	case varvara.HaltState:
		kind = "[HALT!]"
	}
	return fmt.Sprintf("%.4x %- 6s %s %s%s\nws: %v\nrs: %v\n",
		m.PC, op, kind, pcSym, sym, m.Work, m.Ret)
}

func (d *debugView) watchContent(m *uxn.Machine) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var b strings.Builder
	if s := d.brk; s != nil {
		fmt.Fprintf(&b, "%s [%.4x] brk!\n", s.label, s.addr)
	}
	if s := d.dbg; s != nil {
		fmt.Fprintf(&b, "%s [%.4x] dbg?\n", s.label, s.addr)
	}
	for _, w := range d.watches {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s [%.4x] ", w.label, w.addr)
		if w.short {
			fmt.Fprintf(&b, "%.2x%.2x", m.Mem[w.addr], m.Mem[w.addr+1])
		} else {
			fmt.Fprintf(&b, "  %.2x", m.Mem[w.addr])
		}
	}
	return b.String()
}
