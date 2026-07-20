package k8s

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodContainersMarksPreviousInstances(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "app"}, {Name: "sidecar"}},
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{{
				Name:                 "init",
				RestartCount:         1,
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ContainerID: "containerd://init-old", ExitCode: 0}},
			}},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:                 "app",
					RestartCount:         2,
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ContainerID: "containerd://app-old", ExitCode: 1}},
				},
				{
					Name:                 "sidecar",
					RestartCount:         1,
					LastTerminationState: corev1.ContainerState{},
				},
			},
		},
	}
	c := &Client{clientset: fake.NewSimpleClientset(pod)}

	got, err := c.PodContainers(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("PodContainers() error = %v", err)
	}
	want := []PodContainer{
		{Name: "init", PreviousAvailable: true},
		{Name: "app", PreviousAvailable: true},
		{Name: "sidecar", PreviousAvailable: false},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("PodContainers() = %#v, want %#v", got, want)
	}
}

func TestHasPreviousInstanceRequiresContainerID(t *testing.T) {
	tests := []struct {
		name   string
		status corev1.ContainerStatus
		want   bool
	}{
		{
			name: "terminated container",
			status: corev1.ContainerStatus{
				RestartCount:         1,
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ContainerID: "containerd://old"}},
			},
			want: true,
		},
		{
			name: "missing container ID",
			status: corev1.ContainerStatus{
				RestartCount:         1,
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}},
			},
		},
		{
			name: "restart count reset",
			status: corev1.ContainerStatus{
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ContainerID: "containerd://old"}},
			},
			want: true,
		},
		{name: "no terminated container"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasPreviousInstance(tt.status); got != tt.want {
				t.Fatalf("hasPreviousInstance() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPodLogOptionsModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     LogMode
		follow   bool
		previous bool
	}{
		{name: "current follows", mode: LogCurrent, follow: true},
		{name: "previous is finite", mode: LogPrevious, previous: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := podLogOptions("app", 1000, tt.mode)
			if opts.Container != "app" || opts.Follow != tt.follow || opts.Previous != tt.previous {
				t.Fatalf("podLogOptions() = %#v", opts)
			}
			if opts.TailLines == nil || *opts.TailLines != 1000 {
				t.Fatalf("podLogOptions() tail = %v, want 1000", opts.TailLines)
			}
		})
	}
}

func TestDeploymentLogTargets(t *testing.T) {
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
		},
	}
	podA := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-a", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{Name: "init"}},
			Containers:     []corev1.Container{{Name: "web"}, {Name: "sidecar"}},
		},
	}
	podB := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-b", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "worker"}}},
	}
	unmatched := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default", Labels: map[string]string{"app": "other"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web"}}},
	}
	c := &Client{clientset: fake.NewSimpleClientset(dep, unmatched, podB, podA)}

	got, err := c.DeploymentLogTargets(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("DeploymentLogTargets() error = %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, t := range got {
		ids = append(ids, t.Pod+"/"+t.Container)
	}
	want := []string{"api-a/init", "api-a/web", "api-a/sidecar", "api-b/worker"}
	if !slices.Equal(ids, want) {
		t.Fatalf("DeploymentLogTargets() = %v, want %v", ids, want)
	}
}
