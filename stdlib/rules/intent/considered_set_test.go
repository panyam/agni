package intent

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// verdictsFor runs the compiled declaration and returns the verdicts of one rule.
func verdictsFor(t *testing.T, decl Declaration, m check.Model, rule string) []check.Verdict {
	t.Helper()
	var out []check.Verdict
	for _, r := range Compile(decl) {
		if r.Name == rule {
			out = append(out, r.Eval(m)...)
		}
	}
	return out
}

// An intent rule's considered set is the DECLARATION, which is what makes these the conversions worth
// having: the rule already knows exactly what it was asked to look for, so it can say "both declared
// modules are here" where before it said nothing, which is what a declaration nobody wrote says too.
func TestIntentRuleStatesWhatItWasAskedToLookFor(t *testing.T) {
	decl := declOf(t, `
name: I
modules:
  - {name: SoC, class: soc}
  - {name: CAN transceiver, class: can_transceiver}
`)
	present := check.NewModel(&ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"soc"}},
		{RefDes: "U2", DeviceClasses: []string{"can_transceiver"}},
	}})
	vs := verdictsFor(t, decl, present, RuleModuleMissing)
	if len(vs) != 2 {
		t.Fatalf("both declared modules are subjects, want 2 verdicts, got %d", len(vs))
	}
	for _, v := range vs {
		if v.Outcome != check.Pass {
			t.Errorf("%s: want pass on a design carrying it, got %s", check.SubjectRefs(v), v.Outcome)
		}
		if v.Witness == nil || v.Witness.Statement == "" {
			t.Errorf("%s: a pass with no witness is the silence this conversion removes", check.SubjectRefs(v))
		}
	}
	// And the findings do not move: the failing half is what `check` has always reported.
	absent := check.NewModel(&ir.Design{Components: []*ir.Component{{RefDes: "U1", DeviceClasses: []string{"soc"}}}})
	if fs := check.Run(absent, Compile(decl)); len(fs) != 1 {
		t.Errorf("want the one absent-module finding, got %d: %+v", len(fs), fs)
	}
}

// A module declaring no COUNT is not a subject of the count rule. Reporting it as a pass would claim
// the design has the right number of something nobody stated a number for, which is the scope mistake
// fet-vdss shipped and PR 405 fixed.
func TestIntentCountRuleClaimsOnlyModulesThatDeclareOne(t *testing.T) {
	decl := declOf(t, `
name: I
modules:
  - {name: SoC, class: soc}
  - {name: CAN transceiver, class: can_transceiver, count: 2}
`)
	m := check.NewModel(&ir.Design{Components: []*ir.Component{
		{RefDes: "U1", DeviceClasses: []string{"soc"}},
		{RefDes: "U2", DeviceClasses: []string{"can_transceiver"}},
		{RefDes: "U3", DeviceClasses: []string{"can_transceiver"}},
	}})
	vs := verdictsFor(t, decl, m, RuleModuleCount)
	if len(vs) != 1 {
		t.Fatalf("only the module declaring a count is a subject, want 1 verdict, got %d: %v", len(vs), vs)
	}
	if check.SubjectRefs(vs[0]) != "CAN transceiver" || vs[0].Outcome != check.Pass {
		t.Errorf("verdict = %s/%s, want a pass on the counted module", check.SubjectRefs(vs[0]), vs[0].Outcome)
	}
}

// A rail whose NAME states no voltage is neither a match nor a mismatch. The rule compares a
// name-derived nominal against the declared one, so a rail called VBUS supplies nothing to compare,
// and it took the same silent path a correctly-named rail took: a domain declared over rails nobody
// named for their voltage reported total agreement.
func TestVoltageDomainSeparatesAgreementFromNothingToCompare(t *testing.T) {
	decl := declOf(t, `
name: I
voltage_domains:
  - {name: core, nominal: 3.3, rails: [PMIC_CORE_3V3, VBUS, MISSING_RAIL]}
`)
	m := check.NewModel(&ir.Design{Nets: []*ir.Net{
		{Name: "PMIC_CORE_3V3", Prov: &ir.Provenance{SourceFile: "t"}},
		{Name: "VBUS", Prov: &ir.Provenance{SourceFile: "t"}},
	}})
	got := map[string]check.Outcome{}
	for _, v := range verdictsFor(t, decl, m, RuleVoltageDomain) {
		got[check.SubjectRefs(v)] = v.Outcome
	}
	want := map[string]check.Outcome{
		"PMIC_CORE_3V3": check.Pass,
		"VBUS":          check.NotConsidered,
		"MISSING_RAIL":  check.Fail,
	}
	for rail, w := range want {
		if got[rail] != w {
			t.Errorf("rail %s: want %s, got %s", rail, w, got[rail])
		}
	}
}
