package check

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// subjectGateDesign has R1 lost to an unresolved symbol, with its pin drawn to net A. R2 joins A to
// B, so B is one series hop from the loss. C and D sit apart from it. supplied says whether the
// reader named the nets a lost pin may join.
func subjectGateDesign(supplied bool) *ir.Design {
	lib := &ir.PartLibrary{Name: "lib"}
	d := &ir.Design{SourceFormat: "kicad-sch", Libraries: []*ir.PartLibrary{lib}}
	for _, ref := range []string{"R1", "R2", "U1", "U3", "U4", "U5", "U6"} {
		pt := &ir.PartType{Name: ref + "_T"}
		if ref != "R1" {
			pt.Pins = []*ir.Pin{{Name: "1", Designator: "1"}, {Name: "2", Designator: "2"}}
		}
		lib.Parts = append(lib.Parts, pt)
		d.Components = append(d.Components, &ir.Component{RefDes: ref, Sections: []*ir.ComponentSection{{PartRef: ref + "_T", LibraryRef: "lib"}}})
	}
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	d.Nets = []*ir.Net{
		{Name: "A", Connections: []*ir.Connection{conn("U1", "1"), conn("R2", "1")}},
		{Name: "B", Connections: []*ir.Connection{conn("R2", "2"), conn("U3", "1")}},
		{Name: "C", Connections: []*ir.Connection{conn("U4", "1"), conn("U5", "1")}},
		{Name: "D", Connections: []*ir.Connection{conn("U6", "1")}},
	}
	d.InputDiagnostics = &ir.InputDiagnostics{
		UnresolvedSymbols: []*ir.UnresolvedSymbol{{Symref: "lib:Missing", RefDes: []string{"R1"}, Prov: &ir.Provenance{SourceFile: "board.kicad_sch"}}},
	}
	if supplied {
		d.InputDiagnostics.Supplied = []string{"nets_missing_pins"}
		d.InputDiagnostics.NetsMissingPins = []string{"A"}
	}
	classify.Stamp(d)
	return d
}

// subjectRule passes every subject it is handed, so any outcome other than pass is the gate's.
func subjectRule() *Rule {
	pass := func(e Entity, ctx ...ContextSubject) Verdict {
		return Verdict{Subjects: []Entity{e}, Outcome: Pass, Witness: &Witness{Statement: "fine"}, Context: ctx}
	}
	return &Rule{
		Name: "subject-rule", Severity: "error", Reads: []string{"on_net"}, StatesConsideredSet: true,
		Eval: func(context.Context, Model) []Verdict {
			return []Verdict{
				pass(NetNameEntity("A")), pass(NetNameEntity("B")), pass(NetNameEntity("C")),
				pass(ComponentEntity("U3")), pass(ComponentEntity("U5")), pass(ComponentEntity("R1")),
				pass(NetNameEntity("D"), Ctx(NetNameEntity("A"), "rail")),
			}
		},
	}
}

func outcomesBySubject(vs []Verdict) map[string]Outcome {
	out := map[string]Outcome{}
	for _, v := range vs {
		out[EntityRef(v.Subjects[0])] = v.Outcome
	}
	return out
}

// TestUnresolvedGateReachesOnlyTheSubjectsALostPinMayChange is agni issue 88. A reader that names
// the nets a lost pin may join lets the gate keep every verdict the loss cannot reach.
func TestUnresolvedGateReachesOnlyTheSubjectsALostPinMayChange(t *testing.T) {
	fs, vs, err := RunAll(context.Background(), NewModel(subjectGateDesign(true)), []*Rule{subjectRule()})
	if err != nil {
		t.Fatal(err)
	}
	got := outcomesBySubject(vs)
	if len(got) != 7 {
		t.Fatalf("verdicts = %v, want 7 subjects", got)
	}
	for subject, want := range map[string]Outcome{
		"A": Inconclusive, "B": Inconclusive, "U3": Inconclusive, "R1": Inconclusive, "D": Inconclusive,
		"C": Pass, "U5": Pass,
	} {
		if got[subject] != want {
			t.Errorf("%s = %s, want %s", subject, got[subject], want)
		}
	}
	// One finding says the rule lost subjects to the gate. The verdicts name each one.
	if len(fs) != 1 || !fs[0].Inconclusive || EntityRef(fs[0].Subject) != "lib:Missing" {
		t.Errorf("findings = %+v, want the gate's one inconclusive", fs)
	}
}

// TestUnresolvedGateStaysDesignWideWithoutAttribution covers a reader that did not name the nets a
// lost pin may join. Nothing then supports keeping any verdict, so every subject is inconclusive.
func TestUnresolvedGateStaysDesignWideWithoutAttribution(t *testing.T) {
	vs := RunVerdictsBackground(NewModel(subjectGateDesign(false)), []*Rule{subjectRule()})
	for subject, o := range outcomesBySubject(vs) {
		if o != Inconclusive {
			t.Errorf("%s = %s, want inconclusive while the reader names no nets", subject, o)
		}
	}
}

// TestUnresolvedGateStillGatesAFailuresOnlyRuleWhole covers a rule that reports only its failures. Its
// silence on a subject cannot be gated, so a lost pin still replaces its findings with one.
func TestUnresolvedGateStillGatesAFailuresOnlyRuleWhole(t *testing.T) {
	fs := RunBackground(NewModel(subjectGateDesign(true)), []*Rule{alwaysFires("on_net")})
	if len(fs) != 1 || !fs[0].Inconclusive || EntityRef(fs[0].Subject) != "lib:Missing" {
		t.Errorf("findings = %+v, want the gate's one inconclusive", fs)
	}
}
