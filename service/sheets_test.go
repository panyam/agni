package service

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
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
	drawn := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindBus, Ref: "DATA[7:0]"}, Rule: "bus-not-modeled"}
	undrawn := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindBus, Ref: "ADDR"}, Rule: "bus-not-modeled"}

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

	rail := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "VDD_3V3"}, Rule: "decoupling-present"}
	drawn := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "SDA"}, Rule: "i2c-pull-up"}
	ghost := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindNet, Ref: "NOT_A_NET"}, Rule: "x"}
	absent := &checkspb.Finding{Subject: &checkspb.Subject{Kind: check.KindComponent, Ref: "R99"}, Rule: "y"}

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

// A trace was the third consumer of the sheet index and the only one that never asked, so a route
// drew on the design's first sheet whatever it crossed (agni issue 657). These pin the two lookups
// that differ from a finding's, and the outcome that is easiest to leave unfilled.
func TestAnnotateTraceSheets(t *testing.T) {
	// Two sheets, and the parts are on the SECOND. A one-sheet fixture cannot tell "found the right
	// sheet" from "returned the first one", which is exactly the bug.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "contents"},
		{Id: "page2", Placements: []*geom.SymbolPlacement{{RefDes: "U1"}, {RefDes: "R1"}}},
	}}
	tr := &webapi.Trace{
		From: &webapi.TraceEnd{Endpoint: &webapi.TraceEndpoint{RefDes: "U1", Pin: "3"}, Net: "SDA"},
		To:   &webapi.TraceEnd{Endpoint: &webapi.TraceEndpoint{RefDes: "R1", Pin: "1"}, Net: "VCC"},
		Nets: []*webapi.TraceNet{{Name: "SDA"}, {Name: "VCC"}},
	}
	AnnotateTraceSheets(tr, g, nil)

	// An endpoint resolves by PLACEMENT, not by its net: it is a pin on a part, and where that part
	// is drawn is what a reader opens. Resolving by net first put a route on a sheet carrying the
	// middle net and none of the parts.
	if got := tr.GetFrom().GetSheetIds(); len(got) != 1 || got[0] != "page2" {
		t.Errorf("from endpoint sheets = %v, want [page2] from U1's placement", got)
	}
	if got := tr.GetTo().GetSheetIds(); len(got) != 1 || got[0] != "page2" {
		t.Errorf("to endpoint sheets = %v, want [page2] from R1's placement", got)
	}
}

// The no-route case is the one a field filled only on success answers with silence, and it is the
// case a reader most wants a picture of: the two nets that do NOT join are still drawn somewhere.
func TestAnnotateTraceSheetsFillsANoRoute(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "contents"},
		{Id: "page2", Placements: []*geom.SymbolPlacement{{RefDes: "U1"}}},
	}}
	tr := &webapi.Trace{
		Outcome: webapi.TraceOutcome_TRACE_OUTCOME_NO_ROUTE,
		From:    &webapi.TraceEnd{Endpoint: &webapi.TraceEndpoint{RefDes: "U1", Pin: "3"}, Net: "SDA"},
		To:      &webapi.TraceEnd{Endpoint: &webapi.TraceEndpoint{RefDes: "J9", Pin: "1"}, Net: "FAR"},
	}
	AnnotateTraceSheets(tr, g, nil)

	if got := tr.GetFrom().GetSheetIds(); len(got) != 1 || got[0] != "page2" {
		t.Errorf("a no-route still has a drawn FROM: sheets = %v, want [page2]", got)
	}
	// The other end is genuinely not drawn, and empty must mean that rather than "nobody looked".
	if got := tr.GetTo().GetSheetIds(); len(got) != 0 {
		t.Errorf("an undrawn endpoint got sheets %v, want none", got)
	}
}
