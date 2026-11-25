package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.10", -1},
		{"2.0", "1.99.99", 1},
		{"1.0", "1.0.0.0", 0},
		{"v10.0.1", "10.0.1", 0},
		{"1.0.0-beta", "1.0.0", -1},
		{"1.0.1-rc1", "1.0.0", 1},
		{"2024.1", "2023.12", 1},
		{"1.2.3.4567", "1.2.3.4568", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
