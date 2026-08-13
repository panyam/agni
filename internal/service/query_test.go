package service

import (
	"context"
	"errors"
	"github.com/panyam/agni/internal/artifact"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check/naming"
	"github.com/panyam/agni/readers/formats"

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
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri:   "mount://m/x.edn",
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
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	cases := []struct {
		query string
		want  []string
	}{
		{`component-on-net(?r,?n) => ?r, ?n`, []string{"component", "net"}},
		{`component.mpn(?r,?m) => ?r, ?m`, []string{"component", ""}}, // mpn is a scalar label
	}
	for _, tc := range cases {
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: tc.query})
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
	svc := NewQueryService(fakeLoader{design: queryDesign(), geom: g}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/x.kicad_sch", Query: `component-on-net(?r,?n) => ?r, ?n`,
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
	svc := NewQueryService(fakeLoader{design: d}, nil, nil) // no geometry
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/x.edn", Query: `component-on-net(?r,?n) => ?r, ?n`,
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
	svc := NewQueryService(fakeLoader{design: d, geom: g}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/x.kicad_sch", Query: `component-on-net(?r,?n) => ?r, ?n`,
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
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/x.edn", Query: "compnent-on-net(?r,?n) => ?r",
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument", err)
	}
	if !strings.Contains(err.Error(), "did you mean") || !strings.Contains(err.Error(), "component-on-net") {
		t.Errorf("error should suggest the relation, got: %v", err)
	}
}

func TestRunQueryMalformedIsInvalidArgument(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/x.edn", Query: `component-on-net(?r,?n =>`,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument for a parse failure", err)
	}
}

func TestRunQueryNoMatchIsEmptyNotError(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri:   "mount://m/x.edn",
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
	svc := NewQueryService(fakeLoader{err: errors.New("no netlist")}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.eds", Query: `component-on-net(?r,?n) => ?r`})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("err = %v, want ErrInvalidArgument for an unloadable design", err)
	}
}

func TestRunQueryNotFoundPropagates(t *testing.T) {
	svc := NewQueryService(fakeLoader{err: ErrNotFound}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://bad/x.edn", Query: `component-on-net(?r,?n) => ?r`})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound to pass through", err)
	}
}

func TestListRelationsReturnsCatalog(t *testing.T) {
	svc := NewQueryService(fakeLoader{}, nil, nil) // no design needed — the catalog is static
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
	svc := NewQueryService(fakeLoader{}, nil, nil)
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
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	resp, err := svc.ListRelations(context.Background(), &webapi.ListRelationsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range resp.GetExamples() {
		if _, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
			Uri: "mount://m/x.edn", Query: e.GetQuery(),
		}); err != nil {
			t.Errorf("example %q failed to evaluate: %v", e.GetLabel(), err)
		}
	}
}

// guard: the malformed-query message reaches the caller (the panel shows it inline).
func TestRunQueryParseErrorMessagePreserved(t *testing.T) {
	svc := NewQueryService(fakeLoader{design: queryDesign()}, nil, nil)
	_, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: "mount://m/x.edn", Query: `!!!`})
	if err == nil || strings.TrimSpace(err.Error()) == "" {
		t.Fatalf("want a non-empty parse error message, got %v", err)
	}
}

// fsQueryLoader reads a real design file and HONORS the read options, which is the whole point: a
// lexicon is applied at ingestion, so a loader that discarded the option would make the test below
// assert nothing. It embeds fakeLoader for the methods a query never calls.
type fsQueryLoader struct {
	fakeLoader
	base string
}

func (l fsQueryLoader) Design(_ context.Context, uri artifact.URI, opts ...ReadOption) (*ir.Design, error) {
	return (&formats.Loader{Lexicon: ReadOpts(opts...).Lexicon}).ReadDesign(filepath.Join(l.base, uri.Path))
}

// houseConvention is the fixture project's vocabulary: its rails are named function-first
// ("PMIC_VDD_LPM_1V8"), which the built-in rail vocabulary — start-anchored on VCC/VDD/+3V3 — matches
// none of.
func houseConvention(t *testing.T) *webapi.NamingConvention {
	t.Helper()
	cfg, err := naming.Load(filepath.Join("..", "..", "cmd", "agni", "testdata", "review", "conventions.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return ConventionProto(cfg)
}

func queryColumn(t *testing.T, resp *webapi.RunQueryResponse) []string {
	t.Helper()
	var out []string
	for _, row := range resp.GetRows() {
		if len(row.GetCells()) > 0 {
			out = append(out, row.GetCells()[0])
		}
	}
	sort.Strings(out)
	return out
}

// TestRunQueryHonorsTheRequestLexicon is the gap this feature closes, using the issue's own example.
//
// `rail(?n)` is not a relation a convention ADDS. It is a relation whose ANSWER a convention changes,
// because net roles are resolved once at the design read and the lexicon decides which names carry
// which role. Without the project's vocabulary the engine reports one rail on a board with more,
// which is a correct answer to a question the project did not ask, and there was previously no way to
// ask theirs.
//
// This is also why `query` needed it more than a missing flag usually warrants: authoring a lexicon
// is a loop of writing a pattern and asking which nets are rails now, and query is the tool for that
// loop.
func TestRunQueryHonorsTheRequestLexicon(t *testing.T) {
	svc := NewQueryService(fsQueryLoader{base: filepath.Join("..", "..", "cmd", "agni", "testdata")}, nil, nil)
	ask := func(ov *webapi.OverlayConfig) []string {
		t.Helper()
		resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
			Uri: "mount://m/review/conv-demo.edn", Query: "rail(?n) => ?n", Overlay: ov,
		})
		if err != nil {
			t.Fatalf("RunQuery: %v", err)
		}
		return queryColumn(t, resp)
	}

	builtin := ask(nil)
	if slices.Contains(builtin, "PMIC_VDD_LPM_1V8") {
		t.Fatal("the built-in vocabulary already matches the project's rail names; the fixture no longer demonstrates the gap")
	}

	house := ask(&webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: houseConvention(t)}})
	if !slices.Contains(house, "PMIC_VDD_LPM_1V8") {
		t.Errorf("rail(?n) under the project's own vocabulary = %v, want it to include PMIC_VDD_LPM_1V8", house)
	}
	if len(house) <= len(builtin) {
		t.Errorf("the project's vocabulary found no more rails than the built-ins: %v vs %v", house, builtin)
	}
}

// TestRunQueryIgnoresTheConventionsRulesHalf: a project keeps ONE conventions file carrying both
// halves, and a query legitimately consumes only the lexicon. Sending one whose rules would not even
// compile into a catalog must still answer, because a query composes no catalog at all — the
// alternative is refusing a config that is fine for the question being asked.
func TestRunQueryIgnoresTheConventionsRulesHalf(t *testing.T) {
	svc := NewQueryService(fsQueryLoader{base: filepath.Join("..", "..", "cmd", "agni", "testdata")}, nil, nil)
	conv := houseConvention(t)
	// A rule whose name collides with the convention's own namespace would fail a catalog composition.
	conv.Rules = append(conv.Rules, &webapi.NamingRule{Name: "signal-net-naming", Allow: []string{"^X"}})
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{
		Uri: "mount://m/review/conv-demo.edn", Query: "rail(?n) => ?n",
		Overlay: &webapi.OverlayConfig{Config: &webapi.AnalysisConfig{Conventions: conv}},
	})
	if err != nil {
		t.Fatalf("a query must not fail on the rules half of a convention it does not use: %v", err)
	}
	if !slices.Contains(queryColumn(t, resp), "PMIC_VDD_LPM_1V8") {
		t.Error("the lexicon half did not reach the read")
	}
}

// TestPortableCites is the fix for a cite that named a host directory.
//
// A reader stamps provenance with the path it was handed, and the loader hands it an ABSOLUTE host
// path, so query was the one surface where "/Users/someone/work/..." escaped into output a user
// pastes into issues and commits into reports. It is also the only surface that did: `review` prints
// the design as given and a finding's source_file is relative.
func TestPortableCites(t *testing.T) {
	const design = "designs/gateway/gateway.edn"
	for name, tc := range map[string]struct{ in, want string }{
		"an absolute host path is cut back to the design": {
			"/Users/someone/work/agni/examples/tutorial-project/designs/gateway/gateway.edn:GND",
			"designs/gateway/gateway.edn:GND"},
		"a cite already relative is untouched": {
			"designs/gateway/gateway.edn:GND", "designs/gateway/gateway.edn:GND"},
		// A datasheet citation names a document and a page, never a file. Truncating it on a guess
		// would destroy provenance that was correct.
		"a citation that is not a path is left alone": {
			"ACME-LDO-1V8 Datasheet Rev A page 7", "ACME-LDO-1V8 Datasheet Rev A page 7"},
	} {
		t.Run(name, func(t *testing.T) {
			got := portableCites([]string{tc.in}, design)
			if got[0] != tc.want {
				t.Errorf("got %q, want %q", got[0], tc.want)
			}
		})
	}
	if got := portableCites(nil, design); got != nil {
		t.Errorf("no cites should stay nil, got %v", got)
	}
}
