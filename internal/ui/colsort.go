package ui

import (
	"strconv"
	"strings"
)

// cellLess orders two table cells from the named column. Numeric-looking values
// (CPU "245m", MEM "1234Mi", percentages, restart counts, ready ratios) compare
// numerically, ages compare by duration, and everything else compares
// case-insensitively. Unparseable/empty cells sort after numeric ones.
func cellLess(col, a, b string) bool {
	av, aok := sortVal(col, a)
	bv, bok := sortVal(col, b)
	if aok && bok {
		if av != bv {
			return av < bv
		}
		return false
	}
	if aok != bok {
		return aok
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

func sortVal(col, s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "<none>" {
		return 0, false
	}
	if strings.EqualFold(col, "age") {
		return parseAge(s)
	}
	// CPU/MEM/percent/restarts/ready all lead with their number, so the leading
	// numeric prefix is a stable comparison key for them.
	return leadingNum(s)
}

// parseAge converts a Kubernetes age like "5d", "2y64d", or "1h30m" to seconds.
func parseAge(s string) (float64, bool) {
	units := map[byte]float64{'s': 1, 'm': 60, 'h': 3600, 'd': 86400, 'y': 31536000}
	var total float64
	var num strings.Builder
	matched := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			num.WriteByte(c)
			continue
		}
		mult, ok := units[c]
		if !ok || num.Len() == 0 {
			return 0, false
		}
		n, _ := strconv.Atoi(num.String())
		total += float64(n) * mult
		num.Reset()
		matched = true
	}
	if num.Len() > 0 { // trailing digits with no unit: not an age
		return 0, false
	}
	return total, matched
}

// leadingNum parses the leading numeric run of s (e.g. "245m" -> 245, "1/2" -> 1).
func leadingNum(s string) (float64, bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return 0, false
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
