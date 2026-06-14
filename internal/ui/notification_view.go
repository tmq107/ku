package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	notificationMinWidth  = 24
	notificationMinHeight = 6
	notificationMaxWidth  = 56
	notificationMaxLines  = 4
)

func (a App) notificationVisible() bool {
	return a.status != "" && a.width >= notificationMinWidth+4 && a.height >= notificationMinHeight
}

func (a App) renderNotification(frame string) string {
	if !a.notificationVisible() {
		return frame
	}

	y := 1
	maxH := a.height - 2
	if a.height >= 8 {
		y = 2
		maxH = a.height - 3
	}

	box := notificationBox(a.theme, a.status, a.statusErr, a.width-4, maxH)
	if box == "" {
		return frame
	}

	x := a.width - lipgloss.Width(box) - 2
	if x < 0 {
		x = 0
	}
	return overlayBlock(frame, box, x, y, a.width, a.height)
}

func notificationBox(th Theme, text string, isErr bool, maxWidth, maxHeight int) string {
	if maxWidth < notificationMinWidth || maxHeight < 3 {
		return ""
	}
	bodyText := strings.TrimSpace(text)
	title, color := "NOTICE", th.P.Good
	if isErr {
		title, color = "ERROR", th.P.Bad
	}

	maxContentW := clamp(maxWidth-4, notificationMinWidth-4, notificationMaxWidth-4)
	contentW := clamp(ansi.StringWidth(bodyText), notificationMinWidth-4, maxContentW)
	contentLines := clamp(maxHeight-2, 1, notificationMaxLines+1)

	lines := []string{lipgloss.NewStyle().Foreground(color).Bold(true).Render(title)}
	if contentLines > 1 {
		body := ansi.Wordwrap(bodyText, contentW, " ")
		for _, line := range strings.Split(body, "\n") {
			if len(lines) >= contentLines {
				break
			}
			lines = append(lines, ansi.Truncate(line, contentW, "…"))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(color).
		Foreground(th.P.Fg).
		Padding(0, 1).
		Width(contentW + 2).
		Render(strings.Join(lines, "\n"))
}

func overlayBlock(base, overlay string, x, y, width, height int) string {
	baseLines := strings.Split(clampBlock(base, width, height), "\n")

	for i, overlayLine := range strings.Split(overlay, "\n") {
		row := y + i
		if row >= height {
			continue
		}

		line := overlayLine
		if x >= width {
			continue
		}
		if w := lipgloss.Width(line); w > width-x {
			line = ansi.Truncate(line, width-x, "")
		}
		lineW := lipgloss.Width(line)
		if lineW == 0 {
			continue
		}

		left := ansi.Cut(baseLines[row], 0, x)
		if pad := x - lipgloss.Width(left); pad > 0 {
			left += strings.Repeat(" ", pad)
		}

		right := ""
		if start := x + lineW; start < width {
			right = ansi.Cut(baseLines[row], start, width)
		}
		baseLines[row] = left + line + right
	}

	return strings.Join(baseLines, "\n")
}
