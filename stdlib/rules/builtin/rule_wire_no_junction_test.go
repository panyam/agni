package builtin

import (
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// tapDesign is a read that examined wire geometry, with whatever mix of joined and silent taps the
// caller wants. Supplied is what separates a sheet with no tap on it from a format whose reader never
// looked at a wire, and without it the rule gates to not-applicable.
func tapDesign(joined []*ir.JoinedTap, silent ...*ir.DanglingEndpoint) *ir.Design {
	return &ir.Design{InputDiagnostics: &ir.InputDiagnostics{
		Supplied:            []string{"junction_taps"},
		JoinedTaps:          joined,
		NoJunctionEndpoints: silent,
	}}
}

// TestWireNoJunctionStatesConsideredSet (agni issue 420): the verdicts cover every wire-end-on-body
// tap the reader examined, and the silent ones still project to exactly the findings the rule
// reported before.
func TestWireNoJunctionStatesConsideredSet(t *testing.T) {
	if !wireNoJunction.StatesConsideredSet {
		t.Fatal("the rule must declare a considered set, or a sheet whose taps all carry dots means nothing")
	}
	d := tapDesign(
		[]*ir.JoinedTap{{X: 10, Y: 20, JoinKind: "junction", Segments: 3}},
		&ir.DanglingEndpoint{X: 30, Y: 40},
	)
	m := check.NewModel(d)
	vs := wireNoJunction.Eval(m)
	if len(vs) != 2 {
		t.Fatalf("verdicts = %d, want 2 (one per tap the reader examined)", len(vs))
	}
	byOutcome := map[check.Outcome]string{}
	for _, v := range vs {
		if len(v.Subjects) != 1 || v.Subjects[0].Kind != check.KindEndpoint {
			t.Errorf("subject = %+v, want one endpoint", v.Subjects)
			continue
		}
		byOutcome[v.Outcome] = check.EntityRef(v.Subjects[0])
	}
	if byOutcome[check.Pass] != "10,20" {
		t.Errorf("pass subject = %q, want the joined tap", byOutcome[check.Pass])
	}
	if byOutcome[check.Fail] != "30,40" {
		t.Errorf("fail subject = %q, want the silent tap", byOutcome[check.Fail])
	}
	fs := wireNoJunction.Findings(m)
	if len(fs) != 1 || check.EntityRef(fs[0].Subject) != "30,40" {
		t.Errorf("findings = %+v, want the silent tap only, unchanged by the pass verdict", fs)
	}
}

// TestWireNoJunctionPassNamesTheJoin is the witness check (build/evidence.md). "The tap is joined"
// restates the outcome and reads the same on a tap held by a junction dot somebody placed on purpose
// and one held by a label that happens to sit at the meet. Asserting the two statements DIFFER is
// what catches a witness carrying a constant; asserting the label text appears is what makes the pass
// checkable against the drawing.
func TestWireNoJunctionPassNamesTheJoin(t *testing.T) {
	statement := func(j *ir.JoinedTap) string {
		vs := wireNoJunction.Eval(check.NewModel(tapDesign([]*ir.JoinedTap{j})))
		if len(vs) != 1 || vs[0].Witness == nil {
			t.Fatalf("verdicts = %+v, want one carrying a witness", vs)
		}
		return vs[0].Witness.Statement
	}
	dot := statement(&ir.JoinedTap{X: 1, Y: 1, JoinKind: "junction", Segments: 3})
	label := statement(&ir.JoinedTap{X: 1, Y: 1, JoinKind: "label", Label: "I2C_SCL", Segments: 3})
	if dot == label {
		t.Errorf("a junction dot and a mid-span label read identically (%q), so the pass is decoration", dot)
	}
	if !strings.Contains(label, "I2C_SCL") {
		t.Errorf("label statement = %q, does not name the net the tap resolves to", label)
	}
	// The segment count separates an ordinary T from a crossing, so it must move with the data too.
	four := statement(&ir.JoinedTap{X: 1, Y: 1, JoinKind: "junction", Segments: 4})
	if four == dot {
		t.Errorf("a three-way T and a four-way crossing read identically (%q)", dot)
	}
}

// TestWireNoJunctionGatedWhenNobodyLooked: only the KiCad reader examines wire geometry. Without the
// capability the rule must report not-applicable WITH A REASON rather than running over an empty list
// and contributing a considered set of nothing, which reads as a clean sheet (agni issue 309's shape).
func TestWireNoJunctionGatedWhenNobodyLooked(t *testing.T) {
	d := tapDesign(nil)
	d.InputDiagnostics.Supplied = nil
	ok, reason := check.Available(wireNoJunction, check.NewModel(d))
	if ok {
		t.Fatal("the rule ran on a design whose reader never examined a wire, so its silence would read as a pass")
	}
	if !strings.Contains(reason, "wire geometry") {
		t.Errorf("reason = %q, does not say why nothing was checked", reason)
	}
	// And it DOES run once a reader declares it, so the gate is not simply always closed.
	if ok, _ := check.Available(wireNoJunction, check.NewModel(tapDesign(nil))); !ok {
		t.Error("the rule is gated even on a reader that declared it looked")
	}
}

// TestWireNoJunctionFailWitnessSaysWhatIsWrong: the failing side gained a witness it did not have, and
// it has to state the consequence rather than repeat the finding message, since the two sit side by
// side in the verdict output.
func TestWireNoJunctionFailWitnessSaysWhatIsWrong(t *testing.T) {
	vs := wireNoJunction.Eval(check.NewModel(tapDesign(nil, &ir.DanglingEndpoint{X: 5, Y: 6})))
	if len(vs) != 1 || vs[0].Witness == nil {
		t.Fatalf("verdicts = %+v, want one carrying a witness", vs)
	}
	if !strings.Contains(vs[0].Witness.Statement, "netlist") {
		t.Errorf("fail statement = %q, does not say the drawing and the netlist disagree", vs[0].Witness.Statement)
	}
	if vs[0].Finding == nil || !strings.Contains(vs[0].Finding.Message, "no junction dot") {
		t.Errorf("finding = %+v, want the unchanged message", vs[0].Finding)
	}
}

// TestWireNoJunctionSegmentsInTerms keeps the count machine-readable, not only in prose, since the
// CSV form is what a reviewer filters.
func TestWireNoJunctionSegmentsInTerms(t *testing.T) {
	vs := wireNoJunction.Eval(check.NewModel(tapDesign([]*ir.JoinedTap{{X: 1, Y: 1, JoinKind: "junction", Segments: 4}})))
	var got string
	for _, term := range vs[0].Witness.Terms {
		if term.Label == "wire ends" {
			got = term.Value
		}
	}
	if got != strconv.Itoa(4) {
		t.Errorf("wire ends term = %q, want 4", got)
	}
}
