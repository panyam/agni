package builtin

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Declared-vs-actual net-class conformance (WS3-111). These are the first rules in the catalog that
// compare a board against a limit THE PROJECT SET rather than a constant compiled in here. The
// track-width and hole-size rules next door check manufacturability against a universal fabrication
// floor; these check the design against its own stated intent, which is a different question and can
// fail on a perfectly manufacturable board.
//
// Both gate on CapNetClassDefs, so a design that declares no class definitions reads not-applicable
// rather than running over zero comparisons and reporting a clean pass.

var netclassTrackWidth = &check.Rule{
	Name:               "netclass-track-width",
	Severity:           "warning",
	Summary:            "A net is routed narrower than the track width its own net class declares.",
	Impact:             "The project states, per net class, the track width its nets are meant to route at. A net routed below its declared width is a silent departure from that intent: on a power class it is a current-density and heating risk, on a controlled-impedance class it shifts the impedance the class exists to hold. Unlike the fabrication-floor check, this can fire on a board that manufactures fine — the board is buildable, it is just not what the design asked for.",
	Remedy:             "Widen the track to the width its class declares, or amend the class if the declaration is the half that is out of date. The board may build either way, so decide which of the two states the design's intent.",
	Primitives:         []string{"select", "compare"},
	Reads:              []string{"net.declared_track_width", "board.track_width"},
	RequiresCapability: []check.Capability{check.CapNetClassDefs},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("netclass-track-width"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		return declaredVsActual(m, "track_width",
			func(bn check.BoardNet) (float64, bool) { return minSegmentWidthMM(bn) },
			"routed at %s, narrower than the %s its net class %q declares")
	}),
}

var netclassViaDrill = &check.Rule{
	Name:               "netclass-via-drill",
	Severity:           "warning",
	Summary:            "A net's via is drilled smaller than the drill its own net class declares.",
	Impact:             "A net class declares the via drill its nets should use, usually sized for the current the class carries or for the fab process the board is quoted against. A via drilled below it departs from that intent silently: the board may still build, but the class's assumption about current capacity or plating no longer holds.",
	Remedy:             "Enlarge the via drill to the size its class declares, or amend the class if the smaller drill is intended. The class carries an assumption about current capacity or plating, and one of the two has moved.",
	Primitives:         []string{"select", "compare"},
	Reads:              []string{"net.declared_via_drill", "board.via_drill"},
	RequiresCapability: []check.Capability{check.CapNetClassDefs},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryBoard,
		check.KeyTier:         "P",
		check.KeyDistribution: check.DistOpen,
	},
	Detail: ruleDoc("netclass-via-drill"),
	Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
		return declaredVsActual(m, "via_drill",
			func(bn check.BoardNet) (float64, bool) { return minViaDrillMM(bn) },
			"drilled at %s, smaller than the %s its net class %q declares")
	}),
}

// declaredVsActual is the shared body: for each routed net, resolve what the project declared for
// this quantity and compare the copper against it.
//
// The resolution is the whole difficulty, and it is why this cannot be a naive join. A net belongs to
// a SET of classes (WS1-050), and KiCad does not pick one of them: it fills each constraint from the
// highest-priority class that states THAT constraint, with the Default class supplying whatever is
// left and applying to every net including unclassed ones. Comparing the copper against each class
// the net belongs to would fail a net that correctly obeys the class that won.
//
// A net the project constrained nowhere yields nothing to compare and is skipped, never passed.
func declaredVsActual(m check.Model, param string, actual func(check.BoardNet) (float64, bool), msg string) []check.Finding {
	// No explicit empty-definitions guard: with no definitions there is no class stating anything,
	// so declaredFor reports "not stated" for every net and nothing fires. Mutation testing showed a
	// guard here was unreachable. CapNetClassDefs is what reports the situation honestly to a review.
	defs := m.NetClassDefs()
	byName := make(map[string]*ir.Constraint, len(defs))
	var defaultClass string
	for _, c := range defs {
		byName[c.GetName()] = c
		if c.GetParams()["is_default"] == "true" {
			defaultClass = c.GetName()
		}
	}

	var out []check.Finding
	for _, bn := range m.BoardNets() {
		act, ok := actual(bn)
		if !ok {
			continue // no copper of this kind on the net; nothing to compare
		}
		declared, from, ok := declaredFor(m, bn.Net, param, byName, defaultClass)
		if !ok || act >= declared {
			continue
		}
		out = append(out, check.Finding{
			Severity: "warning",
			Kind:     check.KindNet,
			Subject:  bn.Net,
			Message:  fmt.Sprintf(msg, mmText(act), mmText(declared), from),
		})
	}
	return out
}

// declaredFor resolves one quantity for one net: the value from the highest-priority class stating
// it, and the name of that class so a finding can say where the limit came from.
func declaredFor(m check.Model, netName, param string, byName map[string]*ir.Constraint, defaultClass string) (float64, string, bool) {
	var classes []string
	for _, n := range m.Nets() {
		if n.GetName() == netName {
			classes = append(classes, n.GetNetClasses()...)
			break
		}
	}
	sort.SliceStable(classes, func(i, j int) bool {
		return constraintPriority(byName[classes[i]]) < constraintPriority(byName[classes[j]])
	})
	if defaultClass != "" {
		classes = append(classes, defaultClass) // always last, and applies even to an unclassed net
	}
	for _, cls := range classes {
		c := byName[cls]
		if c == nil {
			continue // assigned to a class the project never defined; states nothing
		}
		if v, err := strconv.ParseFloat(c.GetParams()[param], 64); err == nil {
			return v, cls, true
		}
	}
	return 0, "", false
}

func constraintPriority(c *ir.Constraint) int {
	if c == nil {
		return math.MaxInt32
	}
	p, err := strconv.Atoi(c.GetParams()["priority"])
	if err != nil {
		return math.MaxInt32
	}
	return p
}

func mmText(mm float64) string { return strconv.FormatFloat(mm, 'g', -1, 64) + "mm" }

// minSegmentWidthMM / minViaDrillMM narrow a net's copper to the value the declared limit is about:
// its THINNEST track and its SMALLEST drill. A net conforms only if its worst copper does, so a
// single narrow segment on an otherwise-wide net is the finding. Matches board.track_width /
// board.via_drill, which project the same minima, so a rule and an ad-hoc query agree.
func minSegmentWidthMM(bn check.BoardNet) (float64, bool) {
	min := int64(0)
	for _, s := range bn.Segments {
		if s.Width > 0 && (min == 0 || s.Width < min) {
			min = s.Width
		}
	}
	if min == 0 {
		return 0, false
	}
	return float64(min) / 1e6, true
}

func minViaDrillMM(bn check.BoardNet) (float64, bool) {
	min := int64(0)
	for _, v := range bn.Vias {
		if v.Drill > 0 && (min == 0 || v.Drill < min) {
			min = v.Drill
		}
	}
	if min == 0 {
		return 0, false
	}
	return float64(min) / 1e6, true
}
