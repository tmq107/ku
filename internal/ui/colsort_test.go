package ui

import "testing"

func TestCellLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool // want a < b
	}{
		{"2h", "5d", true},      // 2h younger than 5d
		{"5d", "2h", false},     // and not the reverse
		{"90m", "2h", true},     // 90m (5400s) < 2h (7200s)
		{"1h30m", "100m", true}, // 5400s < 6000s
		// Durations sort by elapsed time in any column (e.g. LAST SEEN in
		// Events), not by leading number or lexically.
		{"2s", "2m8s", true},
		{"2m59s", "20m", true},
		{"20m", "3h", true},
		{"3h", "20m", false},
		{"100m", "2000m", true}, // CPU: numeric, not lexical ("2000m" < "100m" lexically)
		{"512Mi", "2048Mi", true},
		{"9%", "80%", true}, // numeric, not lexical
		{"2", "10", true},   // restarts
		{"1/1", "10/10", true},
		{"alpha", "beta", true},
		{"Zeta", "alpha", false}, // case-insensitive: z > a
		{"Running", "Pending", false},
	}
	for _, c := range cases {
		if got := cellLess(c.a, c.b); got != c.want {
			t.Errorf("cellLess(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCellLessEmptySortsLast(t *testing.T) {
	// A real value should come before "-" / "<none>" / "" regardless of direction.
	for _, empty := range []string{"-", "", "<none>"} {
		if cellLess(empty, "5m") {
			t.Errorf("empty %q should not sort before a numeric value", empty)
		}
		if !cellLess("5m", empty) {
			t.Errorf("numeric value should sort before empty %q", empty)
		}
	}
}
