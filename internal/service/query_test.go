package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/netgraph"
)

// queryDesign has two parts on one net, so component-on-net(?r,?n) yields two answer rows.
func queryDesign() *ir.Design {
	return &ir.Design{
		Nets: []*ir.Net{{
			Name:        "SDA",
			Prov:        &ir.Provenance{SourceFile: "x.edn", NativeId: "SDA"},
			Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "5"}},
		}},
	}
}

func TestRunQueryReturnsRowsAndProvenance(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.edn",
		Query: `component-on-net(?r,?n) => ?r, ?n`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetColumns(); len(got) != 2 || got[0] != "r" || got[1] != "n" {
		t.Fatalf("columns = %v, want [r n]", got)
	}
	got := map[string]bool{}
	for _, row := range resp.GetRows() {
		cells := row.GetCells()
		if len(cells) != 2 {
			t.Fatalf("row cells = %v, want 2", cells)
		}
		got[cells[0]+"@"+cells[1]] = true
	}
	for _, want := range []string{"R1@SDA", "U1@SDA"} {
		if !got[want] {
			t.Errorf("missing answer row %q; got %v", want, got)
		}
	}
	// Every answer row carries the provenance of the facts that produced it.
	for _, row := range resp.GetRows() {
		if len(row.GetCites()) == 0 {
			t.Errorf("row %v carries no provenance", row.GetCells())
		}
	}
}

// column_kinds is derived from the relation arg-labels, not the cell values: a ref_des column is
// "component", a net column is "net", and anything else (an mpn string, a scalar) is "". It is
// returned for the query shape regardless of whether any row matched (WS9-038).
func TestRunQueryColumnKinds(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	cases := []struct {
		query string
		want  []string
	}{
		{`component-on-net(?r,?n) => ?r, ?n`, []string{"component", "net"}},
		{`component.mpn(?r,?m) => ?r, ?m`, []string{"component", ""}}, // mpn is a scalar label
	}
	for _, tc := range cases {
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Mount: "m", Path: "x.edn", Query: tc.query})
		if err != nil {
			t.Fatalf("%s: %v", tc.query, err)
		}
		got := resp.GetColumnKinds()
		if len(got) != len(tc.want) {
			t.Fatalf("%s: column_kinds = %v, want %v", tc.query, got, tc.want)
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("%s: column_kinds[%d] = %q, want %q", tc.query, i, got[i], tc.want[i])
			}
		}
	}
}

// A component cell's sheet membership comes from the schematic geometry placements (WS9-038), the
// same source the finding sheet badges use — so a component cell can navigate to its sheet.
func TestRunQueryComponentCellSheetsFromGeometry(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "s1"},
		{Id: "s2", Placements: []*geom.SymbolPlacement{{RefDes: "R1"}}},
	}}
	svc := NewQueryService(fakeLoader{design: queryDesign(), geom: g}, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.kicad_sch", Query: `component-on-net(?r,?n) => ?r, ?n`,
	})
	if err != nil {
		t.Fatal(err)
	}
	var r1sheets []string
	for _, row := range resp.GetRows() {
		if row.GetCells()[0] == "R1" {
			r1sheets = row.GetCellSheets()[0].GetSheetIds() // cell 0 is the component column
		}
	}
	if len(r1sheets) != 1 || r1sheets[0] != "s2" {
		t.Errorf("R1 component cell sheets = %v, want [s2]", r1sheets)
	}
}

// A net cell's sheet membership comes from the netlist AttrSheets attribute (WS9-028), so a net
// cell locates without any geometry loaded (WS9-038).
func TestRunQueryNetCellSheetsFromNetlist(t *testing.T) {
	d := queryDesign()
	d.Nets[0].Attributes = map[string]string{netgraph.AttrSheets: netgraph.EncodeSheets([]string{"s1", "s2"})}
	svc := NewQueryService(fakeLoader{design: d}, nil) // no geometry
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.edn", Query: `component-on-net(?r,?n) => ?r, ?n`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range resp.GetRows() {
		got := row.GetCellSheets()[1].GetSheetIds() // cell 1 is the net column (SDA)
		if len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
			t.Errorf("net cell sheets = %v, want [s1 s2]", got)
		}
	}
}

// A navigable cell gets a locate reason (WS9-039) only for an entity the faithful geometry does not
// draw: a drawn part/wire (R1, SIG) is UNSPECIFIED, an undrawn `#`-ref is virtual, an undrawn rail
// (GND) is a power rail. A drawn rail would report UNSPECIFIED (the VBUS case) — geometry wins over
// the name heuristic.
func TestRunQueryCellReasons(t *testing.T) {
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "SIG", Prov: &ir.Provenance{SourceFile: "x.kicad_sch", NativeId: "SIG"},
			Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "2"}}},
		{Name: "GND", Prov: &ir.Provenance{SourceFile: "x.kicad_sch", NativeId: "GND"},
			Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "2"}, {ComponentRef: "#PWR02", PinRef: "1"}}},
	}}
	// Faithful geometry draws R1/U1 (placements) and SIG (a wire); #PWR02 and GND are not drawn.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id:         "root",
		Placements: []*geom.SymbolPlacement{{RefDes: "R1"}, {RefDes: "U1"}},
		Wires:      []*geom.WireGeometry{{Net: "SIG"}},
	}}}
	svc := NewQueryService(fakeLoader{design: d, geom: g}, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.kicad_sch", Query: `component-on-net(?r,?n) => ?r, ?n`,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range resp.GetRows() {
		reasons := row.GetCellReasons()
		if len(reasons) != 2 {
			t.Fatalf("cell_reasons = %v, want 2 aligned to cells", reasons)
		}
		// the net column: the drawn SIG is UNSPECIFIED, the undrawn GND is a power rail.
		wantNet := checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED
		if row.GetCells()[1] == "GND" {
			wantNet = checkspb.LocateReason_LOCATE_REASON_POWER_RAIL_NO_WIRE
		}
		if reasons[1] != wantNet {
			t.Errorf("net %q reason = %v, want %v", row.GetCells()[1], reasons[1], wantNet)
		}
		// the component column: drawn R1/U1 are UNSPECIFIED, the undrawn #-ref is virtual.
		wantComp := checkspb.LocateReason_LOCATE_REASON_UNSPECIFIED
		if row.GetCells()[0] == "#PWR02" {
			wantComp = checkspb.LocateReason_LOCATE_REASON_VIRTUAL_SYMBOL
		}
		if reasons[0] != wantComp {
			t.Errorf("component %q reason = %v, want %v", row.GetCells()[0], reasons[0], wantComp)
		}
	}
}

// A typo'd relation name reaches the panel/CLI as an invalid-argument whose message suggests the
// closest catalog relation (WS14-003) — teaching the vocabulary at the point of the mistake.
func TestRunQueryUnknownRelationSuggests(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.edn", Query: "compnent-on-net(?r,?n) => ?r",
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
	if !strings.Contains(err.Error(), "did you mean") || !strings.Contains(err.Error(), "component-on-net") {
		t.Errorf("error should suggest the relation, got: %v", err)
	}
}

func TestRunQueryMalformedIsInvalidArgument(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.edn", Query: `component-on-net(?r,?n =>`,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument for a parse failure", err)
	}
}

func TestRunQueryNoMatchIsEmptyNotError(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Mount: "m", Path: "x.edn",
		Query: `component-on-net(?r,?n), ?n = "NOSUCHNET" => ?r`,
	})
	if err != nil {
		t.Fatalf("a well-formed no-match query should not error: %v", err)
	}
	if len(resp.GetRows()) != 0 {
		t.Errorf("rows = %v, want none", resp.GetRows())
	}
}

func TestRunQueryUnloadableIsInvalidArgument(t *testing.T) {
	svc := NewQueryService(fakeLoader{err: errors.New("no netlist")}, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Mount: "m", Path: "x.eds", Query: `component-on-net(?r,?n) => ?r`})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument for an unloadable design", err)
	}
}

func TestRunQueryNotFoundPropagates(t *testing.T) {
	svc := NewQueryService(fakeLoader{err: ErrNotFound}, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Mount: "bad", Path: "x.edn", Query: `component-on-net(?r,?n) => ?r`})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound to pass through", err)
	}
}

func TestListRelationsReturnsCatalog(t *testing.T) {
	svc := NewQueryService(fakeLoader{}, nil) // no design needed — the catalog is static
	resp, err := svc.ListRelations(context.Background(), &webapi.ListRelationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*webapi.RelationInfo{}
	for _, r := range resp.GetRelations() {
		byName[r.GetName()] = r
	}
	con := byName["component-on-net"]
	if con == nil {
		t.Fatalf("component-on-net missing from catalog; got %d relations", len(resp.GetRelations()))
	}
	if got := con.GetArgs(); len(got) != 2 || got[0] != "ref_des" || got[1] != "net" {
		t.Errorf("component-on-net args = %v, want [ref_des net]", con.GetArgs())
	}
	if con.GetKind() != "netlist" {
		t.Errorf("component-on-net kind = %q, want netlist", con.GetKind())
	}
	if byName["reaches"] == nil || byName["reaches"].GetKind() != "predicate" {
		t.Errorf("reaches should be catalogued as a predicate")
	}
	// Detail (WS14-005) rides the catalog: a documented relation carries its reference markdown, an
	// undocumented one carries "" (so the panel falls back to the summary).
	if bl := byName["net.bus_like"]; bl == nil || !strings.HasPrefix(bl.GetDetail(), "## net.bus_like") {
		t.Errorf("net.bus_like Detail should be its reference doc, got %q", bl.GetDetail())
	}
}

func TestListRelationsIncludesExamples(t *testing.T) {
	svc := NewQueryService(fakeLoader{}, nil)
	resp, err := svc.ListRelations(context.Background(), &webapi.ListRelationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetExamples()) == 0 {
		t.Fatal("no examples in ListRelations response")
	}
	for _, e := range resp.GetExamples() {
		if e.GetLabel() == "" || e.GetQuery() == "" || e.GetTeaches() == "" {
			t.Errorf("example missing fields: %+v", e)
		}
	}
}

// Every shipped example must actually evaluate against a real design — not just parse — so a
// relation renamed out from under an example fails here (RunQuery returns InvalidArgument on a bad
// relation), not silently in the panel.
func TestExamplesEvaluate(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	resp, err := svc.ListRelations(context.Background(), &webapi.ListRelationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range resp.GetExamples() {
		if _, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
			Mount: "m", Path: "x.edn", Query: e.GetQuery(),
		}); err != nil {
			t.Errorf("example %q failed to evaluate: %v", e.GetLabel(), err)
		}
	}
}

// guard: the malformed-query message reaches the caller (the panel shows it inline).
func TestRunQueryParseErrorMessagePreserved(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Mount: "m", Path: "x.edn", Query: `!!!`})
	if err == nil || strings.TrimSpace(err.Error()) == "" {
		t.Fatalf("want a non-empty parse error message, got %v", err)
	}
}
