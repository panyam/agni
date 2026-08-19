package service

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	drawn := &checkspb.Finding{Rule: "bus-not-modeled", Subject: &checkspb.Subject{Kind: check.KindBus, Ref: "DATA[7:0]"}}
	undrawn := &checkspb.Finding{Rule: "bus-not-modeled", Subject: &checkspb.Subject{Kind: check.KindBus, Ref: "ADDR"}}

	AnnotateSheets([]*checkspb.Finding{drawn, undrawn}, g, nil)

	if len(drawn.GetSheets()) == 0 {
		t.Error("drawn bus got no sheet badge")
	}
	if drawn.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("drawn bus reason = %v, want UNSPECIFIED (it highlights)", drawn.GetLocateReason())
	}
	if len(undrawn.GetSheets()) != 0 {
		t.Errorf("undrawn bus got sheets %v, want none", undrawn.GetSheets())
	}
	if undrawn.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_BUS_NOT_DRAWN {
		t.Errorf("undrawn bus reason = %v, want BUS_NOT_DRAWN", undrawn.GetLocateReason())
	}

	// A non-bus finding never gets the bus reason, even with no sheets.
	comp := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: "R99"}}
	AnnotateSheets([]*checkspb.Finding{comp}, g, nil)
	if comp.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("component reason = %v, want UNSPECIFIED", comp.GetLocateReason())
	}
}

// A finding whose subject cannot be located has to say WHY, for every subject kind and not only for
// buses.
//
// The classifier this uses already existed and was wired to the query-result path alone, so clicking
// a query cell for a net explained itself and clicking the FINDING for the same net did nothing at
// all. UNSPECIFIED is not a neutral default here: its own contract is "the entity IS drawn — expected
// to highlight", so leaving it on an undrawn subject actively tells the viewer to stay quiet.
func TestAnnotateExplainsEveryUnlocatableSubject(t *testing.T) {
	// A rail distributed by taps: it is in the netlist and carries a power role, and no wire on any
	// sheet is labelled with it. This is the shape a decoupling-present finding lands on.
	d := &ir.Design{
		Nets: []*ir.Net{
			{Name: "VDD_3V3", Roles: classify.ConventionRoles(check.NetRoleRail)},
			{Name: "SDA"},
		},
		Components: []*ir.Component{{RefDes: "U1"}},
	}
	m := check.NewModel(d)
	// Geometry that draws SDA and nothing else, so SDA is the drawn control.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id:    "root",
		Wires: []*geom.WireGeometry{{Net: "SDA", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}}}},
	}}}

	rail := &checkspb.Finding{Rule: "decoupling-present", Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "VDD_3V3"}}
	drawn := &checkspb.Finding{Rule: "i2c-pull-up", Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "SDA"}}
	ghost := &checkspb.Finding{Rule: "x", Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "NOT_A_NET"}}
	absent := &checkspb.Finding{Rule: "y", Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: "R99"}}

	AnnotateSheets([]*checkspb.Finding{rail, drawn, ghost, absent}, g, m)

	if rail.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_POWER_RAIL_NO_WIRE {
		t.Errorf("undrawn rail reason = %v, want POWER_RAIL_NO_WIRE", rail.GetLocateReason())
	}
	// The control that keeps the rest honest: a subject that WILL highlight must stay UNSPECIFIED, or
	// the viewer shows an explanation over a working click.
	if drawn.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("drawn net reason = %v, want UNSPECIFIED", drawn.GetLocateReason())
	}
	if ghost.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_NOT_IN_DESIGN {
		t.Errorf("unknown net reason = %v, want NOT_IN_DESIGN", ghost.GetLocateReason())
	}
	if absent.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_NOT_IN_DESIGN {
		t.Errorf("unknown component reason = %v, want NOT_IN_DESIGN", absent.GetLocateReason())
	}
}

// With no model there is nothing to classify against, so an unlocatable subject stays UNSPECIFIED
// rather than being guessed at. This is the CLI's nil-source path and the pre-existing behaviour.
func TestAnnotateWithoutAModelExplainsNothing(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "root"}}}
	f := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "ANY"}}
	AnnotateSheets([]*checkspb.Finding{f}, g, nil)
	if f.GetLocateReason() != checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED {
		t.Errorf("reason = %v, want UNSPECIFIED with no model to ask", f.GetLocateReason())
	}
}
