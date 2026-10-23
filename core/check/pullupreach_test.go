package check

import (
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// fourHopDesign is SDA reaching +3V3 through four series resistors, one crossing past the default.
func fourHopDesign() (*ir.Design, *ir.Net) {
	conn := func(r, p string) *ir.Connection { return &ir.Connection{ComponentRef: r, PinRef: p} }
	net := func(name string, cs ...*ir.Connection) *ir.Net {
		return &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}, Connections: cs}
	}
	var comps []*ir.Component
	for _, r := range []string{"U1", "R1", "R2", "R3", "R4"} {
		comps = append(comps, &ir.Component{RefDes: r, Prov: &ir.Provenance{SourceFile: "t"}})
	}
	sda := net("SDA", conn("U1", "1"), conn("R1", "1"))
	return &ir.Design{Components: comps, Nets: []*ir.Net{
		sda,
		net("HOP1", conn("R1", "2"), conn("R2", "1")),
		net("HOP2", conn("R2", "2"), conn("R3", "1")),
		net("HOP3", conn("R3", "2"), conn("R4", "1")),
		net("+3V3", conn("R4", "2")),
	}}, sda
}

func reachIntent(net string, hops int32) *configpb.DesignIntent {
	return &configpb.DesignIntent{Nets: map[string]*configpb.NetIntent{net: {PullUpReachHops: hops}}}
}

// Every walk to a pull-up reads the limit off the model, so a declaration reaches the boolean the
// Spec twin and the coverage panel ask, the single path, and the full enumeration alike (agni 820).
func TestEveryPullUpWalkAsksTheDeclaredReach(t *testing.T) {
	d, sda := fourHopDesign()
	spec := specFuncs["pullup_reaches_rail"].Fn
	for _, tc := range []struct {
		name  string
		model Model
		limit int
		held  bool
	}{
		{"no intent", NewModel(d), PullUpReachHops, false},
		{"another net declared", NewModel(d, WithIntent(reachIntent("SCL", 4))), PullUpReachHops, false},
		{"declared 4", NewModel(d, WithIntent(reachIntent("SDA", 4))), 4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model
			if got := m.PullUpReachLimit("SDA"); got != tc.limit {
				t.Errorf("PullUpReachLimit = %d, want %d", got, tc.limit)
			}
			if got := PullUpReachesRail(m, sda); got != tc.held {
				t.Errorf("PullUpReachesRail = %v, want %v", got, tc.held)
			}
			if got := spec(m, map[string]any{"net": sda}, nil); got != tc.held {
				t.Errorf("the Spec twin pullup_reaches_rail = %v, want %v", got, tc.held)
			}
			if got := len(PullUpPathToRail(m, sda)); (got == 4) != tc.held {
				t.Errorf("PullUpPathToRail found %d hops, want 4 only when held", got)
			}
			if got := len(AllPullUpPathsToRail(m, sda)); (got == 1) != tc.held {
				t.Errorf("AllPullUpPathsToRail found %d paths, want 1 only when held", got)
			}
		})
	}
}

// A negative reach never reaches the model through the loader, which refuses it, but a DesignIntent
// can arrive on the wire unvalidated, so the model reads anything below 1 as no declaration.
func TestANonPositiveReachKeepsTheDefault(t *testing.T) {
	d, _ := fourHopDesign()
	for _, h := range []int32{0, -2} {
		if got := NewModel(d, WithIntent(reachIntent("SDA", h))).PullUpReachLimit("SDA"); got != PullUpReachHops {
			t.Errorf("reach %d: PullUpReachLimit = %d, want the default %d", h, got, PullUpReachHops)
		}
	}
}
