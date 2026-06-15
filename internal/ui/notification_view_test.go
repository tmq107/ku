package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/kli/internal/k8s"
)

func TestNotificationOverlayRendersTopRight(t *testing.T) {
	th := PickTheme("ansi")
	app := App{
		client:    &k8s.Client{ContextName: "test"},
		theme:     th,
		keys:      defaultKeys(),
		width:     80,
		height:    20,
		screen:    screenTable,
		res:       k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
		namespace: "default",
		focus:     focusMain,
	}
	app.table = newTableView(th)
	app.table.setData(fakeTable())
	app.relayout()
	app.setStatus("pods failed to load", true)

	out := app.View().Content
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "ERROR") || !strings.Contains(plain, "pods failed to load") {
		t.Fatalf("notification missing from view:\n%s", plain)
	}

	lines := strings.Split(plain, "\n")
	if len(lines) != app.height {
		t.Fatalf("rendered %d lines, want %d", len(lines), app.height)
	}
	if strings.Contains(lines[len(lines)-1], "pods failed to load") {
		t.Fatalf("status leaked into footer instead of overlay: %q", lines[len(lines)-1])
	}

	foundRight := false
	for _, line := range lines {
		if col := strings.Index(line, "ERROR"); col > app.width/2 {
			foundRight = true
		}
		if w := lipgloss.Width(line); w > app.width {
			t.Fatalf("line width %d exceeds %d: %q", w, app.width, line)
		}
	}
	if !foundRight {
		t.Fatalf("notification was not rendered on the right side:\n%s", plain)
	}
}

func TestOverlayBlockReplacesBaseCells(t *testing.T) {
	base := strings.Join([]string{
		"0123456789",
		"abcdefghij",
		"ABCDEFGHIJ",
	}, "\n")

	got := overlayBlock(base, "XX\nYY", 8, 1, 10, 3)
	want := strings.Join([]string{
		"0123456789",
		"abcdefghXX",
		"ABCDEFGHYY",
	}, "\n")
	if got != want {
		t.Fatalf("overlayBlock() = %q, want %q", got, want)
	}
}
