package ui

import "github.com/charmbracelet/lipgloss"

// confirmView is a small yes/no modal used for destructive actions.
type confirmView struct {
	th      Theme
	title   string
	message string
	danger  bool
}

func (c confirmView) View(width, height int) string {
	titleStyle := c.th.ModalTitle
	border := c.th.ModalBorder
	if c.danger {
		titleStyle = lipgloss.NewStyle().Foreground(c.th.P.Bad).Bold(true)
		border = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c.th.P.Bad).Padding(0, 1)
	}

	body := titleStyle.Render(c.title) + "\n\n" +
		c.th.SelItem.Render(c.message) + "\n\n" +
		c.th.FooterKey.Render("y") + c.th.FooterDesc.Render(" confirm") + "    " +
		c.th.FooterKey.Render("n") + c.th.FooterDesc.Render("/") +
		c.th.FooterKey.Render("esc") + c.th.FooterDesc.Render(" cancel")

	boxW := lipgloss.Width(c.message) + 6
	if boxW < 40 {
		boxW = 40
	}
	if boxW > width-4 {
		boxW = width - 4
	}
	if boxW < 8 {
		boxW = 8
	}
	box := border.Width(boxW).Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
