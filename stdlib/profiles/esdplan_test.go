package profiles

import (
	"context"
	"fmt"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// tvsChain is a CAN bus that never leaves the board, beside a chain of 60 nets joined by series
// resistors with a TVS on every net. Nothing here needs ESD, so the requirement's protection walk
// has nowhere to start, while a planner that walks from the TVS parts walks the whole chain once per
// TVS.
func tvsChain() *ir.Design {
	d := &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("BUS_CANH", "U1.1", "U2.1"),
			net("BUS_CANL", "U1.2", "U2.2"),
		},
	}
	const n = 60
	for i := 0; i < n; i++ {
		tvs, r := fmt.Sprintf("TVS%d", i+1), fmt.Sprintf("R%d", i+1)
		d.Components = append(d.Components, comps(tvs, r)...)
		conns := []string{tvs + ".1", r + ".1"}
		if i > 0 {
			conns = append(conns, fmt.Sprintf("R%d.2", i))
		}
		d.Nets = append(d.Nets, net(fmt.Sprintf("CHAIN%d", i), conns...))
	}
	return d
}

// The ESD requirement walks from the nets that need protection and nowhere else (agni issue 1037).
// From jaala v0.1.57 to v0.1.59 the planner started its protection clauses from the TVS index and
// called net.reaches once per TVS net with only its second argument bound, a full walk each time:
// Jetson's ESD rules went from about 95 ms to about 1 s each (panyam/jaala#304). Under a budget a
// walk from the scope fits in easily, that plan runs out of it, and a rule that runs out reports an
// inconclusive finding rather than a clean result.
func TestESDRequirementWalksFromTheNetsThatNeedIt(t *testing.T) {
	r := esdRule(CAN, Requirement{Type: "esd"})
	ctx := query.WithBudget(context.Background(), 20_000)
	findings := r.Findings(ctx, check.NewModel(tvsChain()))
	for _, f := range findings {
		t.Errorf("finding %q on %s (inconclusive %v); want none, since no CAN line leaves the board and the walk has nowhere to start", f.Message, f.Subject.Ref, f.Inconclusive)
	}
	// The positive control: unbudgeted, the same plan is clean, so a finding above is the budget.
	if got := r.Findings(context.Background(), check.NewModel(tvsChain())); len(got) != 0 {
		t.Fatalf("unbudgeted: %d findings, want none", len(got))
	}
}
