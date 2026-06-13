package k8s

import (
	"context"
	"io"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

// defaultShell prefers bash when present, falling back to sh, so the same
// command works across most container images.
var defaultShell = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}

// ResizeQueue feeds terminal size updates to an exec stream. The UI sets the
// size as its embedded-terminal panel changes; only the most recent size is
// kept.
type ResizeQueue struct {
	ch   chan remotecommand.TerminalSize
	done chan struct{}
	once sync.Once
}

func NewResizeQueue() *ResizeQueue {
	return &ResizeQueue{
		ch:   make(chan remotecommand.TerminalSize, 1),
		done: make(chan struct{}),
	}
}

// Set records a new size, replacing any pending one.
func (q *ResizeQueue) Set(cols, rows int) {
	if cols <= 0 || rows <= 0 {
		return
	}
	sz := remotecommand.TerminalSize{Width: uint16(cols), Height: uint16(rows)}
	select {
	case <-q.ch: // drop a stale pending size
	default:
	}
	select {
	case q.ch <- sz:
	case <-q.done:
	default:
	}
}

// Next implements remotecommand.TerminalSizeQueue.
func (q *ResizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case s := <-q.ch:
		return &s
	case <-q.done:
		return nil
	}
}

// Close stops the queue so the executor's size goroutine can exit.
func (q *ResizeQueue) Close() {
	q.once.Do(func() { close(q.done) })
}

// ExecStream runs an interactive shell in a pod container over SPDY, wiring the
// given streams. stdin/stdout are typically a virtual terminal emulator. The
// call blocks until the shell exits or ctx is cancelled.
func (c *Client) ExecStream(ctx context.Context, ns, pod, container string, stdin io.Reader, stdout io.Writer, q *ResizeQueue) error {
	req := c.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(pod).
		Namespace(ns).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   defaultShell,
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    false, // merged into stdout under a TTY
			TTY:       true,
		}, scheme.ParameterCodec)

	// Prefer the WebSocket protocol (Kubernetes 1.29+) and fall back to SPDY on
	// upgrade failure, matching kubectl, so exec works on clusters with SPDY
	// disabled.
	url := req.URL()
	spdyExec, err := remotecommand.NewSPDYExecutor(c.restConfig, "POST", url)
	if err != nil {
		return err
	}
	wsExec, err := remotecommand.NewWebSocketExecutor(c.restConfig, "GET", url.String())
	if err != nil {
		return err
	}
	executor, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return err
	}

	var sizeQueue remotecommand.TerminalSizeQueue
	if q != nil {
		sizeQueue = q
	}
	return executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})
}
