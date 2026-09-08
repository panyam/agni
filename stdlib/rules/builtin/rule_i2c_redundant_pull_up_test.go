package builtin

import (
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pullupCountFixture: SDA doubly pulled to one rail, SCL0 pulled to two DIFFERENT rails, SCL1 pulled
// once (the clean case), and SDA2 with no pull-up at all (i2c-pull-up's subject, not these rules').
func pullupCountFixture() *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	conns := func(specs ...string) []*ir.Connection {
		var out []*ir.Connection
		for _, s := range specs {
			for i := 0; i < len(s); i++ {
				if s[i] == '.' {
					out = append(out, &ir.Connection{ComponentRef: s[:i], PinRef: s[i+1:]})
					break
				}
			}
		}
		return out
	}
	net := func(name string, global bool, specs ...string) *ir.Net {
		n := &ir.Net{Name: name, Connections: conns(specs...), Prov: &ir.Provenance{SourceFile: "t"}}
		if global {
			n.Attributes = map[string]string{"global": "true"}
		}
		return n
	}
	return &ir.Design{
		Components: []*ir.Component{
			comp("U1"), comp("R1"), comp("R2"), comp("R3"), comp("R4"), comp("R5"),
		},
		Nets: []*ir.Net{
			net("SDA", false, "U1.1", "R1.1", "R2.1"),
			net("SCL0", false, "U1.2", "R3.1", "R4.1"),
			net("SCL1", false, "U1.3", "R5.1"),
			net("SDA2", false, "U1.4"),
			net("+3V3", true, "R1.2", "R2.2", "R3.2", "R5.2"),
			net("+1V8", true, "R4.2"),
		},
	}
}

func findingsFor(t *testing.T, rule *check.Rule, d *ir.Design) map[string]check.Finding {
	t.Helper()
	out := map[string]check.Finding{}
	for _, v := range rule.Eval(check.NewModel(d)) {
		if v.Finding != nil {
			out[check.EntityRef(v.Finding.Subject)] = *v.Finding
		}
	}
	return out
}

func subjectsFor(t *testing.T, rule *check.Rule, d *ir.Design) []string {
	t.Helper()
	var out []string
	for _, v := range rule.Eval(check.NewModel(d)) {
		out = append(out, check.EntityRef(v.Subjects[0]))
	}
	sort.Strings(out)
	return out
}

func TestRedundantPullUpFiresOnlyOnTheDoubledBus(t *testing.T) {
	got := findingsFor(t, i2cRedundantPullUp, pullupCountFixture())
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want only SDA", got)
	}
	if f, ok := got["SDA"]; !ok {
		t.Errorf("SDA was not reported: %+v", got)
	} else if f.Message != "I2C net has 2 pull-up resistors to +3V3" {
		t.Errorf("message = %q, want it to name the count and the rail", f.Message)
	}
}

// The two rules fire on disjoint conditions, so a bus is named once by the rule whose remedy applies.
func TestSplitRailAndRedundantNeverReportTheSameNet(t *testing.T) {
	d := pullupCountFixture()
	red := findingsFor(t, i2cRedundantPullUp, d)
	split := findingsFor(t, i2cPullUpSplitRail, d)
	if _, ok := split["SCL0"]; !ok {
		t.Errorf("the two-rail bus was not reported as split-rail: %+v", split)
	}
	for net := range red {
		if _, both := split[net]; both {
			t.Errorf("%s was reported by both rules", net)
		}
	}
}

func TestSplitRailMessageNamesBothRails(t *testing.T) {
	f, ok := findingsFor(t, i2cPullUpSplitRail, pullupCountFixture())["SCL0"]
	if !ok {
		t.Fatal("SCL0 was not reported")
	}
	if f.Message != "I2C net is pulled up to +1V8 and +3V3" {
		t.Errorf("message = %q, want both rails named", f.Message)
	}
}

// A count rule's considered set has to include the buses that are fine, or a clean board cannot be
// told from one nobody looked at. A net with NO pull-up is deliberately absent: that is
// i2c-pull-up's subject, and reporting one absence in three places would treble a single defect.
func TestBothRulesStateWhatTheyLookedAt(t *testing.T) {
	for _, r := range []*check.Rule{i2cRedundantPullUp, i2cPullUpSplitRail} {
		got := subjectsFor(t, r, pullupCountFixture())
		want := []string{"SCL0", "SCL1", "SDA"}
		if len(got) != len(want) {
			t.Errorf("%s considered %v, want %v (SDA2 has no pull-up and belongs to i2c-pull-up)", r.Name, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s considered %v, want %v", r.Name, got, want)
				break
			}
		}
		if !r.StatesConsideredSet {
			t.Errorf("%s does not declare its considered set, so its passes report as nothing", r.Name)
		}
	}
}

// A pass proves itself with the parts it counted, which is what makes a clean bus checkable rather
// than merely unreported.
func TestAPassNamesThePullUpItFound(t *testing.T) {
	for _, v := range i2cRedundantPullUp.Eval(check.NewModel(pullupCountFixture())) {
		if check.EntityRef(v.Subjects[0]) != "SCL1" {
			continue
		}
		if v.Outcome != check.Pass {
			t.Fatalf("SCL1 outcome = %v, want pass", v.Outcome)
		}
		if v.Witness == nil || v.Witness.Statement != "SCL1 is pulled to +3V3 by R5" {
			t.Errorf("witness = %+v, want it to name the resistor and the rail", v.Witness)
		}
		var refs []string
		for _, c := range v.Context {
			refs = append(refs, c.Role)
		}
		if len(refs) != 2 || refs[0] != "pull-up" || refs[1] != "rail" {
			t.Errorf("context roles = %v, want the part and the rail", refs)
		}
		return
	}
	t.Fatal("SCL1 produced no verdict")
}

// THE POSITIVE CONTROL. Both rules count only resistors landing on a net the naming lexicon calls a
// rail, so on a board whose supplies it does not recognise the count collapses to zero and neither
// rule can fire. That is a silent no-op, and the only thing that separates it from a clean board is
// a test that shows the same topology reporting nothing.
func TestNeitherRuleFiresWhenTheSupplyIsNotRecognisedAsARail(t *testing.T) {
	d := pullupCountFixture()
	for _, n := range d.Nets {
		if n.Name == "+3V3" {
			n.Name = "P3V3_SOMETHING" // not a rail by the built-in vocabulary
			n.Attributes = nil
			for _, other := range d.Nets {
				_ = other
			}
		}
	}
	if got := findingsFor(t, i2cRedundantPullUp, d); len(got) != 0 {
		t.Errorf("findings = %+v; expected none, since the supply is not classified as a rail", got)
	}
	// And the control on the control: with the rail recognised, the same topology DOES report.
	if got := findingsFor(t, i2cRedundantPullUp, pullupCountFixture()); len(got) == 0 {
		t.Error("the recognised-rail case reports nothing either, so the test above proves nothing")
	}
}
