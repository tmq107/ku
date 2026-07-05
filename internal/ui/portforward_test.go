package ui

import (
	"testing"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestParseServicePortForwardSpec(t *testing.T) {
	tests := []struct {
		name  string
		id    string
		value string
		want  k8s.PortForwardSpec
	}{
		{name: "selected named port", id: "http", want: k8s.PortForwardSpec{ServicePort: "http"}},
		{name: "typed service port", value: "9090", want: k8s.PortForwardSpec{ServicePort: "9090"}},
		{name: "typed local and named service port", value: "8080:http", want: k8s.PortForwardSpec{LocalPort: 8080, ServicePort: "http"}},
		{name: "typed local and numeric service port", value: "18080:80", want: k8s.PortForwardSpec{LocalPort: 18080, ServicePort: "80"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServicePortForwardSpec(tt.id, tt.value)
			if err != nil {
				t.Fatalf("parseServicePortForwardSpec: %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseServicePortForwardSpec = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseServicePortForwardSpecRejectsBadLocalPort(t *testing.T) {
	if _, err := parseServicePortForwardSpec("", "0:http"); err == nil {
		t.Fatal("parseServicePortForwardSpec accepted local port 0")
	}
	if _, err := parseServicePortForwardSpec("", "70000:http"); err == nil {
		t.Fatal("parseServicePortForwardSpec accepted local port 70000")
	}
}

func TestOpenServicePortForwardStartsPortLookup(t *testing.T) {
	th := PickTheme("ansi")
	app := App{
		client:    &k8s.Client{},
		theme:     th,
		keys:      defaultKeys(),
		res:       k8s.ResourceInfo{Resource: "services", Kind: "Service", Namespaced: true},
		namespace: "default",
	}
	app.table = newTableView(th)
	app.table.setData(&k8s.Table{
		Columns: []k8s.Column{{Name: "Name"}},
		Rows:    []k8s.Row{{Namespace: "default", Name: "api", Cells: []string{"api"}}},
	})

	model, cmd := app.openServicePortForward()
	got := model.(App)
	if cmd == nil {
		t.Fatal("openServicePortForward returned nil command")
	}
	if got.portForwardTarget.ns != "default" || got.portForwardTarget.name != "api" || !got.portForwardTarget.res.IsService() {
		t.Fatalf("portForwardTarget = %+v, want default/api service", got.portForwardTarget)
	}
	if got.statusErr || got.status == "" {
		t.Fatalf("status = %q err=%t, want non-error loading status", got.status, got.statusErr)
	}
}

func TestHandleServicePortsOpensSelector(t *testing.T) {
	cl := &k8s.Client{}
	app := App{
		client:            cl,
		theme:             PickTheme("ansi"),
		portForwardTarget: target{res: k8s.ResourceInfo{Resource: "services"}, ns: "default", name: "api"},
	}
	app.sel = newSelector(app.theme)

	model, cmd := app.handleServicePorts(servicePortsMsg{
		client: cl,
		ns:     "default",
		name:   "api",
		ports:  []k8s.ServicePort{{Name: "http", Port: 80, TargetPort: "8080", Protocol: "TCP"}},
	})
	got := model.(App)
	if cmd != nil {
		t.Fatal("handleServicePorts returned command")
	}
	if got.overlay != overlaySelector || got.sel.kind != selServicePort {
		t.Fatalf("overlay/kind = %v/%v, want service port selector", got.overlay, got.sel.kind)
	}
	if item, ok := got.sel.current(); !ok || item.id != "http" {
		t.Fatalf("current selector item = %+v ok=%v, want http", item, ok)
	}
}

func TestPortForwardShortcutStopsActiveForward(t *testing.T) {
	stopped := false
	app := App{
		keys:    defaultKeys(),
		screen:  screenTable,
		overlay: overlayTerm,
		term: termView{
			detachStatus: "port-forward stopped",
			cancel:       func() { stopped = true },
		},
	}

	model, cmd := app.updateTerm(mkKey("p"))
	got := model.(App)
	if cmd != nil {
		t.Fatal("port-forward shortcut returned unexpected command")
	}
	if !stopped {
		t.Fatal("port-forward shortcut did not stop the active forward")
	}
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone", got.overlay)
	}
	if got.status != "port-forward stopped" || got.statusErr {
		t.Fatalf("status = %q err=%t, want stopped status", got.status, got.statusErr)
	}
}
