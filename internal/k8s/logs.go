package k8s

import (
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodContainers returns the container names of a pod (init containers first,
// then regular containers), used to pick which log stream to follow.
func (c *Client) PodContainers(ctx context.Context, namespace, pod string) ([]string, error) {
	p, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(p.Spec.InitContainers)+len(p.Spec.Containers))
	for i := range p.Spec.InitContainers {
		names = append(names, p.Spec.InitContainers[i].Name)
	}
	for i := range p.Spec.Containers {
		names = append(names, p.Spec.Containers[i].Name)
	}
	return names, nil
}

// LogStream opens a log stream for a pod container. When follow is true the
// caller must close the returned reader (and/or cancel ctx) to stop it.
func (c *Client) LogStream(ctx context.Context, namespace, pod, container string, tail int64, follow bool) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}
	if tail >= 0 {
		opts.TailLines = &tail
	}
	return c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
}
