package profiles

import (
	"testing"

	"github.com/panyam/agni/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestCoverageGood: a fully-wired SPI-NOR bus is detected and every signal reads present (CS is
// pulled up to +3V3 through R1).
func TestCoverageGood(t *testing.T) {
	cov := Coverage(SPINOR, check.NewModel(spinorGood()))
	if cov == nil {
		t.Fatal("SPI_NOR should be detected on the good fixture")
	}
	if cov.Profile != "SPI_NOR" || cov.Anchor != "SPI_CS" {
		t.Errorf("profile/anchor = %q/%q, want SPI_NOR/SPI_CS", cov.Profile, cov.Anchor)
	}
	if len(cov.Signals) != 6 {
		t.Fatalf("signals = %d, want 6", len(cov.Signals))
	}
	for _, s := range cov.Signals {
		if s.State != StatePresent {
			t.Errorf("%s = %s, want present", s.Name, s.State)
		}
	}
}

// TestCoverageBroken: the broken bus is still detected (five signals present), with IO2 missing,
// SCLK dangling (single-pin), and CS's pull-up missing — the exact three the profile rules fire on.
func TestCoverageBroken(t *testing.T) {
	cov := Coverage(SPINOR, check.NewModel(spinorBroken()))
	if cov == nil {
		t.Fatal("SPI_NOR should still be detected on the broken fixture")
	}
	got := map[string]SignalCoverage{}
	for _, s := range cov.Signals {
		got[s.Name] = s
	}
	want := map[string]string{
		"CS": StatePullupMissing, "SCLK": StateDangling, "IO0": StatePresent,
		"IO1": StatePresent, "IO2": StateMissing, "IO3": StatePresent,
	}
	for name, w := range want {
		if got[name].State != w {
			t.Errorf("%s = %s, want %s", name, got[name].State, w)
		}
	}
	if got["IO2"].Net != "" {
		t.Errorf("missing IO2 net = %q, want empty", got["IO2"].Net)
	}
	if got["CS"].Net != "SPI_CS" {
		t.Errorf("CS net = %q, want SPI_CS", got["CS"].Net)
	}
}

// TestCoverageUndetected: a single matching signal is below the in-use confidence gate, so the
// interface is not detected (nil) — no false coverage on a lone _CS net.
func TestCoverageUndetected(t *testing.T) {
	d := &ir.Design{Components: comps("U1"), Nets: []*ir.Net{net("SPI_CS", "U1.1")}}
	if cov := Coverage(SPINOR, check.NewModel(d)); cov != nil {
		t.Errorf("one signal should not detect the interface, got %+v", cov)
	}
}
