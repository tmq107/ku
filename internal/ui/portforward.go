package ui

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/bjarneo/ku/internal/k8s"
)

type portForward struct {
	id           int
	target       target
	spec         k8s.PortForwardSpec
	serviceLabel string
	localPort    int32
	ready        bool
	stopping     bool
	stop         func()
}

type portForwardResult struct {
	ready chan struct{}
	done  chan struct{}
	err   error
}

type portForwardReadyMsg struct{ id int }

type portForwardDoneMsg struct {
	id  int
	err error
}

func (a App) startServicePortForward(spec k8s.PortForwardSpec) (tea.Model, tea.Cmd) {
	cl := a.client
	if cl == nil {
		a.setStatus("port-forward: client unavailable", true)
		return a, nil
	}
	t := a.portForwardTarget
	if t.name == "" || t.ns == "" {
		a.setStatus("port-forward: service unavailable", true)
		return a, nil
	}

	port, ok := a.servicePortForSpec(spec.ServicePort)
	ctx, cancel := context.WithCancel(context.Background())
	stopCh := make(chan struct{})
	var stopOnce sync.Once
	stop := func() {
		cancel()
		stopOnce.Do(func() { close(stopCh) })
	}

	a.nextPortForwardID++
	f := portForward{
		id:           a.nextPortForwardID,
		target:       t,
		spec:         spec,
		serviceLabel: portForwardServiceLabel(spec, port, ok),
		localPort:    portForwardLocalPort(spec, port, ok),
		stop:         stop,
	}
	result := &portForwardResult{ready: make(chan struct{}), done: make(chan struct{})}
	a.portForwards = append(a.portForwards, f)
	a.setStatus("starting port-forward: "+f.display(), false)

	go func() {
		err := cl.PortForwardService(ctx, t.ns, t.name, spec, stopCh, result.ready, io.Discard, io.Discard)
		result.err = err
		close(result.done)
	}()

	return a, tea.Batch(waitPortForwardReady(f.id, result), waitPortForwardDone(f.id, result))
}

func waitPortForwardReady(id int, r *portForwardResult) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-r.ready:
			return portForwardReadyMsg{id: id}
		case <-r.done:
			return nil
		}
	}
}

func waitPortForwardDone(id int, r *portForwardResult) tea.Cmd {
	return func() tea.Msg {
		<-r.done
		return portForwardDoneMsg{id: id, err: r.err}
	}
}

func (a App) handlePortForwardReady(m portForwardReadyMsg) (tea.Model, tea.Cmd) {
	idx := a.portForwardIndex(m.id)
	if idx < 0 {
		return a, nil
	}
	if a.portForwards[idx].stopping {
		return a, nil
	}
	a.portForwards[idx].ready = true
	a.setStatus("port-forward active: "+a.portForwards[idx].display(), false)
	a.refreshPortForwardSelector()
	return a, nil
}

func (a App) handlePortForwardDone(m portForwardDoneMsg) (tea.Model, tea.Cmd) {
	idx := a.portForwardIndex(m.id)
	if idx < 0 {
		return a, nil
	}
	f := a.portForwards[idx]
	a.portForwards = append(a.portForwards[:idx], a.portForwards[idx+1:]...)

	switch {
	case f.stopping:
		a.setStatus("port-forward stopped: "+f.display(), false)
	case m.err != nil && !errors.Is(m.err, context.Canceled):
		a.setStatus("port-forward ended: "+trimErr(m.err), true)
	default:
		a.setStatus("port-forward ended: "+f.display(), false)
	}
	a.refreshPortForwardSelector()
	return a, nil
}

func (a App) openPortForwards() (tea.Model, tea.Cmd) {
	if len(a.portForwards) == 0 {
		a.setStatus("no active port-forwards", false)
		return a, nil
	}
	a.sel.open(selPortForward, "Port-forwards", "select to stop", a.portForwardItems(), false)
	a.overlay = overlaySelector
	return a, nil
}

func (a App) stopPortForward(id int) (tea.Model, tea.Cmd) {
	idx := a.portForwardIndex(id)
	if idx < 0 {
		a.setStatus("port-forward not found", true)
		return a, nil
	}
	if !a.portForwards[idx].stopping {
		a.portForwards[idx].stopping = true
		if a.portForwards[idx].stop != nil {
			a.portForwards[idx].stop()
		}
	}
	a.setStatus("stopping port-forward: "+a.portForwards[idx].display(), false)
	a.refreshPortForwardSelector()
	return a, nil
}

func (a *App) stopPortForwards() {
	for i := range a.portForwards {
		a.portForwards[i].stopping = true
		if a.portForwards[i].stop != nil {
			a.portForwards[i].stop()
		}
	}
	a.portForwards = nil
}

func (a App) portForwardIndex(id int) int {
	for i, f := range a.portForwards {
		if f.id == id {
			return i
		}
	}
	return -1
}

func (a App) portForwardItems() []selItem {
	items := make([]selItem, 0, len(a.portForwards))
	for _, f := range a.portForwards {
		items = append(items, selItem{title: f.display(), desc: f.state(), id: itoa(f.id)})
	}
	return items
}

func (a *App) refreshPortForwardSelector() {
	if a.overlay != overlaySelector || a.sel.kind != selPortForward {
		return
	}
	if len(a.portForwards) == 0 {
		a.overlay = overlayNone
		return
	}
	a.sel.setItems(a.portForwardItems())
}

func (a App) servicePortForSpec(servicePort string) (k8s.ServicePort, bool) {
	servicePort = strings.TrimSpace(servicePort)
	for _, p := range a.portForwardPorts {
		if p.ID() == servicePort || (p.Name != "" && p.Name == servicePort) || itoa(int(p.Port)) == servicePort {
			return p, true
		}
	}
	return k8s.ServicePort{}, false
}

func portForwardLocalPort(spec k8s.PortForwardSpec, port k8s.ServicePort, ok bool) int32 {
	if spec.LocalPort > 0 {
		return spec.LocalPort
	}
	if ok && port.Port > 0 {
		return port.Port
	}
	n, err := strconv.Atoi(strings.TrimSpace(spec.ServicePort))
	if err != nil || n < 1 || n > 65535 {
		return 0
	}
	return int32(n)
}

func portForwardServiceLabel(spec k8s.PortForwardSpec, port k8s.ServicePort, ok bool) string {
	if ok {
		return port.ID()
	}
	return strings.TrimSpace(spec.ServicePort)
}

func (f portForward) display() string {
	local := "localhost:?"
	if f.localPort > 0 {
		local = "localhost:" + itoa(int(f.localPort))
	}
	remote := "service/" + qualified(f.target.ns, f.target.name)
	if f.serviceLabel != "" {
		remote += ":" + f.serviceLabel
	}
	return local + " -> " + remote
}

func (f portForward) state() string {
	switch {
	case f.stopping:
		return "stopping"
	case f.ready:
		return "active"
	default:
		return "starting"
	}
}
