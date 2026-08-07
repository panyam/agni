package profiles

import (
	"fmt"
	"sort"
)

// A matcher that is merely too LOOSE cannot be caught at load time. validateSignalMatcher (WS3-057)
// rejects the definitionally universal patterns — no form, several forms, one that will not compile,
// one that matches the empty net name — because those are wrong against every design. An unanchored
// regex "_H" is different: it is a legitimate pattern that happens to claim every _H net on THIS
// board, Ethernet and CAN alike. Deciding that needs the design, and a profile is loaded once and
// applied to many, so the judgement belongs here rather than in Parse or Compile (WS3-101).
//
// These thresholds are deliberately permissive. A config warning that cries wolf gets ignored, and
// then it is worse than absent, so the bar is "no interface could plausibly look like this" rather
// than "this looks suspicious".
const (
	// overBroadShare is the fraction of a design's nets one signal may claim before the matcher is
	// reported. It is a SHARE rather than a count because designs differ by orders of magnitude and a
	// role legitimately matches one net per interface INSTANCE — a CarCo ECU carries 16 LIN channels,
	// so 16 nets matching _TX is correct, not broken. A quarter of every net on the board cannot be
	// one role of one interface even at that instance count.
	overBroadShare = 0.25

	// overBroadFloor is the smallest number of matched nets worth reporting at all. On a small design
	// (a test fixture, a demo with a handful of nets) a single match is already a large share, so the
	// share test alone would fire on correct profiles. The floor keeps it quiet until the share means
	// something.
	overBroadFloor = 8
)

// Diagnose reports config-quality problems with profile p as applied to a design's net names: a
// matcher claiming an implausible share of the board, and two of the profile's own signals resolving
// to the same net. It returns human-readable lines, empty when the profile looks sound.
//
// These are NOT design findings, and must never be reported as any. An over-broad matcher is a
// mistake in the profile the author wrote, not a defect in the board being checked; emitting it as a
// Finding would put a config problem into a design report and score it against the design.
//
// It takes net NAMES rather than a check.Model because names are all it reads. That keeps it a pure
// function over its actual input — testable without building a design, and callable from a surface
// that has read a design but not built a model, which is where the CLI sits.
func Diagnose(netNames []string, p Profile) []string {
	if len(netNames) == 0 {
		return nil
	}
	var out []string
	out = append(out, overBroadSignals(netNames, p)...)
	out = append(out, collidingSignals(netNames, p)...)
	return out
}

// overBroadSignals reports each signal matching an implausible share of the design's nets. Signals
// are reported in declaration order so the output is stable across runs.
func overBroadSignals(netNames []string, p Profile) []string {
	var out []string
	for _, s := range p.Signals {
		n := 0
		for _, name := range netNames {
			if netMatchesSignal(name, s) {
				n++
			}
		}
		if n < overBroadFloor || float64(n) < overBroadShare*float64(len(netNames)) {
			continue
		}
		out = append(out, fmt.Sprintf(
			"profile %q: signal %q (%s) matches %d of %d nets (%.0f%%) — the matcher looks too broad to name one role; narrow it with a prefix, a glob, or an anchored regex",
			p.Name, s.Name, matcherDesc(s), n, len(netNames), 100*float64(n)/float64(len(netNames))))
	}
	return out
}

// collidingSignals reports a net matched by two DIFFERENT roles of the same profile. Unlike the share
// test this needs no threshold: a profile that cannot tell its own roles apart on a net is broken
// there by definition, and whichever rule runs first will claim it. Only the first colliding net per
// role pair is named, since one example is enough to locate the mistake and a wide matcher would
// otherwise print hundreds.
func collidingSignals(netNames []string, p Profile) []string {
	type pair struct{ a, b string }
	first := map[pair]string{}
	var order []pair
	for _, name := range netNames {
		var hit []string
		for _, s := range p.Signals {
			if netMatchesSignal(name, s) {
				hit = append(hit, s.Name)
			}
		}
		for i := 0; i < len(hit); i++ {
			for j := i + 1; j < len(hit); j++ {
				k := pair{hit[i], hit[j]}
				if _, seen := first[k]; !seen {
					first[k] = name
					order = append(order, k)
				}
			}
		}
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].a != order[j].a {
			return order[i].a < order[j].a
		}
		return order[i].b < order[j].b
	})
	out := make([]string, 0, len(order))
	for _, k := range order {
		out = append(out, fmt.Sprintf(
			"profile %q: signals %q and %q both match net %q — the profile cannot tell these two roles apart, so whichever rule runs first claims the net",
			p.Name, k.a, k.b, first[k]))
	}
	return out
}

// matcherDesc renders a signal's declared matcher for a diagnostic message, so the author sees the
// pattern to edit rather than having to find it.
func matcherDesc(s Signal) string {
	switch {
	case s.Glob != "":
		return fmt.Sprintf("glob %q", s.Glob)
	case s.Regex != "":
		return fmt.Sprintf("regex %q", s.Regex)
	case s.Prefix != "" && s.Suffix != "":
		return fmt.Sprintf("prefix %q + suffix %q", s.Prefix, s.Suffix)
	case s.Prefix != "":
		return fmt.Sprintf("prefix %q", s.Prefix)
	default:
		return fmt.Sprintf("suffix %q", s.Suffix)
	}
}
