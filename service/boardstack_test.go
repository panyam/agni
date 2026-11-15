package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
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
	}
}
