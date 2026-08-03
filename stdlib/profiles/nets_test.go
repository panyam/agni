package profiles

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Nets scopes by host when the interface declares one and the host is present (precise, disambiguates
// shared suffixes), else by signal suffix.
func TestNetsHostBeatsSuffix(t *testing.T) {
	hosted := Profile{
		Name: "LINX", HostAttrKey: "interface", HostAttrVal: "LINX",
		Signals: []Signal{{Name: "TXD", Suffix: "_TX"}, {Name: "RXD", Suffix: "_RX"}},
	}
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U9", Attributes: map[string]string{"interface": "LINX"}}},
		Nets: []*ir.Net{
			{Name: "LIN_A_TX", Connections: []*ir.Connection{{ComponentRef: "U9", PinRef: "1"}}},
			{Name: "CAN_B_TX", Connections: []*ir.Connection{{ComponentRef: "U2", PinRef: "1"}}}, // shares _TX, not on host
		},
	}
	got := Nets(check.NewModel(d), hosted)
	if !got["LIN_A_TX"] || got["CAN_B_TX"] {
		t.Errorf("host-scoped Nets should be the host's nets only, got %v", got)
	}

	// No host on the design: fall back to suffix, which matches both _TX nets.
	conv := Profile{Name: "S", Signals: []Signal{{Name: "TXD", Suffix: "_TX"}}}
	got2 := Nets(check.NewModel(d), conv)
	if !got2["LIN_A_TX"] || !got2["CAN_B_TX"] {
		t.Errorf("suffix Nets should match every _TX net, got %v", got2)
	}
}
