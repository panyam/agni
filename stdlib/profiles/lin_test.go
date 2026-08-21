package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// linGood: LIN/TXD/RXD wired, and the LIN bus pulled up to VBAT through R1 (a resistor the reach walk
// crosses; VBAT is a recognized rail). No profile finding.
func linGood() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1"),
		Nets: []*ir.Net{
			net("BUS_LIN", "U1.1", "U2.1", "R1.1"),
			net("VBAT", "R1.2", "U2.9"),
			net("LIN_TXD", "U1.2", "U2.2"),
			net("LIN_RXD", "U1.3", "U2.3"),
		},
	}
}

// linBroken: RXD net absent (missing), the LIN bus reaches no rail (missing-pullup), and TXD on a
// single-pin net (dangling).
func linBroken() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("BUS_LIN", "U1.1", "U2.1"),
			net("LIN_TXD", "U1.2"),
		},
	}
}

func TestLINSilent(t *testing.T) {
	if fs := check.Run(check.NewModel(linGood()), Compile(LIN)); len(fs) != 0 {
		t.Fatalf("good LIN bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}

func TestLINFires(t *testing.T) {
	got := map[string]check.Finding{}
	for _, f := range check.Run(check.NewModel(linBroken()), Compile(LIN)) {
		got[f.Rule] = f
	}
	if len(got) != 3 {
		t.Fatalf("want 3 distinct rules firing, got %d: %+v", len(got), got)
	}
	if f := got["lin-signal-missing"]; check.EntityRef(f.Subject) != "BUS_LIN" || !strings.Contains(f.Message, "RXD") {
		t.Errorf("signal-missing: want anchor BUS_LIN + RXD, got %+v", f)
	}
	if f := got["lin-missing-pullup"]; check.EntityRef(f.Subject) != "BUS_LIN" {
		t.Errorf("missing-pullup: want BUS_LIN, got %+v", f)
	}
	if f := got["lin-signal-dangling"]; check.EntityRef(f.Subject) != "LIN_TXD" {
		t.Errorf("signal-dangling: want LIN_TXD, got %+v", f)
	}
}

// Compile produces the five LIN rules, registered under "profile".
func TestLINCompileAndRegistered(t *testing.T) {
	if got := len(Compile(LIN)); got != 5 {
		t.Fatalf("Compile(LIN): want 5 rules, got %d", got)
	}
	found := 0
	for _, r := range check.DefaultCatalog().Rules() {
		if strings.HasPrefix(r.Name, "profile/lin-") {
			found++
		}
	}
	if found != 5 {
		t.Fatalf(`want 5 "profile/lin-*" rules in DefaultCatalog, got %d`, found)
	}
}

func linHost(ref string) *ir.Component {
	return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
		Attributes: map[string]string{"interface": "LIN"}}
}

// A component declaring interface=LIN wired to none of its signals: host-anchored completeness flags
// every one of the three required signals.
func TestLINHostWhollyAbsent(t *testing.T) {
	d := &ir.Design{
		Components: append(comps("U1"), linHost("U2")),
		Nets:       []*ir.Net{net("GND", "U2.9", "U1.9")},
	}
	got := 0
	for _, f := range check.Run(check.NewModel(d), Compile(LIN)) {
		if f.Rule == "lin-host-incomplete" && check.EntityRef(f.Subject) == "U2" {
			got++
		}
	}
	if got != 3 {
		t.Fatalf("wholly-absent bus: want 3 host-incomplete findings, got %d", got)
	}
}
