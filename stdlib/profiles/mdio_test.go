package profiles

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// mdioGood: MDIO and MDC both wired, and R1 (a resistor — the pull-up walk crosses R-prefixed 2-net
// parts) bridges ETH_MDIO to the +3V3 rail. MDC deliberately carries NO pull-up, which is how a
// correct board is built, so a silent run here is the profile's central claim.
func mdioGood() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2", "R1"),
		Nets: []*ir.Net{
			net("ETH_MDIO", "U1.1", "U2.1", "R1.1"),
			net("+3V3", "R1.2", "U1.9"),
			net("ETH_MDC", "U1.2", "U2.2"),
		},
	}
}

// mdioBroken: no resistor anywhere, so MDIO reaches no rail.
func mdioBroken() *ir.Design {
	return &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("ETH_MDIO", "U1.1", "U2.1"),
			net("ETH_MDC", "U1.2", "U2.2"),
		},
	}
}

func TestMDIOSilentOnAGoodBus(t *testing.T) {
	if fs := check.Run(check.NewModel(mdioGood()), Compile(MDIO)); len(fs) != 0 {
		t.Fatalf("good MDIO bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}

// The gap issue 516 opened: a management bus with no pull-up reported nothing at all.
func TestMDIOMissingPullUpFires(t *testing.T) {
	var got []check.Finding
	for _, f := range check.Run(check.NewModel(mdioBroken()), Compile(MDIO)) {
		if f.Rule == "mdio-missing-pullup" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("want exactly 1 missing-pullup finding, got %d: %+v", len(got), got)
	}
	if ref := check.EntityRef(got[0].Subject); ref != "ETH_MDIO" {
		t.Errorf("missing-pullup: want ETH_MDIO, got %q", ref)
	}
}

// MDC is a driven clock, not an open-drain line, so requiring a pull-up on it would fail every
// correctly-built board. This is the assertion that keeps the profile from copying the four-signal
// treatment other tools give this bus. Re-check it by adding `pullup: true` to MDC in the yaml.
func TestMDCIsNotRequiredToHaveAPullUp(t *testing.T) {
	for _, f := range check.Run(check.NewModel(mdioBroken()), Compile(MDIO)) {
		if f.Rule == "mdio-missing-pullup" && check.EntityRef(f.Subject) == "ETH_MDC" {
			t.Errorf("MDC must not require a pull-up: %+v", f)
		}
	}
}

// A clock whose name merely opens with MDC is not this bus. Same trap the built-in I2C rule guards
// with SPI_SCLK, and the reason both patterns bound their match at a token boundary.
func TestMDIODoesNotMatchMDCLK(t *testing.T) {
	d := &ir.Design{
		Components: comps("U1", "U2"),
		Nets: []*ir.Net{
			net("MDCLK", "U1.1", "U2.1"),
			net("SPI_MDIOX", "U1.2", "U2.2"),
		},
	}
	if fs := check.Run(check.NewModel(d), Compile(MDIO)); len(fs) != 0 {
		t.Fatalf("MDCLK/MDIOX are not an MDIO bus: want 0 findings, got %d: %+v", len(fs), fs)
	}
}
