package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// highlightYAML applies lightweight, theme-aware syntax highlighting to YAML:
// keys, comments, and scalar values (booleans, numbers, quoted strings). It is
// line-based, which is enough for the single-object manifests kli shows.
func highlightYAML(s string, th Theme) string {
	keyStyle := lipgloss.NewStyle().Foreground(th.P.Accent)
	boolStyle := lipgloss.NewStyle().Foreground(th.P.Accent2)
	valStyle := lipgloss.NewStyle().Foreground(th.P.Fg)

	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = highlightYAMLLine(line, th, keyStyle, boolStyle, valStyle)
	}
	return strings.Join(lines, "\n")
}

func highlightYAMLLine(line string, th Theme, keyStyle, boolStyle, valStyle lipgloss.Style) string {
	indentLen := len(line) - len(strings.TrimLeft(line, " "))
	indent := line[:indentLen]
	rest := line[indentLen:]
	if rest == "" {
		return line
	}
	if strings.HasPrefix(rest, "#") {
		return indent + th.Dim.Render(rest)
	}

	prefix := ""
	switch {
	case rest == "-":
		return indent + th.Dim.Render("-")
	case strings.HasPrefix(rest, "- "):
		prefix = th.Dim.Render("- ")
		rest = rest[2:]
	}

	if ci := keyColon(rest); ci >= 0 {
		key := rest[:ci]
		after := rest[ci+1:] // value part, may start with a space
		out := indent + prefix + keyStyle.Render(key) + ":"
		if strings.TrimSpace(after) == "" {
			return out + after
		}
		lead := after[:len(after)-len(strings.TrimLeft(after, " "))]
		return out + lead + colorScalar(strings.TrimLeft(after, " "), th, boolStyle, valStyle)
	}
	return indent + prefix + colorScalar(rest, th, boolStyle, valStyle)
}

// keyColon returns the index of the colon separating a YAML key from its value,
// or -1 if the line is not a mapping entry.
func keyColon(s string) int {
	if i := strings.Index(s, ": "); i >= 0 {
		return i
	}
	if strings.HasSuffix(s, ":") {
		return len(s) - 1
	}
	return -1
}

func colorScalar(v string, th Theme, boolStyle, valStyle lipgloss.Style) string {
	switch v {
	case "true", "false", "null", "~":
		return boolStyle.Render(v)
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return th.Warn.Render(v)
	}
	if strings.HasPrefix(v, `"`) || strings.HasPrefix(v, "'") {
		return th.Good.Render(v)
	}
	if _, ok := th.P.Fg.(lipgloss.NoColor); ok {
		return v // terminal default (ANSI theme)
	}
	return valStyle.Render(v)
}
