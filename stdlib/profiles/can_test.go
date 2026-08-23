package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// canGood: CANH/CANL/TXD/RXD all wired end-to-end, and R1 (a resistor — the reach walk crosses
// R-prefixed 2-net parts) bridges CANH↔CANL, so `reaches(CANH, CANL)` holds and the pair is
// terminated. No profile finding. (A real terminator named "RT1" is classified by its part-type
// data; this hand fixture has none, so it uses the R-prefix convention.)
func canGood() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1"),
		Nets: []*ir.Net{
			net("CAN_CANH", "U1.1", "U2.1", "R1.1"),
			net("CAN_CANL", "U1.2", "U2.2", "R1.2"),
			net("CAN_TXD", "U1.3", "U2.3"),
			net("CAN_RXD", "U1.4", "U2.4"),
		},
	}
}

// canBroken: no termination resistor (only the multi-pin U1/U2 sit on both bus nets, which `reaches`
// must NOT count), RXD net absent (missing), and TXD on a single-pin net (dangling).
func canBroken() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("CAN_CANH", "U1.1", "U2.1"),
			net("CAN_CANL", "U1.2", "U2.2"),
			net("CAN_TXD", "U1.3"),
		},
	}
}

func TestCANSilent(t *testing.T) {
	if fs := check.Run(check.NewModel(canGood()), Compile(CAN)); len(fs) != 0 {
		t.Fatalf("good CAN bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}

func TestCANFires(t *testing.T) {
	got := map[string]check.Finding{}
	for _, f := range check.Run(check.NewModel(canBroken()), Compile(CAN)) {
		got[f.Rule] = f
	}
	if len(got) != 3 {
		t.Fatalf("want 3 distinct rules firing, got %d: %+v", len(got), got)
	}
	if f := got["can-termination-missing"]; check.EntityRef(f.Subject) != "CAN_CANH" {
		t.Errorf("termination-missing: want CAN_CANH, got %+v", f)
	}
	if f := got["can-signal-missing"]; check.EntityRef(f.Subject) != "CAN_CANH" || !strings.Contains(f.Message, "RXD") {
		t.Errorf("signal-missing: want anchor CAN_CANH + RXD, got %+v", f)
	}
	if f := got["can-signal-dangling"]; check.EntityRef(f.Subject) != "CAN_TXD" {
		t.Errorf("signal-dangling: want CAN_TXD, got %+v", f)
	}
}

// The termination check must not count the transceiver: U1/U2 sit on both CANH and CANL, but with no
// resistor bridging the pair the bus is unterminated. Guards the reaches-not-component-on-both design.
func TestCANTransceiverIsNotTermination(t *testing.T) {
	fired := false
	for _, f := range check.Run(check.NewModel(canBroken()), Compile(CAN)) {
		if f.Rule == "can-termination-missing" {
			fired = true
		}
	}
	if !fired {
		t.Fatal("termination-missing must fire when only the multi-pin transceiver bridges the pair (no resistor)")
	}
}

// Compile produces the five CAN rules (signal-missing, host-incomplete, termination-missing,
// signal-dangling, esd) from the declared Requirements list, registered under "profile".
func TestCANCompileAndRegistered(t *testing.T) {
	if got := len(Compile(CAN)); got != 5 {
		t.Fatalf("Compile(CAN): want 5 rules, got %d", got)
	}
	found := 0
	for _, r := range check.DefaultCatalog().Rules() {
		if strings.HasPrefix(r.Name, "profile/can-") {
			found++
		}
	}
	if found != 5 {
		t.Fatalf(`want 5 "profile/can-*" rules in DefaultCatalog, got %d`, found)
	}
}

func canHost(ref string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Attributes: map[string]string{"interface": "CAN"}}
}

// A component declaring interface=CAN wired to none of its signals: host-anchored completeness flags
// every one of the four required signals (wholly-absent detection via the declared host).
func TestCANHostWhollyAbsent(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1"), canHost("U2")),
		Nets:       []*ir.Net{net("GND", "U2.9", "U1.9")},
	}
	got := 0
	for _, f := range check.Run(check.NewModel(d), Compile(CAN)) {
		if f.Rule == "can-host-incomplete" && check.EntityRef(f.Subject) == "U2" {
			got++
		}
	}
	if got != 4 {
		t.Fatalf("wholly-absent bus: want 4 host-incomplete findings, got %d", got)
	}
}

// TestCANHostVerdictsAreSeparatelyAddressable is the reason host-incomplete carries a subject tuple
// (agni issue 424). The same wholly-absent fixture as above produces FOUR answers about U2, and
// before the signal joined the tuple all four had the id `can-host-incomplete:(component:U2)`. A
// reader following that link reached whichever row the report wrote last, and a consumer indexing by
// id silently kept one of four.
//
// The finding count above is what makes this worth pinning separately: the rule already emitted four
// violations, so nothing in the findings contract would have shown the collision.
func TestCANHostVerdictsAreSeparatelyAddressable(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1"), canHost("U2")),
		Nets:       []*ir.Net{net("GND", "U2.9", "U1.9")},
	}
	ids := map[string]int{}
	signals := map[string]bool{}
	fails := 0
	for _, v := range check.RunVerdicts(check.NewModel(d), Compile(CAN)) {
		if v.Rule != "can-host-incomplete" {
			continue
		}
		ids[check.VerdictID(v)]++
		if len(v.Subjects) != 2 {
			t.Fatalf("want a (component, signal) tuple, got %+v", v.Subjects)
		}
		if k := v.Subjects[1].Kind; k != check.KindSignal {
			t.Errorf("tuple element 2 kind = %q, want %q", k, check.KindSignal)
		}
		signals[v.Subjects[1].Ref] = true
		if v.Outcome == check.Fail {
			fails++
		}
	}
	if fails != 4 {
		t.Fatalf("wholly-absent bus: want 4 failing verdicts, got %d", fails)
	}
	if len(ids) != 4 {
		t.Fatalf("want 4 distinct verdict ids, got %d: %v", len(ids), ids)
	}
	// Named, not merely counted: an implementation that put the same signal in every tuple would
	// still produce four rows and would fail here.
	for _, want := range []string{"CANH", "CANL", "TXD", "RXD"} {
		if !signals[want] {
			t.Errorf("no verdict names required signal %q", want)
		}
	}
}

// TestCANGoodBoardPassesAreWitnessed: the good fixture reports no findings, and the point of the
// considered set is that this silence is now backed by verdicts rather than being indistinguishable
// from a rule that never ran. Every pass must carry a witness.
func TestCANGoodBoardPassesAreWitnessed(t *testing.T) {
	vs := check.RunVerdicts(check.NewModel(canGood()), Compile(CAN))
	passes := 0
	for _, v := range vs {
		if v.Outcome != check.Pass {
			continue
		}
		passes++
		if v.Witness == nil || v.Witness.Statement == "" {
			t.Errorf("%s: a pass with no witness is the silence this work removes", check.VerdictID(v))
		}
	}
	if passes == 0 {
		t.Fatal("a fully-wired CAN bus must produce passing verdicts, not silence")
	}
}
