package k8s

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	sigsyaml "sigs.k8s.io/yaml"
)

// resourceClient returns the dynamic client scoped to the resource and, when
// applicable, namespace.
func (c *Client) resourceClient(res ResourceInfo, namespace string) dynamic.ResourceInterface {
	nr := c.dynamic.Resource(res.GVR())
	if res.Namespaced && namespace != "" {
		return nr.Namespace(namespace)
	}
	return nr
}

// GetYAML fetches a single object and renders it as YAML with managedFields
// stripped for readability.
func (c *Client) GetYAML(ctx context.Context, res ResourceInfo, namespace, name string) (string, error) {
	obj, err := c.resourceClient(res, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	b, err := sigsyaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Delete removes a single object.
func (c *Client) Delete(ctx context.Context, res ResourceInfo, namespace, name string) error {
	return c.resourceClient(res, namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// Apply replaces an object from edited YAML (kubectl edit semantics: an
// optimistic update guarded by resourceVersion). It rejects edits that change
// the object's kind or name, which would otherwise be sent to the wrong
// endpoint or target a different object.
func (c *Client) Apply(ctx context.Context, res ResourceInfo, namespace, name string, yamlBytes []byte) error {
	jsonBytes, err := sigsyaml.YAMLToJSON(yamlBytes)
	if err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(jsonBytes); err != nil {
		return fmt.Errorf("decode object: %w", err)
	}

	gvk := obj.GroupVersionKind()
	if gvk.Kind != res.Kind || gvk.Group != res.Group {
		return fmt.Errorf("edited kind %q does not match %s; change rejected", gvk.Kind, res.Kind)
	}
	if name != "" && obj.GetName() != name {
		return fmt.Errorf("edited name %q does not match %q; change rejected", obj.GetName(), name)
	}

	ns := namespace
	if res.Namespaced && obj.GetNamespace() != "" {
		ns = obj.GetNamespace()
	}
	_, err = c.resourceClient(res, ns).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

// Scale sets spec.replicas via a JSON merge patch on the resource. This works
// for the built-in workloads that expose spec.replicas (deployments,
// statefulsets, replicasets, replicationcontrollers).
func (c *Client) Scale(ctx context.Context, res ResourceInfo, namespace, name string, replicas int) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	_, err := c.resourceClient(res, namespace).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// Namespaces lists namespace names, sorted.
func (c *Client) Namespaces(ctx context.Context) ([]string, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].Name)
	}
	sort.Strings(names)
	return names, nil
}
