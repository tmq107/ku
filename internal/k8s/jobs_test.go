package k8s

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func int32Ptr(v int32) *int32 { return &v }

func TestCronJobJobsReturnsOnlyOwnedJobs(t *testing.T) {
	cjUID := types.UID("cj-uid-1")
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "nightly", Namespace: "default", UID: cjUID},
	}
	owned := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nightly-1", Namespace: "default",
			CreationTimestamp: metav1.Now(),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "nightly", UID: cjUID, Controller: boolPtr(true)}},
		},
		Spec:   batchv1.JobSpec{Completions: int32Ptr(3)},
		Status: batchv1.JobStatus{Succeeded: 2},
	}
	unrelated := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "other-job", Namespace: "default"},
	}
	cs := fake.NewSimpleClientset(cj, owned, unrelated)
	c := &Client{clientset: cs}

	jobs, err := c.CronJobJobs(context.Background(), "default", "nightly")
	if err != nil {
		t.Fatalf("CronJobJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].Name != "nightly-1" {
		t.Fatalf("job name = %q, want nightly-1", jobs[0].Name)
	}
	if jobs[0].Completions != "2/3" {
		t.Fatalf("completions = %q, want 2/3", jobs[0].Completions)
	}
}

func TestCronJobJobsSortedNewestFirst(t *testing.T) {
	cjUID := types.UID("cj-uid-2")
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "hourly", Namespace: "ns", UID: cjUID},
	}
	older := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hourly-old",
			Namespace:         "ns",
			CreationTimestamp: metav1.Unix(1000, 0),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "hourly", UID: cjUID, Controller: boolPtr(true)}},
		},
	}
	newer := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "hourly-new",
			Namespace:         "ns",
			CreationTimestamp: metav1.Unix(2000, 0),
			OwnerReferences:   []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "CronJob", Name: "hourly", UID: cjUID, Controller: boolPtr(true)}},
		},
	}
	cs := fake.NewSimpleClientset(cj, older, newer)
	c := &Client{clientset: cs}

	jobs, err := c.CronJobJobs(context.Background(), "ns", "hourly")
	if err != nil {
		t.Fatalf("CronJobJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("len(jobs) = %d, want 2", len(jobs))
	}
	if jobs[0].Name != "hourly-new" {
		t.Fatalf("first job = %q, want hourly-new (newest first)", jobs[0].Name)
	}
}

func TestCronJobJobsEmpty(t *testing.T) {
	cjUID := types.UID("cj-uid-3")
	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "ns", UID: cjUID},
	}
	cs := fake.NewSimpleClientset(cj)
	c := &Client{clientset: cs}

	jobs, err := c.CronJobJobs(context.Background(), "ns", "empty")
	if err != nil {
		t.Fatalf("CronJobJobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("len(jobs) = %d, want 0", len(jobs))
	}
}

func TestJobPodsViaLabelSelector(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "j-run-1", Namespace: "ns",
				Labels: map[string]string{"batch.kubernetes.io/job-name": "myjob"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "j-run-2", Namespace: "ns",
				Labels: map[string]string{"batch.kubernetes.io/job-name": "myjob"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)
	c := &Client{clientset: cs}

	info, err := c.JobPods(context.Background(), "ns", "myjob")
	if err != nil {
		t.Fatalf("JobPods: %v", err)
	}
	if len(info.Pods) != 2 {
		t.Fatalf("len(Pods) = %d, want 2", len(info.Pods))
	}
	if info.Ready != 1 {
		t.Fatalf("ready = %d, want 1", info.Ready)
	}
}

func TestJobPodsFallbackOwnerRef(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "j-fallback", Namespace: "ns",
				OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: "legacy-job"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: "ns"},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)
	c := &Client{clientset: cs}

	info, err := c.JobPods(context.Background(), "ns", "legacy-job")
	if err != nil {
		t.Fatalf("JobPods: %v", err)
	}
	if len(info.Pods) != 1 {
		t.Fatalf("len(Pods) = %d, want 1", len(info.Pods))
	}
	if info.Pods[0].Name != "j-fallback" {
		t.Fatalf("pod = %q, want j-fallback", info.Pods[0].Name)
	}
}

func TestJobPodsEmpty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{clientset: cs}

	info, err := c.JobPods(context.Background(), "ns", "nope")
	if err != nil {
		t.Fatalf("JobPods: %v", err)
	}
	if len(info.Pods) != 0 {
		t.Fatalf("len(Pods) = %d, want 0", len(info.Pods))
	}
}

func boolPtr(b bool) *bool { return &b }
