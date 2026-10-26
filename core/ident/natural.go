package ident

import (
	"cmp"
	"strings"
)

// CompareNatural compares two names with their digit runs read as numbers, so R2 < R10 and
// U10.3 < U10.12. Text runs compare case-insensitively, and the plain strings break a tie, so R01
// and R1 still order the same way every time.
//
// It is the order a reader expects a list of ref-des, pins or nets in, and it lives here rather than
// in service so core/check can order evidence the way Tabulate orders a name column (agni issue
// 1165). The query panel sorts with its own copy, held to this one by testdata/natural_twin.json.
func CompareNatural(x, y string) int {
	a, b := x, y
	for a != "" && b != "" {
		da, db := isDigit(a[0]), isDigit(b[0])
		switch {
		case da && db:
			ra, rb := digitRun(a), digitRun(b)
			na, nb := strings.TrimLeft(ra, "0"), strings.TrimLeft(rb, "0")
			if len(na) != len(nb) {
				return cmp.Compare(len(na), len(nb))
			}
			if c := strings.Compare(na, nb); c != 0 {
				return c
			}
			a, b = a[len(ra):], b[len(rb):]
		case da != db:
			// A digit sorts before a letter, as it does in plain text.
			if da {
				return -1
			}
			return 1
		default:
			ta, tb := textRun(a), textRun(b)
			if c := strings.Compare(strings.ToLower(ta), strings.ToLower(tb)); c != 0 {
				return c
			}
			a, b = a[len(ta):], b[len(tb):]
		}
	}
	if a != b {
		return cmp.Compare(len(a), len(b))
	}
	return strings.Compare(x, y)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func digitRun(s string) string {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i]
}

func textRun(s string) string {
	i := 0
	for i < len(s) && !isDigit(s[i]) {
		i++
	}
	return s[:i]
}
