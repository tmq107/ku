package ui

import (
	"context"
	"testing"
	"time"

	"github.com/bjarneo/ku/internal/k8s"
)

func TestDeploymentLogsKeyStartsLookup(t *testing.T) {
	th := PickTheme("ansi")
	app := App{
		client:    &k8s.Client{},
		theme:     th,
		keys:      defaultKeys(),
		width:     80,
		height:    20,
		screen:    screenTable,
		res:       k8s.ResourceInfo{Group: "apps", Resource: "deployments", Kind: "Deployment", Namespaced: true},
		namespace: "default",
		focus:     focusMain,
	}
	app.table = newTableView(th)
	app.table.setData(&k8s.Table{
		Columns: []k8s.Column{{Name: "Name"}},
		Rows:    []k8s.Row{{Namespace: "default", Name: "api", Cells: []string{"api"}}},
	})

	model, cmd := app.updateMainKeys(mkKey("L"))
	got := model.(App)
	if cmd == nil {
		t.Fatal("deployment logs key returned nil command")
	}
	if got.logTarget.ns != "default" || got.logTarget.name != "api" || !got.logTarget.res.IsDeployment() {
		t.Fatalf("logTarget = %+v, want default/api deployment", got.logTarget)
	}
	if got.status == "" || got.statusErr {
		t.Fatalf("status = %q err=%t, want non-error loading status", got.status, got.statusErr)
	}
}

func TestDeploymentLogsKeyRequiresDeployment(t *testing.T) {
	app := App{
		keys:   defaultKeys(),
		screen: screenTable,
		res:    k8s.ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true},
	}
	app.table = newTableView(PickTheme("ansi"))
	app.table.setData(fakeTable())

	model, cmd := app.updateMainKeys(mkKey("L"))
	got := model.(App)
	if cmd != nil {
		t.Fatal("non-deployment logs key returned command")
	}
	if got.status != "logs: switch to deployments first" || !got.statusErr {
		t.Fatalf("status = %q err=%t, want deployment error", got.status, got.statusErr)
	}
}

func TestDeploymentLogDoneWaitsForRemainingStreams(t *testing.T) {
	app := App{
		screen:     screenLogs,
		logSession: 7,
		logs: logView{
			streams: 2,
			ch:      make(chan logEvent, 1),
		},
	}

	model, cmd := app.Update(logEvent{session: 7, done: true})
	got := model.(App)
	if got.logs.streams != 1 {
		t.Fatalf("streams after first done = %d, want 1", got.logs.streams)
	}
	if cmd == nil {
		t.Fatal("first done returned nil command")
	}

	model, cmd = got.Update(logEvent{session: 7, done: true})
	got = model.(App)
	if got.logs.streams != 0 {
		t.Fatalf("streams after second done = %d, want 0", got.logs.streams)
	}
	if cmd != nil {
		t.Fatal("final done returned command")
	}
}

func TestSendLogEventReleasesWait(t *testing.T) {
	ch := make(chan logEvent, 1)
	sendLogEvent(context.Background(), ch, logEvent{session: 9, done: true})

	msg := waitForLog(ch, nil, 9)()
	got, ok := msg.(logEvent)
	if !ok {
		t.Fatalf("waitForLog returned %T, want logEvent", msg)
	}
	if got.session != 9 || !got.done {
		t.Fatalf("logEvent = %+v, want session 9 done", got)
	}
}

func TestSendLogEventDoesNotDropTerminalEventWhenFull(t *testing.T) {
	ch := make(chan logEvent, 1)
	ch <- logEvent{session: 9, line: "last line"}
	sent := make(chan struct{})
	go func() {
		sendLogEvent(context.Background(), ch, logEvent{session: 9, done: true})
		close(sent)
	}()

	first := <-ch
	var second logEvent
	select {
	case second = <-ch:
	case <-time.After(time.Second):
		t.Fatal("terminal event was dropped")
	}
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("terminal event sender remained blocked")
	}
	if first.line != "last line" || !second.done {
		t.Fatalf("events = %+v, %+v; want final line then done", first, second)
	}
}

func TestSendLogEventStopsWaitingAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan logEvent, 1)
	ch <- logEvent{session: 9, line: "buffered"}
	cancel()

	finished := make(chan struct{})
	go func() {
		sendLogEvent(ctx, ch, logEvent{session: 9, done: true})
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("terminal event sender ignored cancellation")
	}
}

func TestWaitForLogStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	msg := waitForLog(make(chan logEvent), ctx.Done(), 9)()
	got, ok := msg.(logEvent)
	if !ok || got.session != 9 || !got.done {
		t.Fatalf("waitForLog returned %#v, want session 9 done", msg)
	}
}

func TestHandleDeploymentLogsIgnoresOldClient(t *testing.T) {
	current := &k8s.Client{}
	app := App{
		client:    current,
		logTarget: target{res: k8s.ResourceInfo{Group: "apps", Resource: "deployments"}, ns: "default", name: "api"},
		lookupSeq: 2,
		screen:    screenTable,
	}
	msg := deploymentLogsMsg{
		client: &k8s.Client{},
		seq:    2,
		source: screenTable,
		ns:     "default",
		name:   "api",
		targets: []k8s.LogTarget{{
			Namespace: "default",
			Pod:       "api-123",
			Container: "app",
		}},
	}

	model, cmd := app.handleDeploymentLogs(msg)
	got := model.(App)
	if cmd != nil || got.screen != screenTable {
		t.Fatalf("old-client deployment lookup changed app: screen=%v cmd=%v", got.screen, cmd != nil)
	}
}
