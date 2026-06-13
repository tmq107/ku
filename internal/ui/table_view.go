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

	width, height int
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

	return tableView{th: th, tbl: tbl, filter: fi}
}

func (v *tableView) setSize(w, h int) {
	v.width = w
	v.height = h
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

// clearFilter drops an applied filter and restores the full list.
func (v *tableView) clearFilter() {
	v.filtering = false
	v.filter.Blur()
	v.filter.SetValue("")
	v.rebuild()
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

	// Filter rows.
	q := v.filter.Value()
	v.rows = v.rows[:0]
	if q == "" {
		v.rows = append(v.rows, v.allRows...)
	} else {
		type scored struct {
			row   k8s.Row
			score int
		}
		var matches []scored
		for _, r := range v.allRows {
			hay := strings.Join(r.Cells, " ")
			if s, ok := fuzzyScore(q, hay); ok {
				matches = append(matches, scored{r, s})
			}
		}
		sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
		for _, m := range matches {
			v.rows = append(v.rows, m.row)
		}
	}

	// Fit widths.
	widths := v.fitWidths(vis)
	cols := make([]table.Column, len(vis))
	for n, ci := range vis {
		cols[n] = table.Column{Title: strings.ToUpper(v.cols[ci].Name), Width: widths[n]}
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
