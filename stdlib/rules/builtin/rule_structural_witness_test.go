package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Proof-on-pass for the three STRUCTURAL rules, one per subject kind (net, component, pin).
//
// What these assert is the property build/evidence.md asks for, not the presence of a witness. "A
// witness is non-nil" cannot fail once the field is set, and a statement that reads well but would be
// identical on a design where the rule concluded the opposite proves nothing. So each test below
// changes THE FACT and requires the witness to change with it.
//
// The second thing under test is the considered set itself: a converted rule must return a verdict
// for every subject it was applied to, including the ones it passed and the ones it could not judge.
// Before the conversion all three of those left the rule through the same silent return.

func verdictOf(t *testing.T, vs []check.Verdict, subject, pin string) check.Verdict {
	t.Helper()
	for _, v := range vs {
		if v.Subjects[0].Ref == subject && v.Subjects[0].Pin == pin {
			return v
		}
	}
	t.Fatalf("no verdict for subject %q pin %q in %d verdicts", subject, pin, len(vs))
	return check.Verdict{}
}

func outcomes(vs []check.Verdict) map[string]check.Outcome {
	out := map[string]check.Outcome{}
	for _, v := range vs {
		out[check.EntityRef(v.Subjects[0])] = v.Outcome
	}
	return out
}

// THE PROPERTY for single-pin-net: the connection count is the fact, so the witness must move with
// it. A witness that ignored the topology would say the same thing for a two-pin net as for a stub.
func TestSinglePinNetWitnessTracksTheCount(t *testing.T) {
	stub := singlePinNetVerdicts(check.NewModel(&ir.Design{
		Components: comps("U1", "R1"), Nets: []*ir.Net{tnet("SIG", "U1.1")},
	}))
	wired := singlePinNetVerdicts(check.NewModel(&ir.Design{
		Components: comps("U1", "R1"), Nets: []*ir.Net{tnet("SIG", "U1.1", "R1.1")},
	}))

	if got := verdictOf(t, stub, "SIG", "").Outcome; got != check.Fail {
		t.Errorf("a one-pin net is a stub, want fail, got %s", got)
	}
	if got := verdictOf(t, wired, "SIG", "").Outcome; got != check.Pass {
		t.Errorf("a two-pin net is not a stub, want pass, got %s", got)
	}

	sw, ww := verdictOf(t, stub, "SIG", "").Witness, verdictOf(t, wired, "SIG", "").Witness
	if sw == nil || ww == nil {
		t.Fatal("both outcomes must carry the count they rest on")
	}
	if sw.Statement == ww.Statement {
		t.Errorf("witness does not track the connection count: same statement %q for a stub and a wired net", sw.Statement)
	}
	if got := termValue(ww, "connections"); got != "2" {
		t.Errorf("wired net's witness term = %q, want 2", got)
	}
	if got := termValue(sw, "connections"); got != "1" {
		t.Errorf("stub's witness term = %q, want 1", got)
	}
}

// The no-connect exemption is a PASS with its own reason, not a silence. This is the case the
// conversion exists for: before it, a deliberate stub and a net the rule never saw were the same
// nothing downstream.
func TestIntentionalNoConnectPassesRatherThanVanishing(t *testing.T) {
	vs := singlePinNetVerdicts(check.NewModel(&ir.Design{
		Components: comps("U1"), Nets: []*ir.Net{tnet("unconnected-(U1-Pad1)", "U1.1")},
	}))
	v := verdictOf(t, vs, "unconnected-(U1-Pad1)", "")
	if v.Outcome != check.Pass {
		t.Fatalf("a marked no-connect stub is the design working as intended, want pass, got %s", v.Outcome)
	}
	if v.Witness == nil || !strings.Contains(v.Witness.Statement, "no-connect") {
		t.Errorf("the pass must say WHY it passed, got %+v", v.Witness)
	}
}

// THE PROPERTY for unconnected-component, plus the NotConsidered case. An empty ref-des is not a
// clean part: it is a part the rule cannot judge, and the two must not report the same.
func TestUnconnectedComponentSeparatesCannotJudgeFromClean(t *testing.T) {
	vs := unconnectedComponentVerdicts(check.NewModel(&ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "R9", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{tnet("SIG", "U1.1", "U1.2")},
	}))

	if len(vs) != 3 {
		t.Fatalf("every component is a subject, want 3 verdicts, got %d", len(vs))
	}
	want := map[string]check.Outcome{"U1": check.Pass, "R9": check.Fail, "": check.NotConsidered}
	for subject, w := range want {
		if got := verdictOf(t, vs, subject, "").Outcome; got != w {
			t.Errorf("component %q: want %s, got %s", subject, w, got)
		}
	}
	if r := verdictOf(t, vs, "", "").Reason; r == "" {
		t.Error("a NotConsidered verdict must say why it could not be judged")
	}
	// The count is the fact: U1 lands on one net, and it is counted once despite two pins on it.
	if got := termValue(verdictOf(t, vs, "U1", "").Witness, "nets reached"); got != "1" {
		t.Errorf("U1 reaches one net (two pins on it), witness term = %q, want 1", got)
	}
}

// THE PROPERTY for unconnected-pin: NO_CONNECT and UNSPECIFIED were both silently skipped, and they
// are different answers. One is the design working as documented; the other is a gap in the input.
func TestUnconnectedPinSeparatesDeclaredOpenFromUndeclared(t *testing.T) {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "MCU", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},       // wired -> pass
				{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},       // unwired -> fail
				{Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_NO_CONNECT},  // declared open -> pass
				{Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_UNSPECIFIED}, // undeclared -> not considered
			}},
		}}},
		Components: []*ir.Component{{
			RefDes:   "U1",
			Sections: []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}},
			Prov:     &ir.Provenance{SourceFile: "t"},
		}},
		Nets: []*ir.Net{tnet("SIG", "U1.1")},
	}
	vs := unconnectedPinVerdicts(check.NewModel(d))
	if len(vs) != 4 {
		t.Fatalf("every pin is a subject, want 4 verdicts, got %d", len(vs))
	}
	got := outcomes(vs)
	want := map[string]check.Outcome{
		"U1.1": check.Pass, "U1.2": check.Fail,
		"U1.3": check.Pass, "U1.4": check.NotConsidered,
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("pin %s: want %s, got %s", k, w, got[k])
		}
	}
	// The two passes must not be interchangeable: one is "it is wired", the other "it is declared
	// open". Collapsing them is exactly what the old skip did.
	wired, declared := verdictOf(t, vs, "U1", "1").Witness, verdictOf(t, vs, "U1", "3").Witness
	if wired == nil || declared == nil {
		t.Fatal("both passes must carry a witness")
	}
	if wired.Statement == declared.Statement {
		t.Errorf("a wired pin and a declared-open pin pass for different reasons, both said %q", wired.Statement)
	}
	if r := verdictOf(t, vs, "U1", "4").Reason; r == "" {
		t.Error("an undeclared pin's NotConsidered must say why")
	}
}

func termValue(w *check.Witness, label string) string {
	if w == nil {
		return ""
	}
	for _, tm := range w.Terms {
		if tm.Label == label {
			return tm.Value
		}
	}
	return ""
}
