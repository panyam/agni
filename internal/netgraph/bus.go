package netgraph

import (
	"regexp"
	"strconv"
)

// busRangeRe matches a range-bus name's suffix, two decimal indices in brackets separated by the
// dialect's range operator. A scalar indexed net like `A[3]` is not a range bus.
//
// BOTH separators are accepted because one helper serves every format. xschem and gEDA write
// `DATA[7:0]`, and KiCad writes only `AN[0..7]`, so a KiCad label spelled `DATA[1:0]` is a plain
// scalar net. Missing the `..` form dropped every KiCad bus member at a sheet boundary (agni issue
// 561). See docsite/content/architecture/net-solving.md#buses-cross-by-a-different-rule.
var busRangeRe = regexp.MustCompile(`^(.*)\[(\d+)(?::|\.\.)(\d+)\]$`)

// ExpandBusName expands a RANGE bus name into its member signal names in WRITTEN order: `DATA[7:0]`
// -> [DATA7 DATA6 ... DATA0], `A[0:3]` -> [A0 A1 A2 A3], `AN[0..7]` -> [AN0 AN1 ... AN7]. The member
// name is the prefix concatenated with the index, matching KiCad's bus-member naming and the tap
// labels. It returns nil for a name that is not a range bus (a scalar net, or an alias-named bus
// whose members the caller supplies). Written order lets a diagram read bits as drawn.
func ExpandBusName(name string) []string {
	m := busRangeRe.FindStringSubmatch(name)
	if m == nil {
		return nil
	}
	prefix, hi, lo := m[1], mustAtoi(m[2]), mustAtoi(m[3])
	step := 1
	if hi < lo {
		step = -1 // ascending range written [lo:hi]
	}
	var out []string
	for i := hi; ; i -= step {
		out = append(out, prefix+strconv.Itoa(i))
		if i == lo {
			break
		}
	}
	return out
}

// IsBusName reports whether name carries an index range in either dialect's spelling. It is the
// geometry-free bus detector the readers share.
func IsBusName(name string) bool { return busRangeRe.MatchString(name) }

func mustAtoi(s string) int { n, _ := strconv.Atoi(s); return n }
