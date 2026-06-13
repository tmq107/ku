package ui

import "testing"

func TestCellLess(t *testing.T) {
	cases := []struct {
		col, a, b string
		want      bool // want a < b
	}{
		{"Age", "2h", "5d", true},      // 2h younger than 5d
		{"Age", "5d", "2h", false},     // and not the reverse
		{"Age", "90m", "2h", true},     // 90m (5400s) < 2h (7200s)
		{"Age", "1h30m", "100m", true}, // 5400s < 6000s
		{"CPU", "100m", "2000m", true}, // numeric, not lexical ("2000m" < "100m" lexically)
		{"MEM", "512Mi", "2048Mi", true},
		{"MEM%", "9%", "80%", true}, // numeric, not lexical
		{"Restarts", "2", "10", true},
		{"Ready", "1/1", "10/10", true},
		{"Name", "alpha", "beta", true},
		{"Name", "Zeta", "alpha", false}, // case-insensitive: z > a
		{"Status", "Running", "Pending", false},
	}
	for _, c := range cases {
		if got := cellLess(c.col, c.a, c.b); got != c.want {
			t.Errorf("cellLess(%q, %q, %q) = %v, want %v", c.col, c.a, c.b, got, c.want)
		}
	}
}

func TestCellLessEmptySortsLast(t *testing.T) {
	// A real value should come before "-" / "<none>" / "" regardless of direction.
	for _, empty := range []string{"-", "", "<none>"} {
		if cellLess("CPU", empty, "5m") {
			t.Errorf("empty %q should not sort before a numeric value", empty)
		}
		if !cellLess("CPU", "5m", empty) {
			t.Errorf("numeric value should sort before empty %q", empty)
		}
	}
}
