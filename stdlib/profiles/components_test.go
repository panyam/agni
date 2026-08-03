package profiles

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Components returns the parts on an interface's nets, inheriting Nets' host-beats-convention scope: a
// host-declared interface resolves to the host's nets and thus every part on them (disambiguating a
// shared suffix); with no host it falls back to the suffix-matched signal nets and their parts. The
// join it enables: a component on an interface signal net is kept even though its rail net is not.
func TestComponentsHostAndSuffix(t *testing.T) {
	hosted := Profile{
		Name: "LINX", HostAttrKey: "interface", HostAttrVal: "LINX",
		Signals: []Signal{{Name: "TXD", Suffix: "_TX"}, {Name: "RXD", Suffix: "_RX"}},
	}
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U9", Attributes: map[string]string{"interface": "LINX"}},
			{RefDes: "U2"},
		},
		Nets: []*ir.Net{
			// The host U9 shares a signal net with the transceiver U1.
			{Name: "LIN_A_TX", Connections: []*ir.Connection{{ComponentRef: "U9", PinRef: "1"}, {ComponentRef: "U1", PinRef: "3"}}},
			// A shared _TX suffix on a DIFFERENT bus, not on the host.
			{Name: "CAN_B_TX", Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}},
		},
	}
	got := Components(check.NewModel(d), hosted)
	if !got["U9"] || !got["U1"] || got["U2"] {
		t.Errorf("host-scoped Components should be the parts on the host's nets (U9, U1), got %v", got)
	}

	// No host on the design: fall back to suffix, which matches both _TX nets and both their parts.
	conv := Profile{Name: "S", Signals: []Signal{{Name: "TXD", Suffix: "_TX"}}}
	got2 := Components(check.NewModel(d), conv)
	if !got2["U9"] || !got2["U1"] || !got2["U2"] {
		t.Errorf("suffix Components should match every part on a _TX net, got %v", got2)
	}
}
