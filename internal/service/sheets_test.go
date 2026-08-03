package service

import (
	"testing"

	"github.com/panyam/agni/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestAnnotateBusLocateReason checks the WS7-042c annotation: a bus finding whose bus is drawn gets
// its sheet(s) and stays UNSPECIFIED (so it highlights), while a bus finding with no drawn bus of
// that name gets BUS_NOT_DRAWN — the server-authoritative reason the viewer shows instead of
// silently doing nothing.
func TestAnnotateBusLocateReason(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id: "root",
		Wires: []*geom.WireGeometry{
			{Kind: geom.WireGeometry_KIND_BUS, Net: "DATA[7:0]", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}},
		},
	}}}
	drawn := &webapi.Finding{Rule: "bus-not-modeled", Subject: &webapi.Subject{Kind: check.KindBus, Ref: "DATA[7:0]"}}
	undrawn := &webapi.Finding{Rule: "bus-not-modeled", Subject: &webapi.Subject{Kind: check.KindBus, Ref: "ADDR"}}

	AnnotateSheets([]*webapi.Finding{drawn, undrawn}, g, nil)

	if len(drawn.GetSheets()) == 0 {
		t.Error("drawn bus got no sheet badge")
	}
	if drawn.GetLocateReason() != webapi.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("drawn bus reason = %v, want UNSPECIFIED (it highlights)", drawn.GetLocateReason())
	}
	if len(undrawn.GetSheets()) != 0 {
		t.Errorf("undrawn bus got sheets %v, want none", undrawn.GetSheets())
	}
	if undrawn.GetLocateReason() != webapi.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN {
		t.Errorf("undrawn bus reason = %v, want BUS_NOT_DRAWN", undrawn.GetLocateReason())
	}

	// A non-bus finding never gets the bus reason, even with no sheets.
	comp := &webapi.Finding{Subject: &webapi.Subject{Kind: check.KindComponent, Ref: "R99"}}
	AnnotateSheets([]*webapi.Finding{comp}, g, nil)
	if comp.GetLocateReason() != webapi.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("component reason = %v, want UNSPECIFIED", comp.GetLocateReason())
	}
}
