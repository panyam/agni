package builtin

import (
	"strings"
	"testing"

	"strconv"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func ncDef(name string, priority int, isDefault bool, params map[string]string) *ir.Constraint {
	p := map[string]string{"priority": strconv.Itoa(priority)}
	if isDefault {
		p["is_default"] = "true"
	}
	for k, v := range params {
		p[k] = v
	}
	return &ir.Constraint{Name: name, Kind: "netclass", Params: p}
}

// ncCopper builds a board sidecar: one entry per net, with a track width and/or a via drill in mm.
// Zero means "no copper of that kind on this net".
func ncCopper(entries ...ncNet) *geom.BoardGeometry {
	bg := &geom.BoardGeometry{UnitNm: 1}
	for _, e := range entries {
		nc := &geom.NetCopper{Net: e.net}
		if e.widthMM > 0 {
			nc.Segments = []*geom.TrackSegment{{
				A: &geom.Point{X: 0, Y: 0}, B: &geom.Point{X: 1e6, Y: 0},
				Width: int64(e.widthMM * 1e6), Layer: "F.Cu",
			}}
		}
		if e.drillMM > 0 {
			nc.Vias = []*geom.Via{{At: &geom.Point{X: 0, Y: 0}, Size: int64((e.drillMM + 0.2) * 1e6), Drill: int64(e.drillMM * 1e6)}}
		}
		bg.Nets = append(bg.Nets, nc)
	}
	return bg
}

type ncNet struct {
	net     string
	widthMM float64
	drillMM float64
}

// ncModel builds a board-bearing model: nets with class membership, class definitions, and copper.
func ncModel(t *testing.T, nets []*ir.Net, defs []*ir.Constraint, bg *geom.BoardGeometry) check.Model {
	t.Helper()
	return check.NewModelWithBoard(&ir.Design{Nets: nets, Constraints: defs}, bg)
}

// TestNetclassTrackWidthCascade is the regression that motivated the design. VBUS is in HighSpeed
// (priority 1, declaring only a clearance) and Power (priority 5, declaring 0.8mm). It is routed at
// 0.8mm, which is exactly what the winning class asks for. A naive per-class comparison would fan
// out over both classes and FAIL it against Default's 0.25mm or against a class that lost — this
// asserts it stays silent.
func TestNetclassTrackWidthCascade(t *testing.T) {
	defs := []*ir.Constraint{
		ncDef("HighSpeed", 1, false, map[string]string{"clearance": "0.15"}),
		ncDef("Power", 5, false, map[string]string{"track_width": "0.8"}),
		ncDef("Default", 2147483647, true, map[string]string{"track_width": "0.25"}),
	}
	nets := []*ir.Net{
		{Name: "VBUS", NetClasses: []string{"HighSpeed", "Power"}},
		{Name: "SIG"},
	}
	m := ncModel(t, nets, defs, ncCopper(
		ncNet{net: "VBUS", widthMM: 0.8}, // obeys Power, the class that wins the cascade
		ncNet{net: "SIG", widthMM: 0.25}, // obeys Default
	))
	if f := netclassTrackWidth.Findings(m); len(f) != 0 {
		t.Errorf("conforming nets produced findings: %+v", f)
	}

	// Now route VBUS below its resolved limit. It must fire, and name the class that set it.
	m2 := ncModel(t, nets, defs, ncCopper(ncNet{net: "VBUS", widthMM: 0.3})) // 0.3 < Power's 0.8
	f := netclassTrackWidth.Findings(m2)
	if len(f) != 1 {
		t.Fatalf("under-width net = %+v, want exactly 1 finding", f)
	}
	if f[0].Subject != "VBUS" || !strings.Contains(f[0].Message, "Power") {
		t.Errorf("finding = %+v, want subject VBUS and the class Power named as the limit's source", f[0])
	}
}

// TestNetclassTrackWidthDefaultAppliesToUnclassedNet: the Default class is not just the lowest
// priority, it applies to nets in NO class. A memberships-only cascade would skip this net entirely
// and report clean over a genuine violation.
func TestNetclassTrackWidthDefaultAppliesToUnclassedNet(t *testing.T) {
	defs := []*ir.Constraint{ncDef("Default", 2147483647, true, map[string]string{"track_width": "0.25"})}
	m := ncModel(t, []*ir.Net{{Name: "LONELY"}}, defs, ncCopper(ncNet{net: "LONELY", widthMM: 0.1}))
	f := netclassTrackWidth.Findings(m)
	if len(f) != 1 {
		t.Fatalf("unclassed net under Default's width = %+v, want 1 finding", f)
	}
}

// TestNetclassRulesSilentWithoutDefinitions: no definitions means no limit, so both rules stay
// silent rather than inventing one. The capability gate is what reports this honestly to a review;
// this asserts the rule's own internal guard, which protects a direct caller of Eval.
//
// The VBUS copper here is absurdly thin (1µm), so a rule that invented a limit would certainly fire.
// That is the positive control: "no findings" from a rule that could not fire either way would prove
// nothing.
func TestNetclassRulesSilentWithoutDefinitions(t *testing.T) {
	m := ncModel(t, []*ir.Net{{Name: "VBUS", NetClasses: []string{"Power"}}}, nil,
		ncCopper(ncNet{net: "VBUS", widthMM: 0.001, drillMM: 0.001}))
	if f := netclassTrackWidth.Findings(m); len(f) != 0 {
		t.Errorf("track-width rule fired with no definitions: %+v", f)
	}
	if f := netclassViaDrill.Findings(m); len(f) != 0 {
		t.Errorf("via-drill rule fired with no definitions: %+v", f)
	}
	// And silence is not the whole answer. A project that declared no limit has to be
	// distinguishable from one whose copper clears the limit it declared, which is what the rules
	// could not say before they stated a considered set (agni issue 391).
	for _, r := range []*check.Rule{netclassTrackWidth, netclassViaDrill} {
		vs := r.Eval(m)
		if len(vs) != 1 {
			t.Fatalf("%s: want one verdict about VBUS, got %d", r.Name, len(vs))
		}
		if vs[0].Outcome != check.NoLimit {
			t.Errorf("%s: VBUS verdict is %q; a net no class constrains reached the comparison and "+
				"found no bound, which is NoLimit and must not read as a pass", r.Name, vs[0].Outcome)
		}
	}
}

// TestNetclassViaDrill: same cascade, the other quantity, and a net whose classes state no drill at
// all must be skipped rather than compared against nothing.
func TestNetclassViaDrill(t *testing.T) {
	defs := []*ir.Constraint{
		ncDef("Power", 5, false, map[string]string{"via_drill": "0.4"}),
		ncDef("Default", 2147483647, true, map[string]string{"track_width": "0.25"}), // states NO drill
	}
	m := ncModel(t, []*ir.Net{
		{Name: "VBUS", NetClasses: []string{"Power"}},
		{Name: "SIG"},
	}, defs, ncCopper(
		ncNet{net: "VBUS", drillMM: 0.3}, // < Power's 0.4mm -> fires
		ncNet{net: "SIG", drillMM: 0.1},  // nothing declares a drill -> skipped, not passed
	))
	f := netclassViaDrill.Findings(m)
	if len(f) != 1 || f[0].Subject != "VBUS" {
		t.Fatalf("findings = %+v, want exactly one on VBUS (SIG has no declared drill to compare)", f)
	}
	// SIG is skipped rather than passed, and the verdict is where that distinction now lives. A
	// pass here would state that SIG's 0.1mm drill clears a limit, when no class states one.
	for _, v := range netclassViaDrill.Eval(m) {
		if v.Subject != "SIG" {
			continue
		}
		if v.Outcome != check.NoLimit {
			t.Errorf("SIG verdict is %q; nothing declares a drill for it, so nothing was compared", v.Outcome)
		}
	}
}

// TestNetclassTrackWidthPriorityDecides exercises the ORDER, which the cascade test above cannot:
// there, only one of the net's classes stated a track width, so any ordering gave the same answer.
// Here BOTH state one and they disagree, so the winner is decided purely by priority. Routed at the
// high-priority class's value, the net must be silent; a rule that took the wrong class would fire.
func TestNetclassTrackWidthPriorityDecides(t *testing.T) {
	defs := []*ir.Constraint{
		// Names chosen so ALPHABETICAL order is the REVERSE of priority order. Membership arrives
		// alphabetically sorted (WS1-050), so a cascade that forgot to sort by priority would use the
		// alphabetically-first class and a same-order test would pass by accident.
		ncDef("Zeta", 1, false, map[string]string{"track_width": "0.15"}),  // wins on priority
		ncDef("Alpha", 9, false, map[string]string{"track_width": "0.90"}), // loses despite sorting first
		ncDef("Default", 2147483647, true, map[string]string{"track_width": "0.25"}),
	}
	nets := []*ir.Net{{Name: "SIG", NetClasses: []string{"Alpha", "Zeta"}}} // as the reader sorts them

	// 0.15mm satisfies Zeta (the winner) but is far below Alpha. Silent only if priority is honoured.
	m := ncModel(t, nets, defs, ncCopper(ncNet{net: "SIG", widthMM: 0.15}))
	if f := netclassTrackWidth.Findings(m); len(f) != 0 {
		t.Errorf("net routed at the WINNING class's width produced findings: %+v", f)
	}

	// Below Zeta's width: fires, and names Zeta rather than Alpha or Default.
	m2 := ncModel(t, nets, defs, ncCopper(ncNet{net: "SIG", widthMM: 0.10}))
	f := netclassTrackWidth.Findings(m2)
	if len(f) != 1 {
		t.Fatalf("findings = %+v, want 1", f)
	}
	if !strings.Contains(f[0].Message, "Zeta") {
		t.Errorf("finding names the wrong class: %q, want the highest-priority stating class (Zeta)", f[0].Message)
	}
}
