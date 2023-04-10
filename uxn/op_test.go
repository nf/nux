package uxn

import "testing"

func TestOpBase(t *testing.T) {
	except := map[Op]Op{
		JCI:   JCI,
		JMI:   JMI,
		JSI:   JSI,
		LIT:   LIT,
		LIT2:  LIT,
		LITr:  LIT,
		LIT2r: LIT,
	}
	for _, o := range allOps() {
		got := o.Base()
		want := o & 0x1f
		if w, ok := except[o]; ok {
			want = w
		}
		if got != want {
			t.Errorf("Base(%v) returned %v, want %v", o, got, want)
		}
	}
}

// Check that there are string versions for every opcode,
// and that the flags correspond to the strings.
func TestOpString(t *testing.T) {
	for _, o := range allOps() {
		got := o.String()
		want := o.Base().String()
		if o.Short() {
			want += "2"
		}
		if o.Keep() {
			want += "k"
		}
		if o.Return() {
			want += "r"
		}
		if got != want {
			t.Errorf("Op(%x).String() returned %q, want %q", byte(o), got, want)
		}
	}
}

func allOps() []Op {
	ops := make([]Op, 0x100)
	for i := range ops {
		ops[i] = Op(i)
	}
	return ops
}
