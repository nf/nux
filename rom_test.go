package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nf/nux/varvara"
)

func TestROMs(t *testing.T) {
	roms, err := filepath.Glob("tests/*.rom")
	if err != nil {
		t.Fatal(err)
	}
	if len(roms) == 0 {
		t.Fatal("no ROMs found in tests")
	}

	for _, rom := range roms {
		rom, err := filepath.Abs(rom)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(rom), func(t *testing.T) {
			code, output := runROM(t, rom)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}

			golden := rom + ".golden"
			if os.Getenv("GENERATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, output, 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), string(output)); diff != "" {
				t.Errorf("output mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func runROM(t *testing.T, path string) (int, []byte) {
	t.Helper()
	rom, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	type result struct {
		code   int
		output []byte
	}
	done := make(chan result, 1)
	go func() {
		var output bytes.Buffer
		r := varvara.NewRunner(false, false, nil)
		r.SetOutput(&output)
		done <- result{r.Run(rom), output.Bytes()}
	}()

	select {
	case result := <-done:
		return result.code, result.output
	case <-time.After(5 * time.Second):
		t.Fatal("ROM did not exit within 5 seconds")
		return 0, nil
	}
}
