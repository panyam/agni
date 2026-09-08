package builtin

import (
	"fmt"
	"sort"
	"strings"

	"github.com/panyam/agni/core/check"
)

// i2cRedundantPullUp flags an I2C net pulled to ONE rail by more than one resistor. See Detail.
//
// It is the count question PullUpReachesRail cannot ask. Reachability reports a rail as reached
// however many resistors reach it, so a doubly-pulled bus and a correct one are the same answer, and
// only AllPullUpPathsToRail can tell them apart.
var i2cRedundantPullUp = &check.Rule{
	Name:     "i2c-redundant-pull-up",
	Severity: "warning",
	Summary:  "An I2C net is pulled to one rail by more than one resistor.",
	Impact:   "Pull-ups in parallel are one smaller resistor. Two 2.2k become an effective 1.1k, so the bus sinks roughly twice the current it was sized for and a device driving low may not reach its VOL, which reads as a bus that works on some parts and not others. It usually arrives when a module carries its own termination for a bus the board already pulls, which is the ordinary consequence of designing a mezzanine to work standalone.",
	Remedy:   "Remove one of the pull-ups, or depopulate it. Which one is a board decision: keep the pull-up sized for the bus capacitance and clock rate this design actually runs at, and take out the one that came along with a module.",
	// A warning rather than an error, because a jumper-selectable or DNP termination is legal by
	// design and the netlist cannot see the fit status.
	Primitives: []string{"select", "pattern", "traverse", "exists", "reach"},
	Reads:      []string{"net.names", "on_net", "component.class"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("i2c-redundant-pull-up"),
	Eval:                i2cRedundantPullUpVerdicts,
	StatesConsideredSet: true,
}

// i2cPullUpSplitRail flags an I2C net pulled to MORE THAN ONE rail. See Detail.
//
// A separate rule rather than a clause, because a rule carries one severity and one remedy and this
// is a different defect from the one above: the fix is not "remove a resistor" but "decide which
// supply this bus belongs to". The two are mutually exclusive by construction, so a bus is never
// reported twice.
var i2cPullUpSplitRail = &check.Rule{
	Name:       "i2c-pull-up-split-rail",
	Severity:   "warning",
	Summary:    "An I2C net is pulled up to two different rails.",
	Impact:     "The bus ties two supplies together through its pull-ups whenever one rail is up and the other is not, so a rail that is meant to be off is fed through the bus during sequencing. It also means the bus idles at whichever voltage wins, which may exceed the input rating of a device on the lower supply. Sequencing faults of this kind pass on the bench, where everything powers at once, and appear as a board that fails only on a particular power-up order.",
	Remedy:     "Pull the bus to one rail. Where the bus genuinely crosses supply domains, that is a level translator's job rather than two pull-ups, and the translator's own pull-ups belong one per side of it.",
	Primitives: []string{"select", "pattern", "traverse", "exists", "reach"},
	Reads:      []string{"net.names", "on_net", "component.class"},
	Tags: map[string]string{
		check.KeyCategory:     check.CategoryConnectivity,
		check.KeyTier:         "R",
		check.KeyDistribution: check.DistOpen,
	},
	Detail:              ruleDoc("i2c-pull-up-split-rail"),
	Eval:                i2cPullUpSplitRailVerdicts,
	StatesConsideredSet: true,
}

// i2cRedundantPullUpVerdicts decides every I2C net: more than one resistor, all landing on ONE rail.
//
// Its considered set is every I2C net, including the ones pulled correctly, which is the point of a
// count rule: "this bus has exactly one pull-up" is a fact worth stating, and a rule reporting only
// the doubled ones could not distinguish a clean board from an unexamined one.
func i2cRedundantPullUpVerdicts(m check.Model) []check.Verdict {
	return pullUpCountVerdicts(m, "i2c-redundant-pull-up", func(rails []string) bool {
		return len(rails) == 1
	}, func(n string, terms []check.PullUpTermination) string {
		return fmt.Sprintf("I2C net has %d pull-up resistors to %s", len(terms), terms[0].Rail)
	})
}

// i2cPullUpSplitRailVerdicts decides every I2C net: more than one resistor, landing on DIFFERENT
// rails.
func i2cPullUpSplitRailVerdicts(m check.Model) []check.Verdict {
	return pullUpCountVerdicts(m, "i2c-pull-up-split-rail", func(rails []string) bool {
		return len(rails) > 1
	}, func(n string, terms []check.PullUpTermination) string {
		return fmt.Sprintf("I2C net is pulled up to %s", strings.Join(distinctRails(terms), " and "))
	})
}

// pullUpCountVerdicts is the shared body: enumerate the I2C nets, count their distinct terminating
// resistors, and let the caller decide which rail arrangement is its defect.
//
// One walk, two rules, and the two fire on disjoint conditions (one rail versus several), so a bus
// with a genuine problem is named once by the rule whose remedy applies to it. A rule carries one
// severity and one remedy (see docsite build/check-rule.md), which is why this is two rules over one
// walker rather than one rule with a clause.
//
// A net with ZERO pull-ups is NOT considered here. It is i2c-pull-up's subject, that rule is an
// error where these are warnings, and reporting the same absence three times would treble one defect.
func pullUpCountVerdicts(m check.Model, rule string, fires func(rails []string) bool, message func(string, []check.PullUpTermination) string) []check.Verdict {
	var out []check.Verdict
	for _, n := range m.Nets() {
		if !isI2C(n.Name) {
			continue
		}
		terms := check.AllPullUpPathsToRail(m, n)
		if len(terms) == 0 {
			continue // i2c-pull-up's subject, not this rule's
		}
		subject := check.Entity{Kind: check.KindNet, Ref: n.Name, NetID: n.GetId()}
		v := check.Verdict{Subjects: []check.Entity{subject}, Rule: rule, Outcome: check.Pass}
		rails := distinctRails(terms)
		v.Witness, v.Context = pullUpCountWitness(n.Name, terms, rails)
		if len(terms) > 1 && fires(rails) {
			v.Outcome = check.Fail
			f := check.NetFinding(message(n.Name, terms))(n)
			v.Finding = &f
		}
		out = append(out, v)
	}
	return out
}

// pullUpCountWitness proves the count with the parts it counted, on a pass as much as on a failure.
// "SDA is pulled to +3V3 by R7" is what makes a passing bus checkable, and it is the same
// proof-on-pass discipline PullUpVerdict applies to the absence question.
func pullUpCountWitness(net string, terms []check.PullUpTermination, rails []string) (*check.Witness, []check.ContextSubject) {
	var refs []string
	ctx := make([]check.ContextSubject, 0, len(terms)*2)
	for _, e := range terms {
		refs = append(refs, e.Resistor)
		ctx = append(ctx,
			check.ContextSubject{Entity: check.Entity{Kind: check.KindComponent, Ref: e.Resistor}, Role: "pull-up"},
			check.ContextSubject{Entity: check.Entity{Kind: check.KindNet, Ref: e.Rail}, Role: "rail"})
	}
	w := &check.Witness{
		Statement: fmt.Sprintf("%s is pulled to %s by %s", net,
			strings.Join(rails, " and "), strings.Join(refs, ", ")),
		Terms: []check.WitnessTerm{{Label: "pull-ups", Value: fmt.Sprintf("%d", len(terms))}},
	}
	return w, ctx
}

// distinctRails is the sorted set of rails the terminations landed on. Sorted so a message and a
// witness read the same way on every run, which a committed capture depends on.
func distinctRails(terms []check.PullUpTermination) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range terms {
		if !seen[e.Rail] {
			seen[e.Rail] = true
			out = append(out, e.Rail)
		}
	}
	sort.Strings(out)
	return out
}
