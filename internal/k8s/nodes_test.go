package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCordonUncordonRoundTrip(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}})
	c := &Client{clientset: cs}
	ctx := context.Background()

	if cordoned, err := c.NodeCordoned(ctx, "node-1"); err != nil || cordoned {
		t.Fatalf("fresh node: cordoned=%v err=%v, want false/nil", cordoned, err)
	}
	if err := c.Cordon(ctx, "node-1"); err != nil {
		t.Fatalf("Cordon: %v", err)
	}
	if cordoned, err := c.NodeCordoned(ctx, "node-1"); err != nil || !cordoned {
		t.Fatalf("after Cordon: cordoned=%v err=%v, want true/nil", cordoned, err)
	}
	if err := c.Uncordon(ctx, "node-1"); err != nil {
		t.Fatalf("Uncordon: %v", err)
	}
	if cordoned, err := c.NodeCordoned(ctx, "node-1"); err != nil || cordoned {
		t.Fatalf("after Uncordon: cordoned=%v err=%v, want false/nil", cordoned, err)
	}
}

func TestNodePodsListsScheduledPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns2"}, Spec: corev1.PodSpec{NodeName: "node-1"}, Status: corev1.PodStatus{Phase: corev1.PodPending}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns1"}, Spec: corev1.PodSpec{NodeName: "node-1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}, Spec: corev1.PodSpec{NodeName: "node-1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns1"}, Spec: corev1.PodSpec{NodeName: "node-2"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	)
	c := &Client{clientset: cs}
	info, err := c.NodePods(context.Background(), "node-1")
	if err != nil {
		t.Fatalf("NodePods: %v", err)
	}
	if info.Ready != 1 || info.Running != 2 || len(info.Pods) != 3 {
		t.Fatalf("counts = ready %d running %d len %d, want 1/2/3", info.Ready, info.Running, len(info.Pods))
	}
	if info.Pods[0].Namespace != "ns1" || info.Pods[0].Name != "a" || !info.Pods[0].Ready {
		t.Fatalf("first pod = %+v, want ns1/a ready", info.Pods[0])
	}
}

func TestNamespacePodsListsPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "ns1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns1"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns2"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	)
	c := &Client{clientset: cs}
	info, err := c.NamespacePods(context.Background(), "ns1")
	if err != nil {
		t.Fatalf("NamespacePods: %v", err)
	}
	if info.Ready != 1 || info.Running != 2 || len(info.Pods) != 2 {
		t.Fatalf("counts = ready %d running %d len %d, want 1/2/2", info.Ready, info.Running, len(info.Pods))
	}
	if info.Pods[0].Name != "a" {
		t.Fatalf("first pod = %+v, want a", info.Pods[0])
	}
}

func TestDrainSkipReason(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		wantReason string
		wantSkip   bool
	}{
		{
			name:     "normal pod is evicted",
			pod:      &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}},
			wantSkip: false,
		},
		{
			name:       "completed pod is skipped",
			pod:        &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
			wantReason: "completed",
			wantSkip:   true,
		},
		{
			name: "daemonset pod is skipped",
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "fluentd"}},
			}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
			wantReason: "daemonset",
			wantSkip:   true,
		},
		{
			name: "mirror pod is skipped",
			pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "x"},
			}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
			wantReason: "mirror pod",
			wantSkip:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, skip := drainSkipReason(tt.pod)
			if skip != tt.wantSkip || reason != tt.wantReason {
				t.Fatalf("drainSkipReason = (%q, %v), want (%q, %v)", reason, skip, tt.wantReason, tt.wantSkip)
			}
		})
	}
}
