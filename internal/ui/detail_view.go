package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// detailView shows a single object's YAML in a scrollable viewport.
type detailView struct {
	th    Theme
	vp    viewport.Model
	title string
	ready bool
}

func newDetailView(th Theme) detailView {
	return detailView{th: th, vp: viewport.New(0, 0)}
}

func (d *detailView) setSize(w, h int) {
	if h < 1 {
		h = 1
	}
	d.vp.Width = w
	d.vp.Height = h - 1 // leave a row for the title
}

func (d *detailView) setContent(title, body string) {
	d.title = title
	d.vp.SetContent(body)
	d.vp.GotoTop()
	d.ready = true
}

func (d detailView) Update(msg tea.Msg) (detailView, tea.Cmd) {
	var cmd tea.Cmd
	d.vp, cmd = d.vp.Update(msg)
	return d, cmd
}

func (d detailView) View() string {
	pct := d.th.Dim.Render(scrollPercent(d.vp.ScrollPercent()))
	title := d.th.ModalTitle.Render(d.title)
	gap := d.vp.Width - lipgloss.Width(title) - lipgloss.Width(pct)
	if gap < 1 {
		gap = 1
	}
	header := title + strings.Repeat(" ", gap) + pct
	return header + "\n" + d.vp.View()
}

func scrollPercent(f float64) string {
	p := int(f * 100)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return itoa(p) + "%"
}
