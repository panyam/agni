package builtin

import (
	"fmt"
	"strconv"

	"github.com/panyam/agni/core/check"
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
	Eval: func(m check.Model) []check.Verdict {
		return declaredVsActual(m, "track_width", "routed width", "declared track width",
			func(bn check.BoardNet) (float64, bool) { return minSegmentWidthMM(bn) },
			"the net carries no routed track, so there is no width to compare",
			"routed at %s, narrower than the %s its net class %q declares")
	},
	StatesConsideredSet: true,
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
	Eval: func(m check.Model) []check.Verdict {
		return declaredVsActual(m, "via_drill", "smallest drill", "declared via drill",
			func(bn check.BoardNet) (float64, bool) { return minViaDrillMM(bn) },
			"the net carries no via, so there is no drill to compare",
			"drilled at %s, smaller than the %s its net class %q declares")
	},
	StatesConsideredSet: true,
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
// A NET THE PROJECT CONSTRAINED NOWHERE IS NoLimit, which is the outcome the datasheet rules produce
// for a row stating no bound and is the same situation one tier down: the comparison was reached and
// nothing constrains the value. Before, that took the same silent path as a net comfortably above its
// declared width, so a project that forgot to state a width for its power class read exactly like one
// whose power tracks are all wide enough. It is the false-pass shape CapNetClassDefs prevents for the
// design as a whole and could not see per net.
//
// check.CompareToBound does the comparison and builds the witness in one call, so there is no way to
// reach a pass here without the statement that justifies it.
func declaredVsActual(
	m check.Model,
	param string,
	quantity string, // names the measurement in a witness: "routed width"
	limitName string, // names the bound in a witness: "declared track width"
	actual func(check.BoardNet) (float64, bool),
	noCopperReason string,
	msg string,
) []check.Verdict {
	// No explicit empty-definitions guard: with no definitions there is no class stating anything,
	// so the cascade reports "not stated" for every net and every verdict is NoLimit.
	// CapNetClassDefs is what reports that situation to a review as a rule-level answer.
	cascade := check.NewNetClassCascade(m.NetClassDefs())
	classesOf := map[string][]string{}
	for _, n := range m.Nets() {
		classesOf[n.GetName()] = n.GetNetClasses()
	}

	var out []check.Verdict
	for _, bn := range m.BoardNets() {
		v := check.Verdict{Subjects: []check.Entity{check.Entity{Kind: check.KindNet, Ref: bn.Net}}}
		act, ok := actual(bn)
		if !ok {
			// Not a pass and not a limit question: there is no copper of this kind to measure, so
			// the rule reached no comparison at all.
			v.Outcome = check.NotConsidered
			v.Reason = noCopperReason
			out = append(out, v)
			continue
		}
		declared, from, stated := cascade.Declared(classesOf[bn.Net], param)
		bound := check.Bound{}
		if stated {
			bound.Min = &declared
		}
		outcome, w := check.CompareToBound(act, "mm", bound, quantity, limitName)
		v.Outcome, v.Witness = outcome, w
		if stated && w != nil {
			// Which class the limit came from. The number alone does not say whose rule it is, and
			// on a board where several classes bear on one net that is the first thing a reader asks.
			w.Terms = append(w.Terms, check.WitnessTerm{Label: "declared by class", Value: from})
		}
		if outcome == check.Fail {
			v.Finding = &check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: bn.Net}, Severity: "warning", Message: fmt.Sprintf(msg, mmText(act), mmText(declared), from)}
		}
		out = append(out, v)
	}
	return out
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
