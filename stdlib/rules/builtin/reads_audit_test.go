package builtin

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// auditModel is the design every rule is run against for the declaration audit: the parity
// fixture's messy netlist with BOTH optional tiers attached.
//
// Attaching them is what makes the audit possible at all. Available gates a param or board rule to
// not-applicable when its tier is absent, so on a bare design those rules never execute, never
// touch an accessor, and would report as reading nothing — a false clean covering exactly the
// rules the audit exists for.
//
// The param set is empty on purpose. What is recorded is the CALL, not what it returns: a rule that
// asks for a PartSpec and gets nil has still demonstrated that it depends on the datasheet tier.
// Seeding real specs would deepen each rule's path but is not needed to observe the dependency, and
// an empty set keeps the fixture from encoding one rule's expectations.
func auditModel() *check.RecordingModel {
	d := specParityFixture()
	// Net-class definitions are a third optional tier, gated by its own capability (WS3-105): with
	// none declared, the two netclass rules read not-applicable and the audit never exercises them.
	// One default class is enough, since the audit records the accessor call rather than the verdict.
	d.Constraints = append(d.Constraints, &ir.Constraint{
		Name: "Default", Kind: "netclass",
		Params: map[string]string{"priority": "2147483647", "is_default": "true", "track_width": "0.25", "via_drill": "0.3"},
	})
	return check.NewRecordingModel(check.NewModelWithParams(d, drcBoard(), param.ParamSet{}))
}

// tiersRead runs one rule and reports the gated tiers it reached for.
func tiersRead(m *check.RecordingModel, r *check.Rule) []check.FactTier {
	m.Reset()
	check.Run(m, []*check.Rule{r})
	return m.Read()
}

func tierNames(ts []check.FactTier) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, string(t))
	}
	sort.Strings(out)
	return out
}

// TestReadsAreDeclared is the audit (WS3-122): every gated tier a rule actually reads must appear
// in its declared Reads.
//
// Three gates trust that declaration to decide whether a rule can run — the param tier, the board
// tier, and the connectivity gate that reports inconclusive while a symbol is unresolved. An
// UNDER-declared rule escapes its gate: it runs on a design whose tier was never attached and
// reports over data it does not have, which looks exactly like a clean result.
//
// Only 15 of the shipped rules have a Spec twin whose derived reads are checked against the
// declaration (TestSpecMetadata). This covers the rest, and covers the twinned ones too rather than
// carving them out, since a second independent check of the same property is free here.
//
// The assertion is deliberately ONE-DIRECTIONAL. An observed read proves a dependency; an absent
// read proves nothing, because a rule that early-returns on this fixture never reaches its
// accessors. So an over-declared rule is reported by TestReadsAudit below as information, never
// failed here. Asserting both directions would produce a test that breaks whenever the fixture
// changes shape, which is worse than no test.
func TestReadsAreDeclared(t *testing.T) {
	m := auditModel()
	for _, r := range rules {
		declared := check.DeclaredTiers(r)
		for _, got := range tiersRead(m, r) {
			if !declared[got] {
				t.Errorf("%s reads the %s tier but does not declare it (Reads: %v)\n"+
					"    an undeclared tier escapes its gate: the rule runs where it should read "+
					"not-applicable or inconclusive, and reports over data it does not have",
					r.Name, got, r.Reads)
			}
		}
	}
}

// TestReadsAudit reports the other direction, which cannot be asserted: a tier a rule declares but
// was not observed reading. Each is either a genuine over-declaration (the rule reads
// not-applicable where it would have worked) or simply a path this fixture does not drive. Telling
// those apart needs a human, so this logs and never fails.
//
// It also prints twin coverage, since extending that is the other half of WS3-122 and the count is
// the thing anyone picking it up wants first.
func TestReadsAudit(t *testing.T) {
	m := auditModel()
	var twinned, untwinned int
	var overDeclared []string
	for _, r := range rules {
		if _, ok := specs[r.Name]; ok {
			twinned++
		} else {
			untwinned++
		}
		observed := map[check.FactTier]bool{}
		for _, got := range tiersRead(m, r) {
			observed[got] = true
		}
		var missing []string
		for tier := range check.DeclaredTiers(r) {
			if !observed[tier] {
				missing = append(missing, string(tier))
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			twin := "no twin"
			if _, ok := specs[r.Name]; ok {
				twin = "twinned"
			}
			overDeclared = append(overDeclared, r.Name+" ["+twin+"] declares "+strings.Join(missing, ",")+" but was not observed reading it")
		}
	}
	sort.Strings(overDeclared)
	t.Logf("catalog: %d rules, %d twinned, %d untwinned", len(rules), twinned, untwinned)
	t.Logf("declared-but-unobserved (%d):\n  %s", len(overDeclared), strings.Join(overDeclared, "\n  "))

	// The audit's REACH, which matters more than its verdict. A rule that reached no gated accessor
	// was not audited for under-declaration at all: the pass above says nothing about it. Reporting
	// the number keeps a green run from reading as "the catalog is verified" when a third of it was
	// never exercised.
	var silent []string
	for _, r := range rules {
		if len(tiersRead(m, r)) == 0 && len(check.DeclaredTiers(r)) > 0 {
			silent = append(silent, r.Name)
		}
	}
	sort.Strings(silent)
	t.Logf("declare a gated tier but reached NO gated accessor on this fixture, so under-declaration is unproven for them (%d):\n  %s",
		len(silent), strings.Join(silent, "\n  "))
}

// TestReadsAuditCoversEveryRule pins the audit's own reach. The audit is only worth its runtime if
// the rules actually EXECUTE against the fixture: a rule gated to not-applicable never touches an
// accessor, so a fixture that silently stopped attaching a tier would turn the audit into a test
// that passes by doing nothing.
func TestReadsAuditCoversEveryRule(t *testing.T) {
	m := auditModel()
	for _, r := range rules {
		if ok, reason := check.Available(r, m); !ok {
			t.Errorf("%s is not available against the audit fixture (%s), so the audit never exercises it", r.Name, reason)
		}
	}
	if len(tierNames(tiersRead(m, ruleByNameForAudit(t, "track-width")))) == 0 {
		t.Error("a board rule read no tier against the audit fixture: the board tier is not attached")
	}
}

func ruleByNameForAudit(t *testing.T, name string) *check.Rule {
	t.Helper()
	for _, r := range rules {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("rule %q not in the catalog", name)
	return nil
}
