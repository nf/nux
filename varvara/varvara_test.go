package varvara

import (
	"io"
	"testing"
)

func TestUnimplementedDeviceMemoryWrap(t *testing.T) {
	v := New(nil, nil, io.Discard, io.Discard)
	v.OutShort(0xff, 0x1234)
	if got, want := v.InShort(0xff), uint16(0x1234); got != want {
		t.Fatalf("InShort(ff) = %.4x, want %.4x", got, want)
	}
}
