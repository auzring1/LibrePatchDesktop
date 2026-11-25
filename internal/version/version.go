// Package version compares the loose version strings found in the wild
// ("1.2.3", "2024.1", "v10.0.1-beta", "1.2.3.4567").
package version

import (
	"strconv"
	"strings"
	"unicode"
)

// Compare returns -1, 0 or 1 when a is older than, equal to or newer than b.
// Numeric parts compare as numbers, missing trailing parts count as zero, and a
// pre-release suffix ("-beta") sorts before the same plain version.
func Compare(a, b string) int {
	an, apre := split(a)
	bn, bpre := split(b)
	for i := 0; i < len(an) || i < len(bn); i++ {
		var x, y int
		if i < len(an) {
			x = an[i]
		}
		if i < len(bn) {
			y = bn[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case apre == bpre:
		return 0
	case apre && !bpre:
		return -1
	default:
		return 1
	}
}

// split returns the numeric parts and whether a pre-release suffix is present.
func split(v string) (nums []int, pre bool) {
	v = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(v), "vV"))
	core := v
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		core = v[:i]
		pre = strings.HasPrefix(v[i:], "-") && strings.IndexFunc(v[i:], unicode.IsLetter) >= 0
	}
	for _, p := range strings.FieldsFunc(core, func(r rune) bool { return r == '.' || r == '_' }) {
		n, err := strconv.Atoi(p)
		if err != nil {
			// Take the leading digits of parts such as "3b2".
			end := 0
			for end < len(p) && p[end] >= '0' && p[end] <= '9' {
				end++
			}
			n, _ = strconv.Atoi(p[:end])
		}
		nums = append(nums, n)
	}
	return nums, pre
}
