package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpView renders all keybindings grouped into columns.
type helpView struct {
	th   Theme
	keys keyMap
}

func newHelpView(th Theme, keys keyMap) helpView {
	return helpView{th: th, keys: keys}
}

func (h helpView) View(width, height int) string {
	groups := h.keys.groups()
	colW := 26
	if w := width - 6; w < colW {
		colW = w
	}
	if colW < 12 {
		colW = 12
	}

	blocks := make([]string, len(groups))
	for i, g := range groups {
		var b strings.Builder
		b.WriteString(h.th.HeaderVal.Render(g.title))
		b.WriteString("\n")
		for _, k := range g.keys {
			hk := k.Help()
			b.WriteString(h.th.FooterKey.Render(padRight(hk.Key, 8)))
			b.WriteString(" ")
			b.WriteString(h.th.FooterDesc.Render(hk.Desc))
			b.WriteString("\n")
		}
		blocks[i] = lipgloss.NewStyle().Width(colW).Render(strings.TrimRight(b.String(), "\n"))
	}

	perRow := 3
	switch {
	case width < 62:
		perRow = 1
	case width < 92:
		perRow = 2
	}

	var rows []string
	for i := 0; i < len(blocks); i += perRow {
		end := i + perRow
		if end > len(blocks) {
			end = len(blocks)
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, blocks[i:end]...))
	}
	grid := lipgloss.JoinVertical(lipgloss.Left, rows...)

	title := h.th.ModalTitle.Render("Keybindings")
	hint := h.th.Dim.Render("  ? or esc to close")
	content := title + hint + "\n\n" + grid

	box := h.th.ModalBorder.Render(content)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
