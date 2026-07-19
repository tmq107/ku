package k8s

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPodEventsFiltersByName(t *testing.T) {
	now := metav1.NewTime(time.Now())
	podEvents := []corev1.Event{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "api-pod.1", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "api-pod", Kind: "Pod"},
			Type:          "Normal",
			Reason:        "Scheduled",
			Message:       "Successfully assigned default/api-pod to node-1",
			LastTimestamp: now,
			Count:         1,
		},
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "other-pod.1", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "other-pod", Kind: "Pod"},
			Type:          "Warning",
			Reason:        "BackOff",
			Message:       "Back-off restarting failed container",
			LastTimestamp: now,
			Count:         3,
		},
	}
	cs := fake.NewSimpleClientset(&podEvents[0], &podEvents[1])
	c := &Client{clientset: cs}

	events, err := c.PodEvents(context.Background(), "default", "api-pod")
	if err != nil {
		t.Fatalf("PodEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Reason != "Scheduled" {
		t.Fatalf("reason = %q, want Scheduled", events[0].Reason)
	}
	if events[0].Type != "Normal" {
		t.Fatalf("type = %q, want Normal", events[0].Type)
	}
}

func TestPodEventsChronologicalOrder(t *testing.T) {
	older := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	newer := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	events := []corev1.Event{
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "api.2", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "api", Kind: "Pod"},
			Type:          "Normal",
			Reason:        "Started",
			Message:       "Started container",
			LastTimestamp: newer,
			Count:         1,
		},
		{
			ObjectMeta:    metav1.ObjectMeta{Name: "api.1", Namespace: "default"},
			InvolvedObject: corev1.ObjectReference{Name: "api", Kind: "Pod"},
			Type:          "Normal",
			Reason:        "Pulled",
			Message:       "Successfully pulled image",
			LastTimestamp: older,
			Count:         1,
		},
	}
	cs := fake.NewSimpleClientset(&events[0], &events[1])
	c := &Client{clientset: cs}

	got, err := c.PodEvents(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("PodEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(got))
	}
	if got[0].Reason != "Pulled" || got[1].Reason != "Started" {
		t.Fatalf("events not in chronological order: got %s then %s", got[0].Reason, got[1].Reason)
	}
}

func TestPodEventsEmpty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{clientset: cs}

	events, err := c.PodEvents(context.Background(), "default", "nonexistent")
	if err != nil {
		t.Fatalf("PodEvents: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}
