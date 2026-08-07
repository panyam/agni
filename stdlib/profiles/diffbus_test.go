package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Each differential-bus profile compiles to its three rules (signal-missing, host-incomplete,
// signal-dangling) and registers under "profile".
func TestDiffBusRegistered(t *testing.T) {
	for _, c := range []struct {
		prof Profile
		ns   string
	}{{A2B, "profile/a2b-"}, {PCIE, "profile/pcie-"}, {SGMII, "profile/sgmii-"}} {
		if got := len(Compile(c.prof)); got != 4 {
			t.Errorf("%s Compile: want 4 rules, got %d", c.ns, got)
		}
		n := 0
		for _, r := range check.DefaultCatalog().Rules() {
			if strings.HasPrefix(r.Name, c.ns) {
				n++
			}
		}
		if n != 4 {
			t.Errorf("want 4 %s* rules in catalog, got %d", c.ns, n)
		}
	}
}

func fires(t *testing.T, p Profile, d *ir.Design, want ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, f := range check.Run(check.NewModel(d), Compile(p)) {
		got[f.Rule] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("%s: want %q to fire, got %v", p.Name, w, got)
		}
	}
}

func silent(t *testing.T, p Profile, d *ir.Design) {
	t.Helper()
	if fs := check.Run(check.NewModel(d), Compile(p)); len(fs) != 0 {
		t.Fatalf("good %s bus: want 0 findings, got %d: %+v", p.Name, len(fs), fs)
	}
}

// SGMII: good silent; broken fires signal-missing (RXN absent) + dangling (TXN single-pin).
func TestSGMII(t *testing.T) {
	silent(t, SGMII, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("A_TXP", "U1.1", "U2.1"), net("A_TXN", "U1.2", "U2.2"),
		net("A_RXP", "U1.3", "U2.3"), net("A_RXN", "U1.4", "U2.4"),
	}})
	fires(t, SGMII, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("A_TXP", "U1.1", "U2.1"), net("A_TXN", "U1.2"), // dangling
		net("A_RXP", "U1.3", "U2.3"), // RXN absent -> missing
	}}, "sgmii-signal-missing", "sgmii-signal-dangling")
}

// PCIe: good silent; broken fires signal-missing (PERN absent) + dangling (REFCLKN single-pin).
func TestPCIe(t *testing.T) {
	silent(t, PCIE, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("X_PETP", "U1.1", "U2.1"), net("X_PETN", "U1.2", "U2.2"),
		net("X_PERP", "U1.3", "U2.3"), net("X_PERN", "U1.4", "U2.4"),
		net("X_REFCLKP", "U1.5", "U2.5"), net("X_REFCLKN", "U1.6", "U2.6"),
		net("X_PERST", "U1.7", "U2.7"),
	}})
	fires(t, PCIE, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("X_PETP", "U1.1", "U2.1"), net("X_PETN", "U1.2", "U2.2"),
		net("X_PERP", "U1.3", "U2.3"), // PERN absent -> missing
		net("X_REFCLKP", "U1.5", "U2.5"), net("X_REFCLKN", "U1.6"), // dangling
		net("X_PERST", "U1.7", "U2.7"),
	}}, "pcie-signal-missing", "pcie-signal-dangling")
}

// A2B is a 2-signal bus, so removing a signal drops the in_use gate; the meaningful breakage is a
// dangling bus leg. Good silent; broken fires signal-dangling.
func TestA2B(t *testing.T) {
	silent(t, A2B, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("X_A2B_P", "U1.1", "U2.1"), net("X_A2B_N", "U1.2", "U2.2"),
	}})
	fires(t, A2B, &ir.Design{Components: comps("U1", "U2"), Nets: []*ir.Net{
		net("X_A2B_P", "U1.1", "U2.1"), net("X_A2B_N", "U1.2"), // dangling
	}}, "a2b-signal-dangling")
}
