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
	if f := got["can-termination-missing"]; f.Subject != "CAN_CANH" {
		t.Errorf("termination-missing: want CAN_CANH, got %+v", f)
	}
	if f := got["can-signal-missing"]; f.Subject != "CAN_CANH" || !strings.Contains(f.Message, "RXD") {
		t.Errorf("signal-missing: want anchor CAN_CANH + RXD, got %+v", f)
	}
	if f := got["can-signal-dangling"]; f.Subject != "CAN_TXD" {
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

// Compile produces the four CAN rules (signal-missing, host-incomplete, termination-missing,
// signal-dangling) from the declared Requirements list, registered under "profile".
func TestCANCompileAndRegistered(t *testing.T) {
	if got := len(Compile(CAN)); got != 4 {
		t.Fatalf("Compile(CAN): want 4 rules, got %d", got)
	}
	found := 0
	for _, r := range check.DefaultCatalog().Rules() {
		if strings.HasPrefix(r.Name, "profile/can-") {
			found++
		}
	}
	if found != 4 {
		t.Fatalf(`want 4 "profile/can-*" rules in DefaultCatalog, got %d`, found)
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
		if f.Rule == "can-host-incomplete" && f.Subject == "U2" {
			got++
		}
	}
	if got != 4 {
		t.Fatalf("wholly-absent bus: want 4 host-incomplete findings, got %d", got)
	}
}
