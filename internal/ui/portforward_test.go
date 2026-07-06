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

func TestOpenPortForwardsShowsActiveForward(t *testing.T) {
	app := App{
		theme: PickTheme("ansi"),
		portForwards: []portForward{{
			id:           4,
			target:       target{ns: "default", name: "api"},
			serviceLabel: "http",
			localPort:    8080,
			ready:        true,
		}},
	}
	app.sel = newSelector(app.theme)

	model, cmd := app.openPortForwards()
	got := model.(App)
	if cmd != nil {
		t.Fatal("openPortForwards returned command")
	}
	if got.overlay != overlaySelector || got.sel.kind != selPortForward {
		t.Fatalf("overlay/kind = %v/%v, want port-forward selector", got.overlay, got.sel.kind)
	}
	item, ok := got.sel.current()
	if !ok || item.id != "4" || item.title != "localhost:8080 -> service/default/api:http" || item.desc != "active" {
		t.Fatalf("current selector item = %+v ok=%v", item, ok)
	}
}

func TestStopPortForwardMarksStopping(t *testing.T) {
	stopped := false
	app := App{portForwards: []portForward{{
		id:           7,
		target:       target{ns: "default", name: "api"},
		serviceLabel: "http",
		localPort:    8080,
		stop:         func() { stopped = true },
	}}}

	model, cmd := app.stopPortForward(7)
	got := model.(App)
	if cmd != nil {
		t.Fatal("stopPortForward returned command")
	}
	if !stopped {
		t.Fatal("stopPortForward did not call stop")
	}
	if !got.portForwards[0].stopping {
		t.Fatal("port-forward was not marked stopping")
	}
	if got.status != "stopping port-forward: localhost:8080 -> service/default/api:http" || got.statusErr {
		t.Fatalf("status = %q err=%t", got.status, got.statusErr)
	}
}

func TestPortForwardDoneRemovesForward(t *testing.T) {
	app := App{portForwards: []portForward{
		{id: 1, target: target{ns: "default", name: "api"}, serviceLabel: "http", localPort: 8080, stopping: true},
		{id: 2, target: target{ns: "default", name: "metrics"}, serviceLabel: "http", localPort: 9090},
	}}

	model, cmd := app.handlePortForwardDone(portForwardDoneMsg{id: 1})
	got := model.(App)
	if cmd != nil {
		t.Fatal("handlePortForwardDone returned command")
	}
	if len(got.portForwards) != 1 || got.portForwards[0].id != 2 {
		t.Fatalf("portForwards = %+v, want only id 2", got.portForwards)
	}
	if got.status != "port-forward stopped: localhost:8080 -> service/default/api:http" || got.statusErr {
		t.Fatalf("status = %q err=%t", got.status, got.statusErr)
	}
}
