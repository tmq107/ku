package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func itoa(n int) string { return strconv.Itoa(n) }

// truncate shortens s to at most w display columns, adding an ellipsis when
// cut. It is display-width and ANSI aware (the same engine used everywhere else
// in the package).
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// spread lays out left and right text on one line of the given width, padding
// the gap between them (minimum one space).
func spread(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rw := lipgloss.Width(right)
	if rw >= width {
		return ansi.Truncate(right, width, "")
	}
	if lipgloss.Width(left)+1+rw > width {
		budget := width - rw - 1
		if budget <= 0 {
			left = ""
		} else {
			left = ansi.Truncate(left, budget, "…")
		}
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

const (
	// paneGap separates the sidebar from the active content pane.
	paneGap      = 1
	panePaddingY = 0
	panePaddingX = 1
)

func paneStyleWidth(outer int) int {
	return paneInnerSize(outer, 2)
}

func paneStyleHeight(outer int) int {
	return paneInnerSize(outer, 2)
}

func paneContentWidth(outer int) int {
	return paneInnerSize(outer, 2+2*panePaddingX)
}

func paneContentHeight(outer int) int {
	return paneInnerSize(outer, 2+2*panePaddingY)
}

func paneInnerSize(outer, frame int) int {
	if n := outer - frame; n > 0 {
		return n
	}
	return 1
}
