package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// twoSheetGeometry draws SDA on both sheets and places R9 on S2 alone.
func twoSheetGeometry() *geom.SchematicGeometry {
	wire := func(net string) *geom.WireGeometry {
		return &geom.WireGeometry{Net: net, Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}}}
	}
	return &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "S1", Wires: []*geom.WireGeometry{wire("SDA")}},
		{Id: "S2", Wires: []*geom.WireGeometry{wire("SDA")}, Placements: []*geom.SymbolPlacement{{RefDes: "R9", Transform: &geom.Transform{Origin: &geom.Point{X: 5, Y: 5}}}}},
	}}
}

// A verdict carries the sheets its subjects are drawn on, as a finding does, so the viewer can open
// one before drawing the proof (agni issue 1151). A passing verdict has no finding to borrow them
// from, which is the case this exists for.
func TestCheckDesignVerdictSheets(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R9"}},
		Nets: []*ir.Net{{
			Name:        "SDA",
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
		}},
	}
	svc := NewCheckService(fakeLoader{design: d, geom: twoSheetGeometry()}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snapOf(t, svc.Snapshots(), "mount://m/x.edn")})
	if err != nil {
		t.Fatal(err)
	}
	var sda, passing int
	for _, v := range resp.GetVerdicts() {
		subs := v.GetSubjects()
		if len(subs) != 1 || subs[0].GetKind() != check.KindNet || subs[0].GetRef() != "SDA" {
			continue
		}
		sda++
		if v.GetOutcome() == checkspb.Outcome_OUTCOME_PASS {
			passing++
		}
		if !slicesEqual(v.GetSheets(), []string{"S1", "S2"}) {
			t.Errorf("verdict %s sheets = %v, want [S1 S2]", v.GetId(), v.GetSheets())
		}
	}
	if sda == 0 || passing == 0 {
		t.Fatalf("the run answered %d verdicts about SDA, %d of them passing; the test needs at least one passing", sda, passing)
	}
}

// A verdict about a tuple lists each subject's sheets in the tuple's order, so the first subject's
// sheet is the one a viewer opens.
func TestAnnotateRunSheetsFollowsTupleOrder(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{{Name: "SDA"}}, Components: []*ir.Component{{RefDes: "R9"}}}
	tuple := &checkspb.Verdict{Subjects: []*checkspb.Subject{{Kind: check.KindComponent, Ref: "R9"}, {Kind: check.KindNet, Ref: "SDA"}}}
	ghost := &checkspb.Verdict{Subjects: []*checkspb.Subject{{Kind: check.KindNet, Ref: "NOT_A_NET"}}}
	AnnotateRunSheets(&webapi.CheckDesignResponse{Verdicts: []*checkspb.Verdict{tuple, ghost}}, twoSheetGeometry(), check.NewModel(d))
	if !slicesEqual(tuple.GetSheets(), []string{"S2", "S1"}) {
		t.Errorf("tuple sheets = %v, want [S2 S1]", tuple.GetSheets())
	}
	if len(ghost.GetSheets()) != 0 {
		t.Errorf("an undrawn subject got sheets %v", ghost.GetSheets())
	}
}
