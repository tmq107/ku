package k8s

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServicePorts(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
			{Name: "http", Port: 80, TargetPort: intstr.FromString("web")},
			{Port: 9090, TargetPort: intstr.FromInt32(9091)},
		}},
	})
	c := &Client{clientset: cs}

	ports, err := c.ServicePorts(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("ServicePorts: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("len(ServicePorts) = %d, want 2", len(ports))
	}
	if ports[0].ID() != "http" || ports[0].TargetPort != "web" || ports[0].Protocol != "TCP" {
		t.Fatalf("first port = %+v, want named TCP target web", ports[0])
	}
	if ports[1].ID() != "9090" || ports[1].TargetPort != "9091" {
		t.Fatalf("second port = %+v, want numeric target 9091", ports[1])
	}
}

func TestResolveServicePort(t *testing.T) {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api"}, Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "http", Port: 80},
		{Name: "metrics", Port: 9090},
	}}}

	if p, err := resolveServicePort(svc, "http"); err != nil || p.Port != 80 {
		t.Fatalf("resolve by name = (%d, %v), want 80/nil", p.Port, err)
	}
	if p, err := resolveServicePort(svc, "9090"); err != nil || p.Name != "metrics" {
		t.Fatalf("resolve by number = (%q, %v), want metrics/nil", p.Name, err)
	}
	if _, err := resolveServicePort(svc, ""); err == nil || !strings.Contains(err.Error(), "choose one") {
		t.Fatalf("resolve empty multi-port error = %v, want choose one", err)
	}
}

func TestServicePodPrefersReadyRunningPod(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "api"}},
	}
	cs := fake.NewSimpleClientset(
		svc,
		servicePod("z-not-ready", map[string]string{"app": "api"}, corev1.PodRunning, false),
		servicePod("a-ready", map[string]string{"app": "api"}, corev1.PodRunning, true),
		servicePod("other", map[string]string{"app": "other"}, corev1.PodRunning, true),
	)
	c := &Client{clientset: cs}

	pod, err := c.servicePod(context.Background(), svc)
	if err != nil {
		t.Fatalf("servicePod: %v", err)
	}
	if pod.Name != "a-ready" {
		t.Fatalf("servicePod = %q, want a-ready", pod.Name)
	}
}

func TestServiceBackendsListsSelectedPods(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
			Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "api"}},
		},
		servicePod("z-not-ready", map[string]string{"app": "api"}, corev1.PodRunning, false),
		servicePod("a-ready", map[string]string{"app": "api"}, corev1.PodRunning, true),
		servicePod("b-pending", map[string]string{"app": "api"}, corev1.PodPending, false),
	)
	c := &Client{clientset: cs}

	info, err := c.ServiceBackends(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("ServiceBackends: %v", err)
	}
	if info.Selector != "app=api" {
		t.Fatalf("selector = %q, want app=api", info.Selector)
	}
	if info.Ready != 1 || info.Running != 2 {
		t.Fatalf("counts = ready %d running %d, want 1/2", info.Ready, info.Running)
	}
	if len(info.Pods) != 3 {
		t.Fatalf("len(Pods) = %d, want 3", len(info.Pods))
	}
	if info.Pods[0].Name != "a-ready" || !info.Pods[0].Ready {
		t.Fatalf("first backend = %+v, want ready pod first", info.Pods[0])
	}
	if info.Pods[1].Name != "z-not-ready" {
		t.Fatalf("second backend = %+v, want z-not-ready", info.Pods[1])
	}
	if info.Pods[2].Name != "b-pending" {
		t.Fatalf("last backend = %+v, want b-pending", info.Pods[2])
	}
}

func TestServiceBackendsNoSelector(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}})
	c := &Client{clientset: cs}
	info, err := c.ServiceBackends(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("ServiceBackends: %v", err)
	}
	if info.Selector != "" || len(info.Pods) != 0 || info.Ready != 0 || info.Running != 0 {
		t.Fatalf("expected empty backend info, got %+v", info)
	}
}

func TestServiceTargetPortNumber(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api"}, Spec: corev1.PodSpec{Containers: []corev1.Container{{
		Name:  "web",
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
	}}}}

	got, err := serviceTargetPortNumber(pod, corev1.ServicePort{Port: 80, TargetPort: intstr.FromString("http"), Protocol: corev1.ProtocolTCP})
	if err != nil || got != 8080 {
		t.Fatalf("named target port = (%d, %v), want 8080/nil", got, err)
	}
	got, err = serviceTargetPortNumber(pod, corev1.ServicePort{Port: 5432})
	if err != nil || got != 5432 {
		t.Fatalf("default target port = (%d, %v), want 5432/nil", got, err)
	}
}

func servicePod(name string, labels map[string]string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels},
		Status: corev1.PodStatus{
			Phase:      phase,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readyStatus}},
		},
	}
}
