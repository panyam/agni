package netgraph

import (
	"regexp"
	"strconv"
)

// busRangeRe matches a range-bus name's index-range suffix: a prefix, an open bracket, two decimal
// indices separated by the dialect's range operator, a close bracket at the end. A scalar indexed
// net like `A[3]` (no range) is not a range bus.
//
// BOTH separators are accepted because the formats disagree and one helper serves all of them.
// xschem and gEDA write `DATA[7:0]`; KiCad writes `AN[0..7]`, and only ever that — a KiCad label
// spelled `DATA[1:0]` is a plain scalar net name, not a bus. Recognizing only the colon form is
// what hid agni issue 561: no KiCad bus vector was ever detected as one, so `AN[0..7]` crossing a
// sheet boundary carried none of its members with it.
var busRangeRe = regexp.MustCompile(`^(.*)\[(\d+)(?::|\.\.)(\d+)\]$`)

// ExpandBusName expands a RANGE bus name into its member signal names in WRITTEN order: `DATA[7:0]`
// -> [DATA7 DATA6 ... DATA0], `A[0:3]` -> [A0 A1 A2 A3], `AN[0..7]` -> [AN0 AN1 ... AN7]. The member
// key is the prefix concatenated with the index, matching KiCad's own bus-member naming and the
// labels an author places on the taps.
// Returns nil for a name that is not a range bus (a scalar net, or an alias-named bus whose members
// are an explicit list the caller supplies instead). The order follows the written range direction so
// a consumer that cares (a diagram) reads bits as drawn; a resolution check treats it as a set.
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

// IsBusName reports whether name is a range-bus name (carries an index range in either dialect's
// spelling). It is the geometry-free bus detector the readers share (a bus label's identity is its
// range syntax).
func IsBusName(name string) bool { return busRangeRe.MatchString(name) }

func mustAtoi(s string) int { n, _ := strconv.Atoi(s); return n }
