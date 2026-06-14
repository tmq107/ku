package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunUpgradeHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := runUpgrade([]string{"--help"}); err != nil {
			t.Fatalf("runUpgrade(--help): %v", err)
		}
	})
	if !strings.Contains(out, "usage: kli upgrade") {
		t.Fatalf("help output = %q, want usage", out)
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	f()
	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	os.Stdout = old
	return string(b)
}
