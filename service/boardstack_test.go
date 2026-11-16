package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func TestTheBoardSheetCarriesItsStackInEitherFormat(t *testing.T) {
	b := testBoard()
	cu, core, total := int64(35_000), int64(1_530_000), int64(1_600_000)
	b.Stackup = &geom.Stackup{Thickness: &total, Layers: []*geom.StackLayer{
		{Name: "F.Cu", Kind: "copper", Layer: "F.Cu", Thickness: &cu},
		{Name: "dielectric 1", Kind: "dielectric", Thickness: &core, Material: "FR4"},
		{Name: "B.Cu", Kind: "copper", Layer: "B.Cu", Thickness: &cu},
	}}
	ld := boardLoader{
		fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}},
		board:      b,
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	snap := schematicWithBoard(t, svc)
	for _, f := range []webapi.SheetFormat{webapi.SheetFormat_SHEET_FORMAT_SVG, webapi.SheetFormat_SHEET_FORMAT_PACKED} {
		resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Snapshot: snap, Sheet: boardSheetID, Format: f})
		if err != nil {
			t.Fatalf("%v: %v", f, err)
		}
		if len(resp.GetBoardLayers()) == 0 || resp.GetBoardLayers()[0].GetThickness() != "0.035 mm" {
			t.Errorf("%v: first layer row = %v, want F.Cu at 0.035 mm", f, resp.GetBoardLayers())
		}
		st := resp.GetBoardStack()
		if len(st.GetRows()) != 3 || st.GetRows()[1].GetName() != "dielectric 1" || st.GetSummary() != "1.6 mm, as the file states" {
			t.Errorf("%v: board stack = %v, want three rows summing to the stated 1.6 mm", f, st)
		}
		if len(st.GetOutlineTris()) == 0 || st.GetUnitsPerMm() == 0 || st.GetRows()[1].GetColor() != render.DefaultStyle.Dielectric {
			t.Errorf("%v: board stack has %d outline values, %g units per mm and dielectric colour %q, want the outline filled, a unit and Style.Dielectric",
				f, len(st.GetOutlineTris()), st.GetUnitsPerMm(), st.GetRows()[1].GetColor())
		}
	}
}

// A board's blocks take their parts' classes from the design, in either format (agni 1653). R1 is a
// resistor and D1 a TVS, which draws in the diode colour. U9 is on the board and not in the design,
// so it keeps the plain body colour.
func TestTheBoardStackColoursEachPartByItsDesignClass(t *testing.T) {
	b := testBoard()
	cu := int64(35_000)
	b.Stackup = &geom.Stackup{Layers: []*geom.StackLayer{{Name: "F.Cu", Kind: "copper", Layer: "F.Cu", Thickness: &cu}}}
	pad := []*geom.Pad{{Number: "1", At: &geom.Point{}, Size: &geom.Point{X: 50, Y: 50}}}
	for i, ref := range []string{"R1", "D1", "U9"} {
		b.Placements = append(b.Placements, &geom.ComponentPlacement{RefDes: ref, At: &geom.Point{X: int64(100 + 200*i), Y: 100}, Layer: "F.Cu", Pads: pad})
	}
	tag := func(classes ...string) []*ir.ComponentClassTag {
		var out []*ir.ComponentClassTag
		for _, c := range classes {
			out = append(out, &ir.ComponentClassTag{Class: c, Source: ir.ClassSource_CLASS_SOURCE_CONVENTION})
		}
		return out
	}
	design := &ir.Design{Name: "d", Components: []*ir.Component{
		{RefDes: "R1", DeviceClasses: tag("resistor")},
		{RefDes: "D1", DeviceClasses: tag("diode", "tvs")},
	}}
	ld := boardLoader{
		fakeLoader: fakeLoader{design: design, geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}},
		board:      b,
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	snap := schematicWithBoard(t, svc)
	for _, f := range []webapi.SheetFormat{webapi.SheetFormat_SHEET_FORMAT_SVG, webapi.SheetFormat_SHEET_FORMAT_PACKED} {
		resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Snapshot: snap, Sheet: boardSheetID, Format: f})
		if err != nil {
			t.Fatalf("%v: %v", f, err)
		}
		got := map[string][2]string{}
		for _, body := range resp.GetBoardStack().GetBodies() {
			got[body.GetRefDes()] = [2]string{body.GetPartClass(), body.GetColor()}
		}
		want := map[string][2]string{
			"R1": {"resistor", render.DefaultStyle.PartClass.Resistor},
			"D1": {"tvs", render.DefaultStyle.PartClass.Diode},
			"U9": {"", render.DefaultStyle.PartBody},
		}
		if len(got) != len(want) {
			t.Fatalf("%v: %d bodies, want %d", f, len(got), len(want))
		}
		for ref, w := range want {
			if got[ref] != w {
				t.Errorf("%v: %s = %v, want %v", f, ref, got[ref], w)
			}
		}
	}
}
