package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
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

func TestDevBuildFiles(t *testing.T) {
	dir, err := os.MkdirTemp(".", ".nux-dev-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	include := filepath.Join(dir, "include.tal")
	if err := os.WriteFile(include, []byte("( included )\n|100 @on-reset BRK\n"), 0644); err != nil {
		t.Fatal(err)
	}
	talFile, err := filepath.Abs(filepath.Join(dir, "main.tal"))
	if err != nil {
		t.Fatal(err)
	}
	source := "~" + filepath.ToSlash(include) + "\n"
	if err := os.WriteFile(talFile, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, got, err := devBuildFiles(&output, talFile, filepath.Join(dir, "main.rom"))
	if err != nil {
		t.Fatalf("%v:\n%s", err, output.String())
	}
	for _, want := range []string{filepath.ToSlash(talFile), filepath.ToSlash(include)} {
		if !slices.Contains(got, want) {
			t.Errorf("read files %q do not contain %q", got, want)
		}
	}
}
