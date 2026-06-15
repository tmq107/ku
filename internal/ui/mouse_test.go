package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTableMouseHitTesting(t *testing.T) {
	th := PickTheme("ansi")
	tv := newTableView(th)
	tv.setSize(80, 10)
	tv.setData(fakeTable())

	if ci, ok := tv.colAt(0); !ok || ci != 0 {
		t.Fatalf("colAt(0) = %d, %t; want 0, true", ci, ok)
	}
	if _, ok := tv.rowAt(0); ok {
		t.Fatal("rowAt(0) hit header as row")
	}
	if row, ok := tv.rowAt(1); !ok || row != 0 {
		t.Fatalf("rowAt(1) = %d, %t; want 0, true", row, ok)
	}
	if row, ok := tv.rowAt(2); !ok || row != 1 {
		t.Fatalf("rowAt(2) = %d, %t; want 1, true", row, ok)
	}
}

func TestAppMouseSelectsTableRowsAndSortsHeaders(t *testing.T) {
	th := PickTheme("ansi")
	app := App{theme: th, width: 80, height: 24, screen: screenTable, focus: focusMain}
	app.table = newTableView(th)
	app.relayout()
	app.table.setData(fakeTable())

	m, _ := app.Update(tea.MouseClickMsg{X: 2, Y: 4, Button: tea.MouseLeft})
	app = m.(App)
	if app.table.cursor != 1 {
		t.Fatalf("mouse row click selected cursor %d; want 1", app.table.cursor)
	}

	m, _ = app.Update(tea.MouseClickMsg{X: 2, Y: 2, Button: tea.MouseLeft})
	app = m.(App)
	if app.table.sortCol != 0 {
		t.Fatalf("mouse header click sortCol = %d; want 0", app.table.sortCol)
	}

	m, _ = app.Update(tea.MouseWheelMsg{X: 2, Y: 4, Button: tea.MouseWheelUp})
	app = m.(App)
	if app.table.cursor != 0 {
		t.Fatalf("mouse wheel selected cursor %d; want 0", app.table.cursor)
	}
}

func TestSidebarMouseSelectsEntries(t *testing.T) {
	s := sidebar{
		height:     4,
		selectable: []int{0, 2, 3},
		entries: []navEntry{
			{overview: true, label: "Overview", key: overviewKey},
			{header: true, label: "Workloads"},
			{label: "Pods", key: "pods"},
			{label: "Services", key: "services"},
		},
	}

	if _, ok := s.selectAt(1); ok {
		t.Fatal("selectAt hit a section header")
	}
	e, ok := s.selectAt(2)
	if !ok || e.key != "pods" || s.cursor != 1 {
		t.Fatalf("selectAt(2) = %+v, %t cursor=%d; want pods, true cursor=1", e, ok, s.cursor)
	}
}
