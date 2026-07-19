package k8s

import (
	"context"
	"fmt"
	"sort"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobInfo summarizes a Job owned by a CronJob.
type JobInfo struct {
	Name        string
	Namespace   string
	Active      int32
	Succeeded   int32
	Failed      int32
	Completions string // e.g. "2/3" (succeeded / desired)
	Created     string // RFC3339 creation timestamp
}

// CronJobJobs lists Jobs owned by a CronJob, sorted newest first.
func (c *Client) CronJobJobs(ctx context.Context, namespace, name string) ([]JobInfo, error) {
	cj, err := c.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	all, err := c.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []JobInfo
	for i := range all.Items {
		j := &all.Items[i]
		if !metav1.IsControlledBy(j, cj) {
			continue
		}
		out = append(out, jobInfo(j))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Created > out[j].Created
	})
	return out, nil
}

func jobInfo(j *batchv1.Job) JobInfo {
	desired := int32(1)
	if j.Spec.Completions != nil {
		desired = *j.Spec.Completions
	}
	return JobInfo{
		Name:        j.Name,
		Namespace:   j.Namespace,
		Active:      j.Status.Active,
		Succeeded:   j.Status.Succeeded,
		Failed:      j.Status.Failed,
		Completions: fmt.Sprintf("%d/%d", j.Status.Succeeded, desired),
		Created:     j.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}
}

// JobPods lists pods belonging to a Job, ordered with running/ready pods first.
// It first tries the standard job-name label, then falls back to ownerReference
// matching for controllers that skip the label.
func (c *Client) JobPods(ctx context.Context, namespace, name string) (*NodePods, error) {
	sel := "batch.kubernetes.io/job-name=" + name
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return nil, err
	}
	if len(list.Items) > 0 {
		items := append([]corev1.Pod(nil), list.Items...)
		return summarizePods(items), nil
	}
	// Fallback: list all pods in the namespace and match by ownerReference.
	all, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var matched []corev1.Pod
	for i := range all.Items {
		p := &all.Items[i]
		for _, ref := range p.OwnerReferences {
			if ref.Kind == "Job" && ref.Name == name {
				matched = append(matched, *p)
				break
			}
		}
	}
	return summarizePods(matched), nil
}
