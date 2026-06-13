package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bjarneo/kli/internal/k8s"
)

const (
	maxColWidth = 60
	colFloor    = 4
	nameFloor   = 16
)

// tableView renders a resource list with live filtering and column fitting.
type tableView struct {
	th Theme

	tbl     table.Model
	cols    []k8s.Column // all columns from the server
	allRows []k8s.Row    // unfiltered
	rows    []k8s.Row    // currently displayed (parallel to tbl rows)

	filtering bool
	filter    textinput.Model
	showWide  bool

	sortCol  int // index into cols, -1 for default (server) order
	sortDesc bool

	width int
}

func newTableView(th Theme) tableView {
	km := table.DefaultKeyMap()
	km.LineUp = key.NewBinding(key.WithKeys("up", "k"))
	km.LineDown = key.NewBinding(key.WithKeys("down", "j"))
	km.PageUp = key.NewBinding(key.WithKeys("pgup"))
	km.PageDown = key.NewBinding(key.WithKeys("pgdown"))
	km.HalfPageUp = key.NewBinding(key.WithKeys("ctrl+u"))
	km.HalfPageDown = key.NewBinding(key.WithKeys("ctrl+d"))
	km.GotoTop = key.NewBinding(key.WithKeys("g", "home"))
	km.GotoBottom = key.NewBinding(key.WithKeys("G", "end"))

	tbl := table.New(table.WithFocused(true), table.WithKeyMap(km))
	tbl.SetStyles(table.Styles{
		Header:   th.TableHeader,
		Cell:     th.TableCell,
		Selected: th.TableSelected,
	})

	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "filter"
	fi.Cursor.SetMode(cursor.CursorStatic)

	return tableView{th: th, tbl: tbl, filter: fi, sortCol: -1}
}

// setSort sorts by the given column index. Re-selecting the active column flips
// the direction; idx < 0 restores the server's default order. Numeric columns
// default to descending (most interesting first).
func (v *tableView) setSort(idx int) {
	switch {
	case idx < 0:
		v.sortCol = -1
	case idx == v.sortCol:
		v.sortDesc = !v.sortDesc
	default:
		v.sortCol = idx
		v.sortDesc = sortDescByDefault(v.colName(idx))
	}
	v.rebuild()
}

func (v *tableView) resetSort() {
	v.sortCol = -1
	v.sortDesc = false
}

func (v *tableView) colName(i int) string {
	if i >= 0 && i < len(v.cols) {
		return v.cols[i].Name
	}
	return ""
}

func sortDescByDefault(name string) bool {
	n := strings.ToLower(name)
	switch {
	case n == "age", n == "restarts", strings.Contains(n, "cpu"), strings.Contains(n, "mem"), strings.HasSuffix(n, "%"):
		return true
	}
	return false
}

func (v *tableView) setSize(w, h int) {
	v.width = w
	if h < 1 {
		h = 1
	}
	v.tbl.SetHeight(h)
	v.tbl.SetWidth(w)
	if fw := w - 8; fw > 4 {
		v.filter.Width = fw // bound the filter input so it can't overflow
	}
	v.rebuild()
}

func (v *tableView) setData(t *k8s.Table) {
	if t == nil {
		v.cols = nil
		v.allRows = nil
	} else {
		v.cols = t.Columns
		v.allRows = t.Rows
	}
	v.rebuild()
}

func (v *tableView) toggleWide() {
	v.showWide = !v.showWide
	v.rebuild()
}

func (v *tableView) startFilter() {
	v.filtering = true
	v.filter.Focus()
}

// stopFilter exits filter mode. If clear is true the filter text is dropped and
// the full list restored.
func (v *tableView) stopFilter(clear bool) {
	v.filtering = false
	v.filter.Blur()
	if clear {
		v.filter.SetValue("")
		v.rebuild()
	}
}

// filterActive reports whether a filter is narrowing the list, whether or not
// the filter input is currently focused.
func (v *tableView) filterActive() bool {
	return v.filter.Value() != ""
}

func (v *tableView) filterValue() string {
	return v.filter.Value()
}

func (v *tableView) selected() (k8s.Row, bool) {
	i := v.tbl.Cursor()
	if i < 0 || i >= len(v.rows) {
		return k8s.Row{}, false
	}
	return v.rows[i], true
}

func (v *tableView) count() int { return len(v.rows) }

// visibleCols returns the indices of columns shown given the wide toggle.
func (v *tableView) visibleCols() []int {
	idx := make([]int, 0, len(v.cols))
	for i, c := range v.cols {
		if c.Priority == 0 || v.showWide {
			idx = append(idx, i)
		}
	}
	return idx
}

func cell(cells []string, i int) string {
	if i < 0 || i >= len(cells) {
		return ""
	}
	return cells[i]
}

// rebuild recomputes the filtered rows, fits column widths to the terminal and
// pushes them into the underlying table, preserving the cursor position.
func (v *tableView) rebuild() {
	vis := v.visibleCols()

	// Filter rows (an empty filter keeps all rows in their original order).
	v.rows = fuzzyRank(v.allRows, v.filter.Value(), func(r k8s.Row) string {
		return strings.Join(r.Cells, " ")
	})

	// Apply an explicit column sort on top of the filter, if set.
	if v.sortCol >= 0 && v.sortCol < len(v.cols) {
		name := v.cols[v.sortCol].Name
		less := func(i, j int) bool {
			return cellLess(name, cell(v.rows[i].Cells, v.sortCol), cell(v.rows[j].Cells, v.sortCol))
		}
		if v.sortDesc {
			sort.SliceStable(v.rows, func(i, j int) bool { return less(j, i) })
		} else {
			sort.SliceStable(v.rows, less)
		}
	}

	// Fit widths.
	widths := v.fitWidths(vis)
	cols := make([]table.Column, len(vis))
	for n, ci := range vis {
		title := strings.ToUpper(v.cols[ci].Name)
		if ci == v.sortCol {
			if v.sortDesc {
				title += " ▼"
			} else {
				title += " ▲"
			}
		}
		cols[n] = table.Column{Title: title, Width: widths[n]}
	}

	trows := make([]table.Row, len(v.rows))
	for ri, r := range v.rows {
		tr := make(table.Row, len(vis))
		for n, ci := range vis {
			tr[n] = cell(r.Cells, ci)
		}
		trows[ri] = tr
	}

	cur := v.tbl.Cursor()
	// Clear rows before swapping columns: bubbles re-renders existing rows
	// against the new column set inside SetColumns, which panics if an old row
	// has more cells than the new column count (e.g. toggling wide off).
	v.tbl.SetRows(nil)
	v.tbl.SetColumns(cols)
	v.tbl.SetRows(trows)
	if cur >= len(trows) {
		cur = len(trows) - 1
	}
	if cur < 0 {
		cur = 0
	}
	v.tbl.SetCursor(cur)
}

func (v *tableView) fitWidths(vis []int) []int {
	widths := make([]int, len(vis))
	for n, ci := range vis {
		w := len(v.cols[ci].Name)
		if ci == v.sortCol {
			w += 2 // room for the " ▲" / " ▼" indicator
		}
		for _, r := range v.allRows {
			if l := len(cell(r.Cells, ci)); l > w {
				w = l
			}
		}
		if w > maxColWidth {
			w = maxColWidth
		}
		floor := colFloor
		if strings.EqualFold(v.cols[ci].Name, "name") {
			floor = nameFloor
		}
		if w < floor {
			w = floor
		}
		widths[n] = w
	}

	// Budget excludes the 2 cells of horizontal padding bubbles adds per column.
	budget := v.width - 2*len(vis)
	if budget < len(vis) {
		return widths
	}

	for sum(widths) > budget {
		// Shrink the widest column that is still above its floor; protect the
		// name column until everything else bottoms out.
		target, best := -1, -1
		for n, ci := range vis {
			floor := colFloor
			isName := strings.EqualFold(v.cols[ci].Name, "name")
			if isName {
				floor = nameFloor
			}
			if widths[n] <= floor {
				continue
			}
			weight := widths[n]
			if isName {
				weight -= 1000 // deprioritize shrinking name
			}
			if weight > best {
				best = weight
				target = n
			}
		}
		if target < 0 {
			break
		}
		widths[target]--
	}
	return widths
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

// Update routes input either to the filter box or to table navigation.
func (v tableView) Update(msg tea.Msg) (tableView, tea.Cmd) {
	if v.filtering {
		prev := v.filter.Value()
		var cmd tea.Cmd
		v.filter, cmd = v.filter.Update(msg)
		if v.filter.Value() != prev {
			v.rebuild()
		}
		return v, cmd
	}
	var cmd tea.Cmd
	v.tbl, cmd = v.tbl.Update(msg)
	return v, cmd
}

func (v tableView) View() string {
	return v.tbl.View()
}
