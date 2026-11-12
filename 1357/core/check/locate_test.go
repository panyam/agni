package check

import (
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

func TestLocateReason(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "#PWR02"}},
		Nets: []*ir.Net{
			{Name: "SDA", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}}},
			{Name: "GND"},
			{Name: "V_SENSE", Attributes: map[string]string{netgraph.AttrPowerDriven: "true"}},
		},
	}
	m := NewModel(d)
	cases := []struct {
		kind, subject, want string
	}{
		{KindComponent, "R1", LocatableNormal},     // a real placed part
		{KindComponent, "#PWR02", LocateVirtual},   // a virtual power symbol
		{KindComponent, "NOPE", LocateNotInDesign}, // unknown ref
		{KindNet, "SDA", LocatableNormal},          // a normal signal net
		{KindNet, "GND", LocatePowerRail},          // ground rail by name
		{KindNet, "V_SENSE", LocatePowerRail},      // rail by the power-driven attribute
		{KindNet, "NOSUCH", LocateNotInDesign},     // unknown net
		{KindPin, "R1", LocatableNormal},           // an unhandled kind is never a reason
	}
	for _, tc := range cases {
		if got := LocateReason(m, tc.kind, tc.subject); got != tc.want {
			t.Errorf("LocateReason(%s, %q) = %q, want %q", tc.kind, tc.subject, got, tc.want)
		}
	}
}
