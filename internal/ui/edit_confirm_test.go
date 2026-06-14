package ui

import (
	"os"
	"testing"

	"github.com/bjarneo/kli/internal/k8s"
)

func TestApplyEditedFilePromptsBeforeApply(t *testing.T) {
	path := writeTempEdit(t, "changed")
	app := App{theme: PickTheme("ansi")}

	model, cmd := app.applyEditedFile(nil, k8s.ResourceInfo{Resource: "pods", Kind: "Pod"}, "default", "api", path, "original")
	got := model.(App)
	if cmd != nil {
		t.Fatal("applyEditedFile returned an apply command before confirmation")
	}
	if got.overlay != overlayConfirm {
		t.Fatalf("overlay = %v, want overlayConfirm", got.overlay)
	}
	if got.confirm.title != "Apply edit" {
		t.Fatalf("confirm title = %q, want Apply edit", got.confirm.title)
	}
	if got.confirm.action == nil {
		t.Fatal("confirm action is nil")
	}
	if got.confirm.cancel == nil {
		t.Fatal("confirm cancel action is nil")
	}
}

func TestCancelEditRemovesTempFile(t *testing.T) {
	path := writeTempEdit(t, "changed")
	msg := cancelEditCmd(path)()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temp edit file still exists after cancel: err=%v", err)
	}
	status, ok := msg.(statusMsg)
	if !ok || status.text != "edit cancelled" || status.err {
		t.Fatalf("cancelEditCmd msg = %#v, want non-error edit cancelled status", msg)
	}
}

func TestStaleEditReadyRemovesTempFile(t *testing.T) {
	path := writeTempEdit(t, "changed")
	current := &k8s.Client{}
	stale := &k8s.Client{}
	app := App{client: current, theme: PickTheme("ansi")}

	model, cmd := app.Update(editReadyMsg{
		client: stale,
		path:   path,
		res:    k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
		ns:     "default",
		name:   "api",
	})
	got := model.(App)
	if cmd != nil {
		t.Fatal("stale edit returned a command")
	}
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone", got.overlay)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("stale edit temp file still exists: err=%v", err)
	}
}

func writeTempEdit(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "kli-edit-confirm-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}
