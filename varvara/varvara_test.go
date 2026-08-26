package varvara

import (
	"bytes"
	"io"
	"testing"

	"github.com/nf/nux/uxn"
)

func TestUnimplementedDeviceMemoryWrap(t *testing.T) {
	v := New(nil, nil, io.Discard, io.Discard)
	v.OutShort(0xff, 0x1234)
	if got, want := v.InShort(0xff), uint16(0x1234); got != want {
		t.Fatalf("InShort(ff) = %.4x, want %.4x", got, want)
	}
}

func TestSystemStateExitsAtBRK(t *testing.T) {
	rom := []byte{
		byte(uxn.LIT), 0x80, byte(uxn.LIT), 0x0f, byte(uxn.DEO),
		byte(uxn.LIT), 0x01, byte(uxn.LIT), 0x0f, byte(uxn.DEO),
		byte(uxn.LIT), 0x00, byte(uxn.LIT), 0x0f, byte(uxn.DEO),
		byte(uxn.LIT), 'x', byte(uxn.LIT), 0x18, byte(uxn.DEO),
		byte(uxn.LIT), 0x80, byte(uxn.LIT), 0x0f, byte(uxn.DEO),
		byte(uxn.BRK),
	}
	var output bytes.Buffer
	r := NewRunner(false, false, nil)
	r.SetOutput(&output)

	if got, want := r.Run(rom), 0; got != want {
		t.Errorf("Run exit code = %d, want %d", got, want)
	}
	if got, want := output.String(), "x"; got != want {
		t.Errorf("Run output = %q, want %q", got, want)
	}
}
