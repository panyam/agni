package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/expect"
	"github.com/panyam/agni/internal/netgraph"
)

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAvailableLayouts(t *testing.T) {
	cases := map[string][]string{
		"x.eds":       {"faithful", "force", "grid", "layered", "orthogonal", "stress"}, // geometry + netlist
		"x.kicad_sch": {"faithful", "force", "grid", "layered", "orthogonal", "stress"}, // geometry + netlist
		"x.kicad_pcb": {"force", "grid", "layered", "orthogonal", "stress"},             // netlist only (no faithful board)
		"x.edn":       {"force", "grid", "layered", "orthogonal", "stress"},             // netlist only
		"board.xml":   {"force", "grid", "layered", "orthogonal", "stress"},
	}
	for path, want := range cases {
		if got := availableLayouts(path); !slicesEqual(got, want) {
			t.Errorf("availableLayouts(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestLayoutForFile(t *testing.T) {
	if got := layoutForFile("a.kicad_sch", "grid"); got != "grid" {
		t.Errorf("kicad_sch + grid = %q, want grid (honored)", got)
	}
	if got := layoutForFile("a.kicad_sch", ""); got != "faithful" {
		t.Errorf("kicad_sch default = %q, want faithful", got)
	}
	if got := layoutForFile("a.eds", "grid"); got != "grid" {
		t.Errorf("eds + grid = %q, want grid (now available — .eds carries a netlist)", got)
	}
	if got := layoutForFile("a.eds", ""); got != "faithful" {
		t.Errorf("eds default = %q, want faithful (geometry is the natural default)", got)
	}
	if got := layoutForFile("a.edn", ""); got != "grid" {
		t.Errorf("edn default = %q, want grid", got)
	}
}

// fakeLoader is an in-memory Loader: it proves the services run with no os (C13). Each
// method returns its canned value (or the shared wrapped error). geomErr fails only Geometry,
// for the "design has a netlist but no resolvable geometry" cases.
type fakeLoader struct {
	design  *ir.Design
	geom    *geom.SchematicGeometry
	board   *geom.BoardGeometry
	report  *graph.ConversionReport
	expect  *expect.Expectations
	err     error
	geomErr error
}

func (f fakeLoader) Design(context.Context, artifact.URI, ...ReadOption) (*ir.Design, error) {
	return f.design, f.err
}
func (f fakeLoader) Geometry(context.Context, artifact.URI, string, bool, ...ReadOption) (*geom.SchematicGeometry, error) {
	if f.geomErr != nil {
		return nil, f.geomErr
	}
	return f.geom, f.err
}
func (f fakeLoader) Report(context.Context, artifact.URI, bool, ...ReadOption) (*graph.ConversionReport, error) {
	return f.report, f.err
}
func (f fakeLoader) Expectations(context.Context, artifact.URI) (*expect.Expectations, error) {
	return f.expect, f.err
}
func (f fakeLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return f.board, f.err // nil board is normal (netlist-only); a board fixture drives the tier
}

// noNative is a NativeRenderer that offers nothing (the common server default in tests).
type noNative struct{}

func (noNative) Available(artifact.URI) bool { return false }
func (noNative) Render(context.Context, artifact.URI, int) (string, error) {
	return "", ErrNativeNoTool
}

func TestCheckDesignOverFakeLoader(t *testing.T) {
	// SDA is an I2C net with no pull-up resistor, so check.Run flags it. The service does no I/O:
	// the design is handed in by the (fake) loader.
	d := &ir.Design{Nets: []*ir.Net{{
		Name:        "SDA",
		Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
	}}}
	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetRule() == "i2c-pull-up" && f.GetSubject().GetRef() == "SDA" {
			found = true
		}
	}
	if !found {
		t.Errorf("want an i2c-pull-up finding on SDA, got %+v", resp.GetFindings())
	}

	// A loader ErrNotFound (unknown mount) stays classified as not-found for the transport.
	bad := NewCheckService(fakeLoader{err: fmt.Errorf("no such mount: %w", ErrNotFound)}, check.DefaultCatalog(), nil, "", nil, nil)
	if _, err := bad.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://no/x"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestGetInterfaceCoverage exercises the coverage RPC end to end (handler + adapter path is the
// server's): an SPI-NOR bus with IO2 absent, IO3 single-pin, and CS unpulled yields the matrix the
// coverage panel renders, one entry per detected interface.
func TestGetInterfaceCoverage(t *testing.T) {
	conn := func(ref, pin string) *ir.Connection { return &ir.Connection{ComponentRef: ref, PinRef: pin} }
	d := &ir.Design{Nets: []*ir.Net{
		{Name: "SPI_CS", Connections: []*ir.Connection{conn("U1", "1"), conn("U2", "1")}},   // unpulled
		{Name: "SPI_SCLK", Connections: []*ir.Connection{conn("U1", "2"), conn("U2", "2")}}, // present
		{Name: "SPI_IO0", Connections: []*ir.Connection{conn("U1", "3"), conn("U2", "3")}},
		{Name: "SPI_IO1", Connections: []*ir.Connection{conn("U1", "4"), conn("U2", "4")}},
		{Name: "SPI_IO3", Connections: []*ir.Connection{conn("U1", "6")}}, // single-pin -> dangling
	}}
	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.GetInterfaceCoverage(context.Background(), &webapi.GetInterfaceCoverageRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetInterfaces()) != 1 || resp.GetInterfaces()[0].GetProfile() != "SPI_NOR" {
		t.Fatalf("want one SPI_NOR interface, got %+v", resp.GetInterfaces())
	}
	got := map[string]string{}
	for _, s := range resp.GetInterfaces()[0].GetSignals() {
		got[s.GetName()] = s.GetState()
	}
	want := map[string]string{"CS": "pullup_missing", "SCLK": "present", "IO0": "present", "IO1": "present", "IO2": "missing", "IO3": "dangling"}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %s, want %s", name, got[name], w)
		}
	}
}

// TestListRules maps the injected catalog to RuleInfo: every rule reports its tags, the facts it
// reads, and its availability (the built-ins are topology rules, so all available).
func TestListRules(t *testing.T) {
	// The catalog is the built-ins plus every RegisterSource'd suite (the interface profiles register
	// via profiles.init(), pulled in by this package's coverage handler), so assert against the
	// catalog's own count, not the built-in slice.
	catalog := check.DefaultCatalog()
	svc := NewCheckService(fakeLoader{}, catalog, nil, "", nil, nil)
	resp, err := svc.ListRules(context.Background(), &webapi.ListRulesRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(resp.GetRules()); got != len(catalog.Rules()) {
		t.Fatalf("ListRules returned %d rules, want the catalog's %d", got, len(catalog.Rules()))
	}
	byName := map[string]*webapi.RuleInfo{}
	for _, r := range resp.GetRules() {
		byName[r.GetName()] = r
	}
	spn := byName["single-pin-net"]
	if spn == nil {
		t.Fatal("single-pin-net missing from catalog")
	}
	if spn.GetTags()[check.KeyCategory] != check.CategoryConnectivity {
		t.Errorf("single-pin-net category tag = %q, want %q", spn.GetTags()[check.KeyCategory], check.CategoryConnectivity)
	}
	if len(spn.GetReads()) == 0 {
		t.Error("single-pin-net reports no reads")
	}
	if !spn.GetAvailable() {
		t.Errorf("single-pin-net should be available (topology), reason %q", spn.GetUnavailableReason())
	}
	// The catalog carries the full prose (WS9-020): summary, impact, and the long-form detail
	// markdown, so the viewer renders rule explanations without a second fetch.
	if spn.GetSummary() == "" || spn.GetImpact() == "" {
		t.Errorf("single-pin-net prose incomplete: summary %q, impact %q", spn.GetSummary(), spn.GetImpact())
	}
	if !strings.Contains(spn.GetDetail(), "single-pin-net") {
		t.Errorf("single-pin-net detail should carry the rule's markdown, got %q", spn.GetDetail())
	}
}

// TestCheckDesignSubsetAndSubject checks that request.rules restricts the run to the named rules
// and that each finding carries a structured subject (kind + ref), not a bare string.
func TestCheckDesignSubsetAndSubject(t *testing.T) {
	// SDA trips i2c-pull-up; R9 (on no net) trips unconnected-component.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R9"}},
		Nets: []*ir.Net{{
			Name:        "SDA",
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
		}},
	}
	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), nil, "", nil, nil)

	// Full run: both rules fire.
	full, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	rules := map[string]bool{}
	for _, f := range full.GetFindings() {
		rules[f.GetRule()] = true
	}
	if !rules["i2c-pull-up"] || !rules["unconnected-component"] {
		t.Fatalf("full run missing expected rules, got %v", rules)
	}

	// Subset: only i2c-pull-up requested, so unconnected-component must not appear.
	sub, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{
		Uri: "mount://m/x.edn", Rules: []string{"i2c-pull-up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range sub.GetFindings() {
		if f.GetRule() != "i2c-pull-up" {
			t.Errorf("subset leaked rule %q", f.GetRule())
		}
		// Structured subject: the I2C finding is about a net.
		if f.GetSubject().GetKind() != check.KindNet || f.GetSubject().GetRef() != "SDA" {
			t.Errorf("subject = %+v, want {net, SDA}", f.GetSubject())
		}
	}
	if len(sub.GetFindings()) == 0 {
		t.Error("subset run returned no findings")
	}
}

// TestCheckDesignSheets pins the WS9-024 join: each finding carries the sheet ids where its
// subject appears in the geometry — a component's from its placements, a net's from its wires
// on every sheet it spans — so the viewer can navigate to the right sheet before highlighting.
// A design whose geometry cannot be resolved degrades to findings with no sheets, never an error.
func TestCheckDesignSheets(t *testing.T) {
	// R9 (on no net) trips unconnected-component; SDA trips i2c-pull-up.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R9"}},
		Nets: []*ir.Net{{
			Name:        "SDA",
			Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "5"}, {ComponentRef: "U2", PinRef: "5"}},
		}},
	}
	// R9 is placed on sheet S2 only; SDA has wires on both sheets (a spanning net). The two SDA
	// wire segments on S1 must not duplicate the sheet id.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{
			Id: "S1",
			Wires: []*geom.WireGeometry{
				{Net: "SDA", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}}},
				{Net: "SDA", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 10, Y: 0}, {X: 20, Y: 0}}}}},
			},
		},
		{
			Id:         "S2",
			Wires:      []*geom.WireGeometry{{Net: "SDA", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 10, Y: 0}}}}}},
			Placements: []*geom.SymbolPlacement{{RefDes: "R9", Transform: &geom.Transform{Origin: &geom.Point{X: 5, Y: 5}}}},
		},
	}}
	svc := NewCheckService(fakeLoader{design: d, geom: g}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	sheetsBySubject := map[string][]string{}
	for _, f := range resp.GetFindings() {
		sheetsBySubject[f.GetSubject().GetRef()] = f.GetSheets()
	}
	if got := sheetsBySubject["R9"]; !slicesEqual(got, []string{"S2"}) {
		t.Errorf("R9 sheets = %v, want [S2]", got)
	}
	if got := sheetsBySubject["SDA"]; !slicesEqual(got, []string{"S1", "S2"}) {
		t.Errorf("SDA sheets = %v, want [S1 S2] (spanning, deduped)", got)
	}

	// The severity pivot carries the same join (the report panel shares the navigation).
	rep, err := svc.GetCheckReport(context.Background(), &webapi.GetCheckReportRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sec := range rep.GetReport().GetSections() {
		for _, rg := range sec.GetRules() {
			for _, f := range rg.GetFindings() {
				if f.GetSubject().GetRef() == "SDA" && slicesEqual(f.GetSheets(), []string{"S1", "S2"}) {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("GetCheckReport findings missing the SDA sheet annotation")
	}

	// Geometry failure degrades to empty sheets, not an error.
	noGeom := NewCheckService(fakeLoader{design: d, geomErr: fmt.Errorf("no geometry")}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err = noGeom.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetFindings()) == 0 {
		t.Fatal("degraded run lost its findings")
	}
	for _, f := range resp.GetFindings() {
		if len(f.GetSheets()) != 0 {
			t.Errorf("finding on %s has sheets %v without geometry", f.GetSubject().GetRef(), f.GetSheets())
		}
	}
}

// TestCheckDesignNetSheetsFromAttribute pins the WS9-028 net-membership channel: a net that
// carries AttrSheets (the hierarchy walk's authoritative membership) badges its finding from
// that attribute, NOT from wire geometry. This is the case WS9-024's wire join could not reach —
// a wireless sub-sheet net has no wire naming it, so the geometry tally leaves it badge-less. The
// attribute also OVERRIDES the geometry tally when both exist, so the authoritative membership
// wins over a partial wire picture.
func TestCheckDesignNetSheetsFromAttribute(t *testing.T) {
	// /amp2/CTRL is a one-pin net (trips single-pin-net) carrying membership "/amp2"; SPANNED is a
	// one-pin net whose attribute says "/a,/b" while geometry only wires it on "/a" — the attribute
	// must win.
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "R1"}, {RefDes: "R2"}},
		Nets: []*ir.Net{
			{
				Name:        "/amp2/CTRL",
				Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}},
				Attributes:  map[string]string{netgraph.AttrSheets: netgraph.EncodeSheets([]string{"/amp2"})},
			},
			{
				Name:        "SPANNED",
				Connections: []*ir.Connection{{ComponentRef: "R2", PinRef: "1"}},
				Attributes:  map[string]string{netgraph.AttrSheets: netgraph.EncodeSheets([]string{"/a", "/b"})},
			},
		},
	}
	// Geometry has the sheets but no wire names /amp2/CTRL (the wireless case); SPANNED is wired
	// only on /a, so the geometry tally alone would under-report it.
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{
		{Id: "/", Placements: []*geom.SymbolPlacement{{RefDes: "R1"}, {RefDes: "R2"}}},
		{Id: "/a", Wires: []*geom.WireGeometry{{Net: "SPANNED", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0}, {X: 1}}}}}}},
		{Id: "/amp2"},
		{Id: "/b"},
	}}
	svc := NewCheckService(fakeLoader{design: d, geom: g}, check.DefaultCatalog(), nil, "", nil, nil)
	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Uri: "mount://m/x.kicad_pro"})
	if err != nil {
		t.Fatal(err)
	}
	bySubject := map[string][]string{}
	for _, f := range resp.GetFindings() {
		bySubject[f.GetSubject().GetRef()] = f.GetSheets()
	}
	if got := bySubject["/amp2/CTRL"]; !slicesEqual(got, []string{"/amp2"}) {
		t.Errorf("/amp2/CTRL sheets = %v, want [/amp2] (from the netlist attribute, not wire geometry)", got)
	}
	if got := bySubject["SPANNED"]; !slicesEqual(got, []string{"/a", "/b"}) {
		t.Errorf("SPANNED sheets = %v, want [/a /b] (attribute overrides the /a-only wire tally)", got)
	}
}

// TestHighlightSheet drives the overlay endpoint over the fake loader: PACKED resolves specs to
// primitive-index groups (joining what GetSheet's PackedSheet would carry), SVG returns a
// transparent overlay document, and NATIVE — which has no overlay concept — is rejected up front.
func TestHighlightSheet(t *testing.T) {
	g := &geom.SchematicGeometry{
		Symbols: []*geom.SymbolDef{{
			CellRef: "R", LibraryRef: "L", ViewRef: "v",
			Shapes: []*geom.Shape{{Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: 0, Y: 0}, {X: 40, Y: 20}}}},
			Pins:   []*geom.PinPoint{{PortRef: "1", Loc: &geom.Point{X: 0, Y: 10}}},
		}},
		Sheets: []*geom.SheetGeometry{{
			Id: "P1",
			Wires: []*geom.WireGeometry{{Net: "NET1", Polylines: []*geom.Polyline{
				{Points: []*geom.Point{{X: 100, Y: 100}, {X: 200, Y: 100}}},
			}}},
			Placements: []*geom.SymbolPlacement{{
				RefDes: "R1", CellRef: "R", LibraryRef: "L", ViewRef: "v",
				Transform: &geom.Transform{Origin: &geom.Point{X: 300, Y: 400}},
			}},
		}},
	}
	svc := NewDesignService(fakeLoader{geom: g}, noNative{}, render.Style{}, nil)
	specs := []*geom.HighlightSpec{{Components: []string{"R1"}, Nets: []string{"NET1"}, Color: "#ff0000"}}

	// PACKED (the default): primitives 0 (NET1 wire), 1 (R1 rect), 2 (R1 pin).
	resp, err := svc.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{
		Uri: "mount://m/x.eds", Specs: specs,
	})
	if err != nil {
		t.Fatal(err)
	}
	packed := resp.GetPacked()
	if packed == nil || len(packed.GetGroups()) != 1 {
		t.Fatalf("want 1 packed group, got %+v", resp.GetContent())
	}
	if got := packed.GetGroups()[0].GetPrimitives(); len(got) != 3 {
		t.Errorf("group primitives = %v, want the wire + rect + pin (3)", got)
	}

	// SVG: a transparent overlay document carrying the spec color.
	resp, err = svc.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{
		Uri: "mount://m/x.eds", Format: webapi.SheetFormat_SHEET_FORMAT_SVG, Specs: specs,
	})
	if err != nil {
		t.Fatal(err)
	}
	svg := resp.GetSvg()
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, `stroke="#ff0000"`) {
		t.Errorf("svg overlay missing root/spec color: %q", svg)
	}

	// NATIVE has no overlay: classified invalid-argument for the transport.
	if _, err := svc.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{
		Uri: "mount://m/x.eds", Format: webapi.SheetFormat_SHEET_FORMAT_NATIVE,
	}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument for NATIVE, got %v", err)
	}
}

// TestSecondSourceFlowsThroughService is the WS3-006 done-when: an embedder source's rules
// reach ListRules under their namespaced name with the source tag stamped, and the
// CheckDesign subset runs them by that name — the same seam the built-ins use.
func TestSecondSourceFlowsThroughService(t *testing.T) {
	everyNet := &check.Rule{
		Name: "every-net", Severity: "info", Summary: "fires per net (test source)",
		Reads: []string{"net.names"},
		Tags:  map[string]string{check.KeyCategory: check.CategoryNaming},
		Eval: check.FailuresOnly(func(m check.Model) []check.Finding {
			var out []check.Finding
			for _, n := range m.Nets() {
				out = append(out, check.Finding{Subject: check.Entity{Kind: check.KindNet, Ref: n.Name}, Message: "seen"})
			}
			return out
		}),
	}
	catalog, err := check.NewCatalog(check.Builtins, check.NewSource("demo", []*check.Rule{everyNet}))
	if err != nil {
		t.Fatal(err)
	}
	d := &ir.Design{Nets: []*ir.Net{{
		Name:        "N1",
		Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U2", PinRef: "1"}},
	}}}
	svc := NewCheckService(fakeLoader{design: d}, catalog, nil, "", nil, nil)

	list, err := svc.ListRules(context.Background(), &webapi.ListRulesRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	var demo *webapi.RuleInfo
	for _, r := range list.GetRules() {
		if r.GetName() == "demo/every-net" {
			demo = r
		}
	}
	if demo == nil || demo.GetTags()["source"] != "demo" {
		t.Fatalf("demo/every-net missing or untagged in ListRules: %+v", demo)
	}

	sub, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{
		Uri: "mount://m/x.edn", Rules: []string{"demo/every-net"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sub.GetFindings()) != 1 || sub.GetFindings()[0].GetRule() != "demo/every-net" ||
		sub.GetFindings()[0].GetSubject().GetRef() != "N1" {
		t.Fatalf("subset run of the second source = %+v", sub.GetFindings())
	}
}

// boardLoader is a fakeLoader whose file also has a board sidecar (a .kicad_pcb): GetDesign
// must list the synthetic board sheet after the drawable ones, and GetSheet/HighlightSheet
// must answer for it — SVG from BoardSVG, PACKED from PackBoard (WS7-035).
type boardLoader struct {
	fakeLoader
	board *geom.BoardGeometry
}

func (b boardLoader) Board(context.Context, artifact.URI) (*geom.BoardGeometry, error) {
	return b.board, nil
}

func testBoard() *geom.BoardGeometry {
	return &geom.BoardGeometry{
		Layers: []*geom.BoardLayer{{Number: 0, Name: "F.Cu", Kind: "signal"}},
		Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{
			{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 800}, {X: 0, Y: 800}, {X: 0, Y: 0},
		}}}},
		Nets: []*geom.NetCopper{{Net: "SIG", Segments: []*geom.TrackSegment{
			{A: &geom.Point{X: 10, Y: 10}, B: &geom.Point{X: 500, Y: 400}, Width: 20, Layer: "F.Cu"},
		}}},
	}
}

func TestGetDesignListsBoardSheet(t *testing.T) {
	ld := boardLoader{
		fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph", Name: "netlist graph"}}}},
		board:      testBoard(),
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: "mount://m/x.kicad_pcb"})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range resp.GetSheets() {
		ids = append(ids, s.GetId())
	}
	if len(ids) != 2 || ids[1] != "board" {
		t.Fatalf("sheets = %v, want the drawable sheets plus a trailing board sheet", ids)
	}
	if resp.GetSheets()[1].GetName() != "Board" {
		t.Errorf("board sheet name = %q, want Board", resp.GetSheets()[1].GetName())
	}
}

func TestGetSheetBoard(t *testing.T) {
	ld := boardLoader{
		fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}},
		board:      testBoard(),
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Uri: "mount://m/x.kicad_pcb", Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_SVG})
	if err != nil {
		t.Fatal(err)
	}
	svgDoc, ok := resp.GetContent().(*webapi.GetSheetResponse_Svg)
	if !ok || !strings.Contains(svgDoc.Svg, "copper-front") {
		t.Fatalf("SVG format must answer with the board document, ok=%v", ok)
	}
	// PACKED gets the WS7-035 packed board: same envelope, board sheet id, triangle records.
	resp, err = svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Uri: "mount://m/x.kicad_pcb", Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_PACKED})
	if err != nil {
		t.Fatal(err)
	}
	packed, ok := resp.GetContent().(*webapi.GetSheetResponse_Packed)
	if !ok || packed.Packed.GetSheetId() != "board" || len(packed.Packed.GetPrimitives()) == 0 {
		t.Fatalf("PACKED format must answer with the packed board, ok=%v", ok)
	}
}

func TestHighlightSheetBoard(t *testing.T) {
	ld := boardLoader{
		fakeLoader: fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}},
		board:      testBoard(),
	}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	resp, err := svc.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{
		Uri: "mount://m/x.kicad_pcb", Sheet: "board", Format: webapi.SheetFormat_SHEET_FORMAT_SVG,
		Specs: []*geom.HighlightSpec{{Nets: []string{"SIG"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	svgDoc, ok := resp.GetContent().(*webapi.HighlightSheetResponse_Svg)
	if !ok || !strings.Contains(svgDoc.Svg, "<line") {
		t.Fatalf("board highlight must be the SVG overlay with the matched copper, got ok=%v", ok)
	}
}

// TestBoardSheetAbsent: a loader without board geometry (nil, nil — absence is normal)
// lists no board sheet, and asking for one is ErrNotFound.
func TestBoardSheetAbsent(t *testing.T) {
	ld := fakeLoader{geom: &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "graph"}}}}
	svc := NewDesignService(ld, noNative{}, render.DefaultStyle, nil)
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: "mount://m/x.edn"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetSheets()) != 1 {
		t.Fatalf("no-board file lists %d sheets, want 1", len(resp.GetSheets()))
	}
	if _, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Uri: "mount://m/x.edn", Sheet: "board"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("board sheet on a boardless file = %v, want ErrNotFound", err)
	}
}

// TestHighlightSheetCompanionNameJoin: on a NAME-ONLY canvas (a companion .eds: wires named by net,
// no per-instance net_id), an id-only net spec (as the web builds for a netlist finding) is resolved
// to its net NAME via the netlist and matches by name. Without the resolution the id-only spec would
// paint nothing on such a canvas.
func TestHighlightSheetCompanionNameJoin(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id:    "P1",
		Wires: []*geom.WireGeometry{{Net: "SIGA", Polylines: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}}}}},
	}}}
	d := &ir.Design{Nets: []*ir.Net{{Id: "n1", Name: "SIGA"}}}
	svc := NewDesignService(fakeLoader{design: d, geom: g}, noNative{}, render.Style{}, nil)

	resp, err := svc.HighlightSheet(context.Background(), &webapi.HighlightSheetRequest{
		Uri: "mount://m/x.edn", Sheet: "P1",
		Format: webapi.SheetFormat_SHEET_FORMAT_SVG,
		Specs:  []*geom.HighlightSpec{{NetIds: []string{"n1"}}},
	})
	if err != nil {
		t.Fatalf("highlight: %v", err)
	}
	if !strings.Contains(resp.GetSvg(), "stroke-opacity") {
		t.Errorf("an id-only net spec should name-join on the companion canvas and paint; svg:\n%s", resp.GetSvg())
	}
}

// TestNameOnlyCanvas: a canvas with named wires but no net_id is name-only (the .eds case); one with
// any net_id is id-capable (leave it to the id-join); an empty one is neither.
func TestNameOnlyCanvas(t *testing.T) {
	nameOnly := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Wires: []*geom.WireGeometry{{Net: "A"}}}}}
	idCapable := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Wires: []*geom.WireGeometry{{Net: "A", NetId: "h1"}}}}}
	empty := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{}}}
	if !nameOnlyCanvas(nameOnly) {
		t.Error("named wires + no net_id should be name-only")
	}
	if nameOnlyCanvas(idCapable) {
		t.Error("a net_id-bearing canvas is not name-only")
	}
	if nameOnlyCanvas(empty) {
		t.Error("no wires is not name-only")
	}
}

// uriStr builds an artifact URI string for a request literal in a test. A fixture URI that will not
// parse is a broken test rather than a condition under test, so it panics instead of returning an
// error nobody would check.
func uriStr(mount, p string) string {
	u, err := artifact.New(mount, p)
	if err != nil {
		panic(err)
	}
	return u.String()
}

// TestGetDesignReportsUndrawnPlacements: a viewer cannot tell a complete sheet from a bodyless one by
// looking at it, because a render that lost its symbols still draws every reference designator, every
// wire and the title block. The shortfall has to arrive as data (agni issue 354).
func TestGetDesignReportsUndrawnPlacements(t *testing.T) {
	g := twoSheetGeom()
	g.Undrawn = []*geom.UndrawnPlacement{{RefDes: "U1", CellRef: "MCU", LibraryRef: "Acme", SheetId: "P1"}}
	svc := NewDesignService(fakeLoader{geom: g}, noNative{}, render.Style{}, nil)
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: "mount://m/x.eds"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetUndrawn()) != 1 {
		t.Fatalf("undrawn = %d, want 1", len(resp.GetUndrawn()))
	}
	if u := resp.GetUndrawn()[0]; u.GetRefDes() != "U1" || u.GetCellRef() != "MCU" {
		t.Errorf("undrawn = %+v, want the placement and the symbol it wanted", u)
	}

	// The common case, and the one that keeps a banner worth reading.
	clean := NewDesignService(fakeLoader{geom: twoSheetGeom()}, noNative{}, render.Style{}, nil)
	got, err := clean.GetDesign(context.Background(), &webapi.GetDesignRequest{Uri: "mount://m/x.eds"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.GetUndrawn()) != 0 {
		t.Errorf("a complete geometry must report nothing undrawn, got %+v", got.GetUndrawn())
	}
}
