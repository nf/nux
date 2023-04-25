package main

import (
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

const helpText = `
nux debugger commands and keyboard shortcuts:

	reset (F4)
		Halt the uxn program, reset it to its initial state from ROM,
		and run it. This unpauses uxn if it was paused, but preserves
		any break or debug points that are set.
	cont  (F5)
		Resume uxn exeuction, if paused.
	step  (F6)
		Pause uxn (if not already paused) and execute one instruction.
	halt  (F7)
		Halt the uxn program.
	break [ref]
		Set the break point to the given reference (memory address or
		label), or unset the break point if no reference is given.
		When uxn reaches the break point it pauses execution.
	debug [ref]
		Set the debug point to the given reference, or unset the debug
		point if no reference is given. When uxn reaches the debug
		point it updates the debug status lines and any watches.
	watch[2] <ref> ...
		Add a watch for the given references. If the "watch2" variant
		is used then each reference is treated as a short, not a byte.
	rmwatch <ref> ...
		Remove any watches for the given references.
	exit  (^C)
		Exit nux.
	help
		Print these instructions. :)

Refrences may use a "*" suffix to select all labels that match a prefix.

Commands may be abbreviated using just their first character ("r" for "reset",
etc), with the exceptions of "w2" for "watch2" and "rmw" for "rmwatch".
`

type Debugger struct {
	Runner *varvara.Runner // Must be set before calling Run.
	Log    io.Writer

	log   *tview.TextView
	watch *tview.TextView
	tick  *tview.TextView
	state *tview.TextView
	input *tview.InputField
	right *tview.Flex
	cols  *tview.Flex
	rows  *tview.Flex
	app   *tview.Application

	dbg, brk *symbol

	mu        sync.Mutex
	syms      *symbols
	watches   []watch
	started   time.Time
	lastState time.Time
}

type watch struct {
	symbol
	short   bool
	last    uint16
	changed time.Time
}

func (d *Debugger) addWatch(s symbol, short bool) {
	d.rmWatch(s) // Prevent duplicate entries.
	d.mu.Lock()
	defer d.mu.Unlock()
	d.watches = append(d.watches, watch{symbol: s, short: short})
}

func (d *Debugger) rmWatch(s symbol) (removed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, w := range d.watches {
		if w.symbol == s {
			d.watches = append(d.watches[:i], d.watches[i+1:]...)
			return true
		}
	}
	return false
}

func (d *Debugger) symbols() *symbols {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.syms
}

func (d *Debugger) SetSymbols(s *symbols) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.syms = s

	// Rewrite watch addresses as they may have changed.
	for i := 0; i < len(d.watches); {
		w := &d.watches[i]
		if w.label == "" {
			// Preserve unlabeled addresses.
			i++
			continue
		}
		ss := s.withLabel(w.label)
		if len(ss) == 0 {
			// Remove labels that are now missing.
			d.watches = append(d.watches[:i], d.watches[i+1:]...)
			continue
		}
		w.addr = ss[0].addr
		i++
	}
	// Adjust break and debug point if they have changed.
	if bs := d.brk; bs != nil && bs.label != "" {
		if ss := s.withLabel(bs.label); len(ss) == 0 {
			d.brk = nil
			d.Runner.Debug("break", 0)
		} else if ss[0].addr != bs.addr {
			bs.addr = ss[0].addr
			d.Runner.Debug("break", bs.addr)
		}
	}
	if ds := d.dbg; ds != nil && ds.label != "" {
		if ss := s.withLabel(ds.label); len(ss) == 0 {
			d.dbg = nil
			d.Runner.Debug("debug", 0)
		} else if ss[0].addr != ds.addr {
			ds.addr = ss[0].addr
			d.Runner.Debug("debug", ds.addr)
		}
	}
}

func NewDebugger() *Debugger {
	d := &Debugger{
		log: tview.NewTextView().
			SetMaxLines(1000),
		watch: tview.NewTextView().
			SetWrap(false).
			SetTextAlign(tview.AlignRight),
		tick: tview.NewTextView().
			SetWrap(false).
			SetTextAlign(tview.AlignRight),
		state: tview.NewTextView().
			SetWrap(false).
			SetDynamicColors(true),
		input: tview.NewInputField(),
		right: tview.NewFlex().
			SetDirection(tview.FlexRow),
		cols: tview.NewFlex(),
		rows: tview.NewFlex().
			SetDirection(tview.FlexRow),
		app: tview.NewApplication(),
	}
	d.Log = d.log
	d.log.SetChangedFunc(func() { d.app.Draw() })
	d.watch.SetBackgroundColor(tcell.ColorDarkBlue)
	d.tick.SetBackgroundColor(tcell.ColorDarkBlue)
	d.state.SetBackgroundColor(tcell.ColorDarkGrey)
	d.right.
		AddItem(d.watch, 0, 1, false).
		AddItem(d.tick, 4, 0, false)
	d.cols.
		AddItem(d.log, 0, 2, false).
		AddItem(d.right, 0, 1, false)
	d.rows.
		AddItem(d.cols, 0, 1, false).
		AddItem(d.state, 3, 0, false).
		AddItem(d.input, 1, 0, true)
	d.app.
		SetRoot(d.rows, true).
		SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			switch e.Key() {
			case tcell.KeyF4:
				d.Runner.Debug("reset", 0)
			case tcell.KeyF5:
				d.Runner.Debug("cont", 0)
			case tcell.KeyF6:
				d.Runner.Debug("step", 0)
			case tcell.KeyF7:
				d.Runner.Debug("halt", 0)
			default:
				return e
			}
			return nil
		})

	d.input.SetAutocompleteFunc(func(t string) (entries []string) {
		if cmd, ref, ok := strings.Cut(t, " "); ok && ref != "" {
			others := ""
			switch cmd {
			case "b", "break", "d", "debug":
				// Only one arg permitted.
			case "w", "watch", "w2", "watch2", "rmw", "rmwatch":
				// Support completing later args.
				if i := strings.LastIndexByte(ref, ' '); i >= 0 {
					others = ref[:i+1]
					ref = ref[i+1:]
				}
			default:
				return
			}
			for _, s := range d.symbols().withLabelPrefix(ref) {
				entries = append(entries, cmd+" "+others+s.label)
			}
			return
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
		cmd := strings.TrimSpace(d.input.GetText())
		if cmd == "" {
			return
		}
		d.input.SetText("")
		cmd, arg, _ := strings.Cut(cmd, " ")
		cmd, ok := parseCommand(cmd)
		if !ok {
			log.Printf("bad command %q", cmd)
			return
		}
		switch cmd {
		case "exit":
			d.app.Stop()
			return
		case "help":
			log.Print(helpText)
			return
		case "break", "debug", "watch", "watch2", "rmwatch":
			if arg == "" {
				switch cmd {
				case "break":
					d.Runner.Debug("break", 0)
					d.brk = nil
					log.Print("cleared break")
				case "debug":
					d.Runner.Debug("debug", 0)
					d.dbg = nil
					log.Print("cleared debug")
				default:
					log.Printf("%s requires reference argument(s)", cmd)
				}
				return
			}
			args := strings.Fields(arg)
			if len(args) > 1 && (cmd == "break" || cmd == "debug") {
				log.Printf("%s only accepts one argument", cmd)
				return
			}
			for _, arg := range args {
				syms := d.symbols().resolve(arg)
				switch len(syms) {
				case 0:
					log.Printf("unknown reference %q", arg)
					continue
				case 1:
					// OK
				default:
					if cmd == "break" || cmd == "debug" {
						log.Printf("wildcards not supported for %s", cmd)
						return
					}
				}
				for i := range syms {
					s := syms[i]
					switch cmd {
					case "break":
						d.Runner.Debug("break", s.addr)
						d.brk = &s
					case "debug":
						d.Runner.Debug("debug", s.addr)
						d.dbg = &s
					case "watch", "watch2":
						d.addWatch(s, strings.HasSuffix(cmd, "2"))
					case "rmwatch":
						if d.rmWatch(s) {
							log.Printf("watch removed: %s", s)
						}
						continue
					}
					log.Printf("%s set: %s", cmd, s)
				}
			}
		default:
			d.Runner.Debug(cmd, 0)
		}
	})
	return d
}

func parseCommand(in string) (string, bool) {
	if out, ok := map[string]string{
		"help": "help", "exit": "exit",
		"h": "halt", "halt": "halt",
		"r": "reset", "reset": "reset",
		"s": "step", "step": "step",
		"c": "cont", "cont": "cont",
		"b": "break", "break": "break",
		"d": "debug", "debug": "debug",
		"w": "watch", "watch": "watch",
		"w2": "watch2", "watch2": "watch2",
		"rmw": "rmwatch", "rmwatch": "rmwatch",
	}[in]; ok {
		return out, true
	}
	return in, false
}

func (d *Debugger) Run() error {
	t := time.NewTicker(30 * time.Millisecond)
	defer t.Stop()
	go func() {
		for {
			<-t.C
			d.app.QueueUpdateDraw(func() {
				d.tick.SetText(d.tickContent())
			})
		}
	}()
	return d.app.Run()
}

func (d *Debugger) StateFunc(m *uxn.Machine, k varvara.StateKind) {
	d.mu.Lock()
	now := time.Now()
	d.lastState = time.Now()
	if d.started.IsZero() {
		d.started = now
	}
	if k == varvara.HaltState {
		d.started = time.Time{}
	}
	updateWatches(now, d.watches, m)
	watch := watchContent(d.watches)
	d.mu.Unlock()

	var state string
	if k != varvara.ClearState && k != varvara.QuietState {
		state = stateContent(d.symbols(), m, k)
	}
	d.app.QueueUpdate(func() {
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
		d.watch.SetText(watch).ScrollToEnd()
		if k != varvara.QuietState {
			d.state.SetText(state)
		}
	})
}

func stateContent(syms *symbols, m *uxn.Machine, k varvara.StateKind) string {
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
		default:
			switch op.Base() {
			case uxn.DEO, uxn.DEI:
				sym = s[0].String()
			default:
				sym = s[len(s)-1].String()
			}
		}
		if sym != "" {
			switch op.Base() {
			case uxn.JCI, uxn.JMI, uxn.JSI:
				// Address doesn't come from stack.
			default:
				sym = stackColor1 + sym + "[-:-]"
			}
		}
	}
	kind := "     "
	switch k {
	case varvara.BreakState:
		kind = "BREAK"
	case varvara.DebugState:
		kind = "DEBUG"
	case varvara.PauseState:
		kind = "PAUSE"
	case varvara.HaltState:
		kind = "HALT!"
	}
	var workOp, retOp uxn.Op
	if op.Base() == uxn.STH {
		workOp, retOp = op, op
	} else if op.Return() {
		retOp = op
	} else {
		workOp = op
	}
	return fmt.Sprintf("%s %.4x %- 6s %s%s\nws: %v\nrs: %v\n",
		kind, m.PC, op, pcSym, sym,
		formatStack(&m.Work, workOp),
		formatStack(&m.Ret, retOp))
}

const (
	stackColor1 = "[black:aqua]"
	stackColor2 = "[black:fuchsia]"
	stackColor3 = "[black:lime]"
)

func formatStack(st *uxn.Stack, op uxn.Op) string {
	v1, v2, v3 := op.StackArgs()

	var b strings.Builder
	b.WriteByte('(')
	for i, v := range st.Bytes[:st.Ptr] {
		b.WriteByte(' ')
		if op > 0 {
			idx, pre, post := int(st.Ptr)-i, "", ""
			formatStackVal(idx, &pre, &post, v3, stackColor3)
			formatStackVal(idx, &pre, &post, v2, stackColor2)
			formatStackVal(idx, &pre, &post, v1, stackColor1)
			b.WriteString(pre)
			fmt.Fprintf(&b, "%.2x", v)
			b.WriteString(post)
		} else {
			fmt.Fprintf(&b, "%.2x", v)
		}
	}
	b.WriteByte(' ')
	b.WriteByte(')')
	return b.String()
}

func formatStackVal(i int, pre, post *string, v uxn.StackVal, color string) {
	if v.Index > 0 && (v.Index == i || v.Index-(v.Size-1) == i) {
		*pre, *post = color, "[-:-]"
	}
}

func updateWatches(now time.Time, watches []watch, m *uxn.Machine) {
	now = now.Truncate(10 * time.Second) // Prevent jumpiness.
	for i := range watches {
		w := &watches[i]
		var v uint16
		if w.short {
			v = uint16(m.Mem[w.addr])<<8 + uint16(m.Mem[w.addr+1])
		} else {
			v = uint16(m.Mem[w.addr])
		}

		if w.last != v {
			w.changed = now
			w.last = v
		}
	}
	sort.SliceStable(watches, func(i, j int) bool {
		return watches[i].changed.Before(watches[j].changed)
	})
}

func watchContent(watches []watch) string {
	var b strings.Builder
	for i, w := range watches {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s [%.4x] ", w.label, w.addr)
		if w.short {
			fmt.Fprintf(&b, "%.4x", w.last)
		} else {
			fmt.Fprintf(&b, "  %.2x", w.last)
		}
	}
	return b.String()
}

func (d *Debugger) tickContent() string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var b strings.Builder
	now := time.Now()
	if s := d.brk; s != nil {
		fmt.Fprintf(&b, "%s [%.4x] brk\n", s.label, s.addr)
	} else {
		b.WriteByte('\n')
	}
	if s := d.dbg; s != nil {
		fmt.Fprintf(&b, "%s [%.4x] dbg\n", s.label, s.addr)
	} else {
		b.WriteByte('\n')
	}
	if age := now.Sub(d.lastState).Truncate(time.Second); age > 0 {
		fmt.Fprintf(&b, "%s since state\n", age)
	} else {
		b.WriteByte('\n')
	}
	if !d.started.IsZero() {
		fmt.Fprintf(&b, "%s since start\n", now.Sub(d.started).Truncate(time.Second))
	} else {
		b.WriteByte('\n')
	}
	return b.String()
}
