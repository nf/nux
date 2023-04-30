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
	break <ref> ...
		Set break points at the given references (memory address or
		label). When uxn reaches the break point it pauses execution.
	rmbreak <ref> ...
		Unset the break points at the given references.
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
etc), with the exceptions of rmb/rmbreak, w2/watch2, and rmw/rmwatch.
`

type Debugger struct {
	Runner *varvara.Runner // Must be set before calling Run.
	Log    io.Writer

	log      *tview.TextView
	watch    *tview.TextView
	ops      *tview.TextView
	tick     *tview.TextView
	state    *tview.TextView
	stateLog *tview.TextView
	memory   *tview.TextView
	input    *tview.InputField
	right    *tview.Flex
	cols     *tview.Flex
	rows     *tview.Flex
	app      *tview.Application

	mu        sync.Mutex
	syms      *symbols
	breaks    []symbol
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

func NewDebugger() *Debugger {
	d := &Debugger{
		log: tview.NewTextView().
			SetMaxLines(1000).
			ScrollToEnd(),
		watch: tview.NewTextView().
			SetWrap(false).
			SetDynamicColors(true).
			SetTextAlign(tview.AlignRight),
		ops: tview.NewTextView().
			SetWrap(false).
			SetDynamicColors(true),
		tick: tview.NewTextView().
			SetWrap(false).
			SetTextAlign(tview.AlignRight),
		state: tview.NewTextView().
			SetWrap(false).
			SetDynamicColors(true),
		stateLog: tview.NewTextView().
			SetMaxLines(300).
			ScrollToEnd().
			SetDynamicColors(true),
		memory: tview.NewTextView().
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
		AddItem(d.tick, 2, 0, false)
	d.rows.
		AddItem(d.cols, 0, 1, false).
		AddItem(d.state, 3, 0, false).
		AddItem(d.input, 1, 0, true)
	d.app.SetRoot(d.rows, true)

	const (
		logVisible = iota
		stateLogVisible
		memoryVisible
	)
	setMainWindow := func(mode int) {
		switch mode {
		case logVisible:
			d.cols.Clear().
				AddItem(d.ops, 35, 0, false).
				AddItem(d.log, 0, 1, false).
				AddItem(d.right, 25, 0, false)
		case stateLogVisible:
			d.cols.Clear().
				AddItem(d.stateLog, 0, 1, false).
				AddItem(d.ops, 35, 0, false).
				AddItem(d.right, 25, 0, false)
		case memoryVisible:
			d.cols.Clear().
				AddItem(d.ops, 35, 0, false).
				AddItem(d.memory, 0, 1, false).
				AddItem(d.right, 25, 0, false)
		}
	}
	setMainWindow(logVisible)

	d.app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		switch e.Key() {
		case tcell.KeyF4:
			d.Runner.Debug("reset", 0)
		case tcell.KeyF5:
			d.Runner.Debug("cont", 0)
		case tcell.KeyF6:
			d.Runner.Debug("step", 0)
		case tcell.KeyF7:
			d.Runner.Debug("halt", 0)
		case tcell.KeyF8:
			setMainWindow(logVisible)
		case tcell.KeyF9:
			setMainWindow(stateLogVisible)
		case tcell.KeyF10:
			setMainWindow(memoryVisible)
		default:
			return e
		}
		return nil
	})
	d.input.SetAutocompleteFunc(func(t string) (entries []string) {
		if cmd, ref, ok := strings.Cut(t, " "); ok && ref != "" {
			others := ""
			switch cmd {
			case "d", "debug":
				// Only one arg permitted.
			case "b", "break", "rmb", "rmbreak",
				"w", "watch", "w2", "watch2", "rmw", "rmwatch":
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
		case "break", "rmbreak", "watch", "watch2", "rmwatch":
			args := strings.Fields(arg)
			for _, arg := range args {
				syms := d.symbols().resolve(arg)
				if len(syms) == 0 {
					log.Printf("unknown reference %q", arg)
					continue
				}
				for i := range syms {
					s := syms[i]
					switch cmd {
					case "break":
						d.Runner.Debug("break", s.addr)
						d.addBreak(s)
					case "rmbreak":
						d.Runner.Debug("rmbreak", s.addr)
						if d.rmBreak(s) {
							log.Printf("break removed: %s", s)
						}
						continue
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
		"rmb": "rmbreak", "rmbreak": "rmbreak",
		"w": "watch", "watch": "watch",
		"w2": "watch2", "watch2": "watch2",
		"rmw": "rmwatch", "rmwatch": "rmwatch",
	}[in]; ok {
		return out, true
	}
	return in, false
}

func (d *Debugger) addBreak(s symbol) {
	d.rmBreak(s)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.breaks = append(d.breaks, s)
}

func (d *Debugger) rmBreak(s symbol) (removed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, v := range d.breaks {
		if v == s {
			d.breaks = append(d.breaks[:i], d.breaks[i+1:]...)
			return true
		}
	}
	return false
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
	// Adjust break points if they have changed.
	for i := len(d.breaks) - 1; i >= 0; i-- {
		bs := &d.breaks[i]
		if bs.label == "" {
			// Can't rewrite unlabeled breaks.
			continue
		}
		if ss := s.withLabel(bs.label); len(ss) == 0 {
			d.Runner.Debug("rmbreak", bs.addr)
			d.breaks = append(d.breaks[:i], d.breaks[i+1:]...)
		} else if ss[0].addr != bs.addr {
			d.Runner.Debug("break", ss[0].addr)
			d.Runner.Debug("rmbreak", bs.addr)
			bs.addr = ss[0].addr
		}
	}
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

	breaks := append([]symbol(nil), d.breaks...)
	sort.Slice(breaks, func(i, j int) bool { return breaks[i].addr < breaks[j].addr })
	var (
		ops    = opsText(d.syms, breaks, m)
		memory = memoryText(d.syms, breaks, m, m.PC)
		watch  = watchText(m.PC, d.breaks, d.watches)
		state  string
	)
	if k != varvara.ClearState && k != varvara.QuietState {
		state = stateText(d.syms, m, k)
	}
	d.mu.Unlock()

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
		d.ops.SetText(ops).ScrollToBeginning()
		d.memory.SetText(memory).ScrollToBeginning()
		if k != varvara.QuietState {
			d.stateLog.Write([]byte(d.state.GetText(false)))
			d.state.SetText(state)
		}
	})
}

func stateText(syms *symbols, m *uxn.Machine, k varvara.StateKind) string {
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
	return fmt.Sprintf("%s %.4x %- 6s %s%s\nws: %v\nrs: %v",
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

const (
	beforeOps = 0x10
	totalOps  = 0x40
)

func opsText(syms *symbols, breaks []symbol, m *uxn.Machine) string {
	start := m.PC - beforeOps
	if m.PC < beforeOps {
		start = 0
	}
	end := start + totalOps

	opAddr, hasAddr := m.OpAddr(m.PC)
	opShort := opAddrShort(uxn.Op(m.Mem[m.PC]))

	var s strings.Builder
	for addr := start; addr < end; addr++ {
		b := m.Mem[addr]
		op := uxn.Op(b)
		label := ""
		beforeLabel := false
		ss := syms.forAddr(addr)
		if len(ss) == 0 && addr == start {
			beforeLabel = true
			ss = syms.beforeAddr(addr)
		}
		for _, s := range ss {
			if len(label) > 0 {
				label += " "
			}
			if !beforeLabel {
				if _, child, ok := strings.Cut(s.label, "/"); ok {
					label += "&" + child
					continue
				}
			}
			label += s.label
		}
		if len(label) > 16 {
			label = label[:16]
		}
		for len(breaks) > 0 && breaks[0].addr < addr {
			breaks = breaks[1:]
		}
		isBreak := len(breaks) > 0 && breaks[0].addr == addr
		lineColor := ""
		if addr == m.PC {
			lineColor = "-:darkblue"
		} else if hasAddr && (addr == opAddr || opShort && addr-1 == opAddr) {
			lineColor = "black:aqua"
		}
		if lineColor != "" {
			fmt.Fprintf(&s, "[%s] [%.4x] %.2x %- 6s %- 16s [-:-]\n",
				lineColor, addr, b, op, label)
		} else {
			hexColor := "grey"
			opColor := "-"
			if isMaybeLiteral(m, addr) {
				hexColor = "olive"
				opColor = "grey"
			}
			addrColor := "-"
			labelColor := "-"
			if isBreak {
				addrColor = "yellow"
				labelColor = "yellow"
			}
			if beforeLabel {
				labelColor = "grey"
			}
			fmt.Fprintf(&s, " [%s][%.4x][-] [%s]%.2x[-] [%s]%- 6s[-] [%s]%- 16s[-]\n",
				addrColor, addr, hexColor, b, opColor, op, labelColor, label)
		}
	}
	return s.String()
}

func isMaybeLiteral(m *uxn.Machine, addr uint16) bool {
	return uxn.Op(m.Mem[addr-1]) == uxn.LIT ||
		uxn.Op(m.Mem[addr-1]) == uxn.LIT2 ||
		uxn.Op(m.Mem[addr-2]) == uxn.LIT2
}

const (
	beforeMem = 0x080
	totalMem  = 0x200
)

func memoryText(syms *symbols, breaks []symbol, m *uxn.Machine, addr uint16) string {
	var (
		from = addr&0xfff0 - beforeMem
		to   = from + totalMem
		ss   = syms.byAddr
	)

	opAddr, hasAddr := m.OpAddr(addr)
	opShort := opAddrShort(uxn.Op(m.Mem[addr]))

	var (
		s strings.Builder
		b = make([]byte, 0x10*3+20)
	)
	for addr := from; addr < to; addr++ {
		if addr&0xf == 0 {
			if addr != from {
				fmt.Fprintf(&s, "\n        %s\n", b)
				for i := range b {
					b[i] = ' '
				}
			}
			fmt.Fprintf(&s, " [%.4x]", addr)
		}
		if addr&0xf == 0x8 {
			fmt.Fprintf(&s, " ")
		}
		for len(breaks) > 0 && breaks[0].addr < addr {
			breaks = breaks[1:]
		}
		isBreak := len(breaks) > 0 && breaks[0].addr == addr
		hexCol := "grey"
		if addr == m.PC {
			if isBreak {
				hexCol = "yellow:darkblue"
			} else {
				hexCol = ":darkblue"
			}
		} else if hasAddr && (addr == opAddr || opShort && addr-1 == opAddr) {
			hexCol = "black:aqua"
		} else if isBreak {
			hexCol = "black:yellow"
		} else if isMaybeLiteral(m, addr) {
			hexCol = "olive"
		}
		if hexCol == "" {
			fmt.Fprintf(&s, " %.2x", m.Mem[addr])
		} else {
			fmt.Fprintf(&s, " [%s]%.2x[-:-]", hexCol, m.Mem[addr])
		}
		for len(ss) > 0 && ss[0].addr < addr {
			ss = ss[1:]
		}
		if len(ss) > 0 && ss[0].addr == addr {
			i := addr & 0xf * 3
			if addr&0xf >= 0x8 {
				i++
			}
			label, child, ok := strings.Cut(ss[0].label, "/")
			if ok {
				label = "&" + child
			} else {
				label = "@" + label
			}
			copy(b[i:], label+strings.Repeat(" ", 20))
		}
	}
	return s.String()
}

func updateWatches(now time.Time, watches []watch, m *uxn.Machine) {
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
}

func watchText(pc uint16, breaks []symbol, watches []watch) string {
	var b strings.Builder
	for _, s := range breaks {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		if pc == s.addr {
			fmt.Fprint(&b, "[yellow]")
		}
		fmt.Fprintf(&b, "%s [%.4x] brk!", s.label, s.addr)
		if pc == s.addr {
			fmt.Fprint(&b, "[-]")
		}
	}
	for _, w := range watches {
		if b.Len() > 0 {
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

func opAddrShort(op uxn.Op) bool {
	if op.Short() {
		switch op.Base() {
		case uxn.LDR, uxn.STR, uxn.LDA, uxn.STA, uxn.LDZ, uxn.STZ:
			return true
		}
	}
	return false
}
