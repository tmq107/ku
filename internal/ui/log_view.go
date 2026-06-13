package ui

import (
	"bufio"
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bjarneo/kli/internal/k8s"
)

const maxLogLines = 5000

// logView streams a pod container's logs into a viewport. The stream always
// follows server-side; the follow flag controls whether new lines auto-scroll
// to the bottom (so the user can scroll up to read history).
type logView struct {
	th     Theme
	vp     viewport.Model
	title  string
	ns     string
	pod    string
	cont   string
	follow bool

	session int
	cancel  context.CancelFunc
	ch      chan logEvent
	lines   []string
}

func newLogView(th Theme) logView {
	return logView{th: th, vp: viewport.New(0, 0), follow: true}
}

func (l *logView) setSize(w, h int) {
	if h < 1 {
		h = 1
	}
	l.vp.Width = w
	l.vp.Height = h - 1
}

func (l *logView) appendLine(s string) {
	l.lines = append(l.lines, s)
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
	}
	l.vp.SetContent(strings.Join(l.lines, "\n"))
	if l.follow {
		l.vp.GotoBottom()
	}
}

func (l *logView) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
}

func (l logView) Update(msg tea.Msg) (logView, tea.Cmd) {
	var cmd tea.Cmd
	l.vp, cmd = l.vp.Update(msg)
	return l, cmd
}

func (l logView) View() string {
	state := "following"
	style := l.th.Good
	if !l.follow {
		state = "paused"
		style = l.th.Warn
	}
	right := style.Render("● " + state)
	title := l.th.ModalTitle.Render(l.title)
	gap := l.vp.Width - lipgloss.Width(title) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	header := title + strings.Repeat(" ", gap) + right
	return header + "\n" + l.vp.View()
}

// streamLogs opens the log stream and feeds lines onto ch until the context is
// canceled or the stream ends. It always delivers exactly one terminal event
// (done) via defer so the waiting command never leaks.
func streamLogs(ctx context.Context, cl *k8s.Client, ns, pod, cont string, session int, ch chan logEvent) {
	defer func() {
		select {
		case ch <- logEvent{session: session, done: true}:
		default:
		}
	}()

	rc, err := cl.LogStream(ctx, ns, pod, cont, logTailLines, true)
	if err != nil {
		select {
		case ch <- logEvent{session: session, err: err}:
		case <-ctx.Done():
		}
		return
	}
	defer rc.Close()

	sc := bufio.NewScanner(rc)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		case ch <- logEvent{session: session, line: sc.Text()}:
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil {
		select {
		case ch <- logEvent{session: session, err: err}:
		case <-ctx.Done():
		}
	}
}

func waitForLog(ch chan logEvent) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}
