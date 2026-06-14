package k8s

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
)

func TestDeploymentReadyUsesDesiredReplicas(t *testing.T) {
	replicas := func(n int32) *int32 { return &n }
	tests := []struct {
		name    string
		desired *int32
		ready   int32
		want    bool
	}{
		{name: "stale zero status with desired replicas", desired: replicas(3), ready: 0, want: false},
		{name: "fully ready", desired: replicas(3), ready: 3, want: true},
		{name: "partially ready", desired: replicas(3), ready: 2, want: false},
		{name: "scaled to zero", desired: replicas(0), ready: 0, want: true},
		{name: "nil desired defaults to one", desired: nil, ready: 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deploymentReady(&appsv1.Deployment{
				Spec:   appsv1.DeploymentSpec{Replicas: tt.desired},
				Status: appsv1.DeploymentStatus{ReadyReplicas: tt.ready},
			})
			if got != tt.want {
				t.Fatalf("deploymentReady() = %t, want %t", got, tt.want)
			}
		})
	}
}
