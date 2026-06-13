package ui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/bjarneo/kli/internal/k8s"
)

const (
	maxColWidth = 60
	colFloor    = 4
	nameFloor   = 16
)

// tableView renders a resource list with live filtering, column sorting, and
// per-cell color. It owns its own cursor/scroll so it can color cells (which
// the bubbles table cannot do safely).
type tableView struct {
	th Theme

	cols    []k8s.Column // all columns from the server
	allRows []k8s.Row    // unfiltered
	rows    []k8s.Row    // currently displayed (filtered + sorted)
	vis     []int        // visible column indices (parallel to widths)
	widths  []int        // fitted widths for visible columns

	filtering bool
	filter    textinput.Model
	showWide  bool

	sortCol  int // index into cols, -1 for default (server) order
	sortDesc bool

	cursor int
	offset int // first visible row
	width  int
	height int
}

func newTableView(th Theme) tableView {
	fi := textinput.New()
	fi.Prompt = "/"
	fi.Placeholder = "filter"
	fi.Cursor.SetMode(cursor.CursorStatic)
	return tableView{th: th, filter: fi, sortCol: -1}
}

func (v *tableView) setSize(w, h int) {
	v.width = w
	if h < 1 {
		h = 1
	}
	v.height = h
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

func (v *tableView) filterActive() bool  { return v.filter.Value() != "" }
func (v *tableView) filterValue() string { return v.filter.Value() }

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

func (v *tableView) selected() (k8s.Row, bool) {
	if v.cursor < 0 || v.cursor >= len(v.rows) {
		return k8s.Row{}, false
	}
	return v.rows[v.cursor], true
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

// rebuild recomputes the filtered+sorted rows and fitted widths, keeping the
// cursor in range and visible.
func (v *tableView) rebuild() {
	v.vis = v.visibleCols()

	// Filter (an empty filter keeps all rows in their original order).
	v.rows = fuzzyRank(v.allRows, v.filter.Value(), func(r k8s.Row) string {
		return strings.Join(r.Cells, " ")
	})

	// Optional explicit column sort, layered on top of the filter.
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

	v.widths = v.fitWidths(v.vis)

	if v.cursor >= len(v.rows) {
		v.cursor = len(v.rows) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	v.ensureVisible()
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

	// Budget excludes the 2 cells of horizontal padding per column.
	budget := v.width - 2*len(vis)
	if budget < len(vis) {
		return widths
	}
	for sum(widths) > budget {
		// Shrink the widest column above its floor; protect the name column.
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
				weight -= 1000
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

// --- navigation -------------------------------------------------------------

func (v *tableView) visibleRows() int {
	if v.height > 1 {
		return v.height - 1 // one line for the header
	}
	return 0
}

func (v *tableView) moveCursor(d int) { v.setCursor(v.cursor + d) }

func (v *tableView) setCursor(i int) {
	if len(v.rows) == 0 {
		v.cursor, v.offset = 0, 0
		return
	}
	v.cursor = clamp(i, 0, len(v.rows)-1)
	v.ensureVisible()
}

func (v *tableView) ensureVisible() {
	vr := v.visibleRows()
	if vr <= 0 {
		v.offset = 0
		return
	}
	if v.cursor < v.offset {
		v.offset = v.cursor
	}
	if v.cursor >= v.offset+vr {
		v.offset = v.cursor - vr + 1
	}
	maxOff := len(v.rows) - vr
	if maxOff < 0 {
		maxOff = 0
	}
	v.offset = clamp(v.offset, 0, maxOff)
}

// Update routes input to the filter box, or moves the cursor.
func (v tableView) Update(msg tea.Msg) (tableView, tea.Cmd) {
	if v.filtering {
		prev := v.filter.Value()
		var cmd tea.Cmd
		v.filter, cmd = v.filter.Update(msg)
		if v.filter.Value() != prev {
			v.cursor, v.offset = 0, 0
			v.rebuild()
		}
		return v, cmd
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return v, nil
	}
	vr := v.visibleRows()
	switch k.String() {
	case "up", "k":
		v.moveCursor(-1)
	case "down", "j":
		v.moveCursor(1)
	case "pgup":
		v.moveCursor(-vr)
	case "pgdown":
		v.moveCursor(vr)
	case "ctrl+u":
		v.moveCursor(-vr / 2)
	case "ctrl+d":
		v.moveCursor(vr / 2)
	case "g", "home":
		v.setCursor(0)
	case "G", "end":
		v.setCursor(len(v.rows) - 1)
	}
	return v, nil
}

// --- rendering --------------------------------------------------------------

func (v tableView) View() string {
	lines := make([]string, 0, v.height)
	lines = append(lines, v.headerLine())
	vr := v.visibleRows()
	for i := 0; i < vr; i++ {
		idx := v.offset + i
		if idx >= len(v.rows) {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, v.renderRow(v.rows[idx], idx == v.cursor))
	}
	return strings.Join(lines, "\n")
}

func (v tableView) headerLine() string {
	var b strings.Builder
	for n, ci := range v.vis {
		title := strings.ToUpper(v.cols[ci].Name)
		if ci == v.sortCol {
			if v.sortDesc {
				title += " ▼"
			} else {
				title += " ▲"
			}
		}
		b.WriteString(" " + v.th.HeaderVal.Render(cellFit(title, v.widths[n])) + " ")
	}
	return b.String()
}

func (v tableView) renderRow(row k8s.Row, selected bool) string {
	var b strings.Builder
	for n, ci := range v.vis {
		val := cell(row.Cells, ci)
		txt := cellFit(val, v.widths[n])
		if selected {
			b.WriteString(" " + txt + " ") // colored later via the whole-row highlight
		} else {
			b.WriteString(" " + styleCell(v.th, v.cols[ci].Name, val).Render(txt) + " ")
		}
	}
	if selected {
		return v.th.SelItemSel.Render(b.String())
	}
	return b.String()
}

// cellFit truncates s to w display columns (with an ellipsis when cut) and pads
// it on the right to exactly w.
func cellFit(s string, w int) string {
	s = truncate(s, w)
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}
