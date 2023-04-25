package main

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/nf/nux/uxn"
	"github.com/nf/nux/varvara"
)

type Debugger struct {
	Runner *varvara.Runner // Must be set before calling Run.
	Log    io.Writer

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

func (d *Debugger) symbols() *symbols {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.syms
}

func (d *Debugger) SetSymbols(s *symbols) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.syms = s
}

func NewDebugger() *Debugger {
	d := &Debugger{
		log: tview.NewTextView().
			SetMaxLines(1000),
		watch: tview.NewTextView().
			SetWrap(false).
			SetTextAlign(tview.AlignRight),
		state: tview.NewTextView().
			SetWrap(false).
			SetDynamicColors(true),
		input: tview.NewInputField(),
		cols:  tview.NewFlex(),
		rows: tview.NewFlex().
			SetDirection(tview.FlexRow),
		app: tview.NewApplication(),
	}
	d.Log = d.log
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
	d.app.
		SetRoot(d.rows, true).
		SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
			switch e.Key() {
			case tcell.KeyF4:
				d.Runner.Debug("reset", 0)
			case tcell.KeyF5:
				d.Runner.Debug("cont", 0)
			case tcell.KeyF6:
				d.Runner.Debug("pause", 0)
			case tcell.KeyF7:
				d.Runner.Debug("step", 0)
			default:
				return e
			}
			return nil
		})

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
				d.Runner.Debug(cmd, s.addr)
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
		d.Runner.Debug(cmd, 0)
		switch cmd {
		case "break", "b":
			d.brk = nil
			log.Print("cleared break")
		case "debug", "d":
			d.dbg = nil
			log.Print("cleared debug")
		}
	})
	return d
}

func (d *Debugger) Run() error { return d.app.Run() }

func (d *Debugger) StateFunc(m *uxn.Machine, k varvara.StateKind) {
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

func stateMsg(syms *symbols, m *uxn.Machine, k varvara.StateKind) string {
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

func (d *Debugger) watchContent(m *uxn.Machine) string {
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
