package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// styleCell picks a style for a cell based on its column and value, so lists
// read at a glance: names stay bright, status/ready/restarts/usage are colored
// by health, and metadata (age, ip, node) is dimmed.
func styleCell(th Theme, col, val string) lipgloss.Style {
	lc := strings.ToLower(strings.TrimSpace(col))
	switch {
	case lc == "status" || lc == "state" || lc == "phase" || lc == "reason":
		return statusStyle(th, val)
	case lc == "ready":
		return readyStyle(th, val)
	case lc == "restarts":
		if n, ok := leadingNum(val); ok && n > 0 {
			if n >= 5 {
				return th.Bad
			}
			return th.Warn
		}
		return th.Dim
	case strings.HasSuffix(lc, "%"):
		if n, ok := leadingNum(val); ok {
			switch {
			case n >= 85:
				return th.Bad
			case n >= 70:
				return th.Warn
			}
		}
		return th.Dim
	case lc == "age" || lc == "ip" || lc == "node" || lc == "nominated node" ||
		lc == "readiness gates" || lc == "cpu" || lc == "mem" || lc == "memory":
		return th.Dim
	default:
		return th.Cell
	}
}

func statusStyle(th Theme, v string) lipgloss.Style {
	switch statusClass(v) {
	case classGood:
		return th.Good
	case classWarn:
		return th.Warn
	case classBad:
		return th.Bad
	}
	return th.Cell
}

const (
	classNone = iota
	classGood
	classWarn
	classBad
)

// statusClass classifies a status/phase string by substring so it copes with
// the many variants (ContainerCreating, Init:0/2, CrashLoopBackOff, ...).
func statusClass(v string) int {
	lc := strings.ToLower(strings.TrimSpace(v))
	if lc == "" {
		return classNone
	}
	containsAny := func(subs ...string) bool {
		for _, s := range subs {
			if strings.Contains(lc, s) {
				return true
			}
		}
		return false
	}
	switch {
	case containsAny("err", "fail", "crash", "backoff", "oomkill", "evict", "unschedul", "lost", "invalid", "cannot", "forbidden", "denied"):
		return classBad
	case containsAny("pend", "creating", "terminat", "init", "progress", "unknown", "notready", "not ready", "wait", "release", "drain", "cordon"):
		return classWarn
	case containsAny("run", "ready", "active", "bound", "complete", "avail", "healthy", "succeed", "true", "normal"):
		return classGood
	}
	return classNone
}

// readyStyle colors an "x/y" ready ratio: all-ready green, none-ready red,
// partial yellow.
func readyStyle(th Theme, v string) lipgloss.Style {
	parts := strings.SplitN(strings.TrimSpace(v), "/", 2)
	if len(parts) != 2 {
		return th.Cell
	}
	x, ok1 := leadingNum(parts[0])
	y, ok2 := leadingNum(parts[1])
	if !ok1 || !ok2 || y == 0 {
		return th.Cell
	}
	switch {
	case x >= y:
		return th.Good
	case x == 0:
		return th.Bad
	default:
		return th.Warn
	}
}
