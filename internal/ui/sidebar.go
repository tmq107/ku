package ui

import (
	"strings"

	"github.com/bjarneo/kli/internal/k8s"
)

// navCatalog is the curated, lazygit-style quick list. Entries that the cluster
// does not expose are dropped, and empty sections are hidden.
var navCatalog = []struct {
	section string
	items   []struct{ label, query string }
}{
	{"Workloads", []struct{ label, query string }{
		{"Pods", "pods"},
		{"Deployments", "deployments"},
		{"StatefulSets", "statefulsets"},
		{"DaemonSets", "daemonsets"},
		{"ReplicaSets", "replicasets"},
		{"Jobs", "jobs"},
		{"CronJobs", "cronjobs"},
	}},
	{"Network", []struct{ label, query string }{
		{"Services", "services"},
		{"Ingresses", "ingresses"},
		{"Endpoints", "endpoints"},
	}},
	{"Config", []struct{ label, query string }{
		{"ConfigMaps", "configmaps"},
		{"Secrets", "secrets"},
		{"ServiceAccounts", "serviceaccounts"},
	}},
	{"Storage", []struct{ label, query string }{
		{"PVCs", "persistentvolumeclaims"},
		{"PVs", "persistentvolumes"},
		{"StorageClasses", "storageclasses"},
	}},
	{"Cluster", []struct{ label, query string }{
		{"Nodes", "nodes"},
		{"Namespaces", "namespaces"},
		{"Events", "events"},
	}},
}

type navEntry struct {
	header bool
	label  string
	res    k8s.ResourceInfo
	key    string
}

// sidebar is the left navigation pane listing common resource kinds.
type sidebar struct {
	th         Theme
	entries    []navEntry
	selectable []int // indices into entries that are real items
	cursor     int   // index into selectable
	width      int
	height     int
}

func newSidebar(th Theme, reg *k8s.Registry) sidebar {
	s := sidebar{th: th}
	for _, sec := range navCatalog {
		var items []navEntry
		for _, it := range sec.items {
			ri, ok := reg.Resolve(it.query)
			if !ok {
				continue
			}
			items = append(items, navEntry{label: it.label, res: ri, key: ri.Key()})
		}
		if len(items) == 0 {
			continue
		}
		s.entries = append(s.entries, navEntry{header: true, label: sec.section})
		for _, it := range items {
			s.selectable = append(s.selectable, len(s.entries))
			s.entries = append(s.entries, it)
		}
	}
	return s
}

func (s *sidebar) setSize(w, h int) {
	s.width = w
	s.height = h
}

func (s *sidebar) moveUp() {
	if s.cursor > 0 {
		s.cursor--
	}
}

func (s *sidebar) moveDown() {
	if s.cursor < len(s.selectable)-1 {
		s.cursor++
	}
}

func (s *sidebar) move(delta int) {
	s.cursor = clamp(s.cursor+delta, 0, len(s.selectable)-1)
}

func (s *sidebar) moveTop() { s.cursor = 0 }

func (s *sidebar) moveBottom() {
	if len(s.selectable) > 0 {
		s.cursor = len(s.selectable) - 1
	}
}

func (s *sidebar) current() (k8s.ResourceInfo, bool) {
	if len(s.selectable) == 0 {
		return k8s.ResourceInfo{}, false
	}
	return s.entries[s.selectable[s.cursor]].res, true
}

// syncTo moves the cursor to the entry matching key, if present, so the
// highlight follows resource switches made elsewhere (palette, jump).
func (s *sidebar) syncTo(key string) {
	for i, ei := range s.selectable {
		if s.entries[ei].key == key {
			s.cursor = i
			return
		}
	}
}

func (s sidebar) View(activeKey string, focused bool) string {
	th := s.th
	curEntry := -1
	if len(s.selectable) > 0 {
		curEntry = s.selectable[s.cursor]
	}

	// Scroll so the cursor stays visible.
	offset := 0
	if s.height > 0 && curEntry >= s.height {
		offset = curEntry - s.height + 1
	}

	var lines []string
	for i := offset; i < len(s.entries) && len(lines) < s.height; i++ {
		e := s.entries[i]
		if e.header {
			lines = append(lines, th.NavSection.Render(truncate(e.label, s.width)))
			continue
		}
		marker := "  "
		label := e.label
		if e.key == activeKey {
			marker = th.HeaderVal.Render("▸ ")
			label = th.HeaderVal.Render(label)
		} else {
			label = th.SelItem.Render(label)
		}
		line := marker + label
		if focused && i == curEntry {
			line = th.SelItemSel.Width(s.width).Render("  " + truncate(e.label, s.width-2))
		}
		lines = append(lines, line)
	}
	for len(lines) < s.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
