package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/kli/internal/k8s"
)

func TestOverlayCenterKeepsBackgroundVisible(t *testing.T) {
	base := strings.TrimRight(strings.Repeat("background-row\n", 10), "\n")
	box := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Render("MODAL")

	out := overlayCenter(base, box, 40, 10)

	if !strings.Contains(out, "MODAL") {
		t.Fatalf("overlay box not composited on top:\n%s", out)
	}
	if !strings.Contains(out, "background-row") {
		t.Fatalf("background should stay visible around the overlay:\n%s", out)
	}
}

// TestCommandOverlayFloatsOverScreen asserts the command overlay (C) renders on
// top of the current screen instead of replacing it: both the command and the
// underlying table are present in one frame.
func TestCommandOverlayFloatsOverScreen(t *testing.T) {
	th := PickTheme("ansi")
	app := App{
		client:    &k8s.Client{ContextName: "test"},
		theme:     th,
		keys:      defaultKeys(),
		width:     120,
		height:    40,
		screen:    screenTable,
		res:       k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
		namespace: "default",
		focus:     focusMain,
	}
	app.table = newTableView(th)
	app.table.setData(fakeTable())
	app.relayout()

	app.command = newCommandView(th)
	app.command.setCommand("kubectl get pods -n default")
	app.overlay = overlayCommand

	plain := ansi.Strip(app.View().Content)
	if !strings.Contains(plain, "kubectl get pods") {
		t.Fatalf("command overlay not shown:\n%s", plain)
	}
	if !strings.Contains(plain, "api-7d9") {
		t.Fatalf("underlying table should stay visible behind the overlay:\n%s", plain)
	}
}
