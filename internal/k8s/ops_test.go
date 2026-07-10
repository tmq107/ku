package k8s

import (
	"context"
	"strings"
	"testing"
)

func TestApplyRejectsNamespaceChange(t *testing.T) {
	res := ResourceInfo{Resource: "pods", Kind: "Pod", Namespaced: true}
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "changed namespace",
			yaml: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: api\n  namespace: other\n",
		},
		{
			name: "removed namespace",
			yaml: "apiVersion: v1\nkind: Pod\nmetadata:\n  name: api\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := (&Client{}).Apply(context.Background(), res, "default", "api", []byte(tt.yaml))
			if err == nil {
				t.Fatal("Apply succeeded after namespace change")
			}
			if !strings.Contains(err.Error(), "namespace") {
				t.Fatalf("Apply error = %q, want namespace rejection", err)
			}
		})
	}
}

