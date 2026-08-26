package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDevBuild(t *testing.T) {
	wantROM, err := os.ReadFile("tests/varvara.file.rom")
	if err != nil {
		t.Fatal(err)
	}
	wantSymbols, err := os.ReadFile("tests/varvara.file.rom.sym")
	if err != nil {
		t.Fatal(err)
	}
	talFile, err := filepath.Abs("tests/varvara.file.tal")
	if err != nil {
		t.Fatal(err)
	}
	romFile := filepath.Join(t.TempDir(), "varvara.file.rom")

	gotROM, err := devBuild(io.Discard, talFile, romFile)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantROM, gotROM); diff != "" {
		t.Errorf("ROM mismatch (-want +got):\n%s", diff)
	}
	gotSymbols, err := os.ReadFile(romFile + ".sym")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantSymbols, gotSymbols); diff != "" {
		t.Errorf("symbols mismatch (-want +got):\n%s", diff)
	}
}
