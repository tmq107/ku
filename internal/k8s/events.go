package k8s

import (
	"context"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodEvents returns events for a specific pod, sorted chronologically
// oldest-first (like kubectl describe). The list is capped at the 50 newest.
func (c *Client) PodEvents(ctx context.Context, namespace, name string) ([]EventLine, error) {
	list, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + name,
	})
	if err != nil {
		return nil, err
	}

	// Filter by involvedObject in code as well, since the fake clientset
	// ignores field selectors and real clusters may return related events.
	var items []int
	for i := range list.Items {
		if list.Items[i].InvolvedObject.Name == name {
			items = append(items, i)
		}
	}

	// Sort chronologically oldest-first (opposite of recentWarnings).
	sort.Slice(items, func(a, b int) bool {
		return eventTime(&list.Items[items[a]]).Before(eventTime(&list.Items[items[b]]))
	})

	// Cap at 50 newest events when the list is large.
	if len(items) > 50 {
		items = items[len(items)-50:]
	}

	result := make([]EventLine, 0, len(items))
	for _, idx := range items {
		e := &list.Items[idx]
		result = append(result, EventLine{
			Age:     ageString(eventTime(e)),
			Type:    e.Type,
			Reason:  e.Reason,
			Message: e.Message,
			Count:   int(e.Count),
		})
	}
	return result, nil
}
