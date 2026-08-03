package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/native"
	"github.com/panyam/agni/internal/service"
	"github.com/panyam/agni/render"
)

// newDesignSvc builds a DesignService over the os-backed adapters for the given mounts (no native
// tools enabled, default render style) — the same composition serve.go wires.
func newDesignSvc(mounts []mounts.Mount) *service.DesignService {
	return newDesignSvcNative(mounts, nil)
}

// newDesignSvcNative is newDesignSvc with a native-tool allowlist, for the native-gate tests.
func newDesignSvcNative(mounts []mounts.Mount, enabled map[string]bool) *service.DesignService {
	return service.NewDesignService(
		&osLoader{mounts: mounts},
		&osNative{mounts: mounts, enabled: enabled, cache: native.NewCache()},
		render.Style{},
	)
}

// newCheckSvc builds a CheckService over the same os-backed loader, with the built-in catalog.
func newCheckSvc(mounts []mounts.Mount) *service.CheckService {
	return service.NewCheckService(&osLoader{mounts: mounts}, check.DefaultCatalog(), nil)
}

// designFixtureSvc mounts the shared edif testdata (basic.edn netlist, sample.eds geometry)
// relative to this package.
func designFixtureSvc(t *testing.T) *service.DesignService {
	t.Helper()
	return newDesignSvc(edifFixtureMounts(t))
}

func edifFixtureMounts(t *testing.T) []mounts.Mount {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "readers", "edif", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	return []mounts.Mount{{Name: "t", Root: root}}
}

func TestDesignServiceGetDesign(t *testing.T) {
	svc := designFixtureSvc(t)
	get := func(path string) (*webapi.GetDesignResponse, error) {
		return svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "t", Path: path})
	}

	t.Run("netlist gets grid layout, sheets, and IR counts", func(t *testing.T) {
		msg, err := get("basic.edn")
		if err != nil {
			t.Fatal(err)
		}
		if msg.GetLayout() != "grid" {
			t.Errorf("layout = %q, want grid", msg.GetLayout())
		}
		if len(msg.GetSheets()) == 0 {
			t.Error("want at least one drawable sheet")
		}
		if msg.GetComponentCount() == 0 || msg.GetNetCount() == 0 {
			t.Errorf("want non-zero IR counts, got components=%d nets=%d", msg.GetComponentCount(), msg.GetNetCount())
		}
	})

	t.Run("eds gets faithful layout and sheets", func(t *testing.T) {
		msg, err := get("sample.eds")
		if err != nil {
			t.Fatal(err)
		}
		if msg.GetLayout() != faithfulLayout {
			t.Errorf("layout = %q, want faithful", msg.GetLayout())
		}
		if len(msg.GetSheets()) == 0 {
			t.Error("want at least one faithful sheet")
		}
	})

	t.Run("unknown mount classifies as not-found", func(t *testing.T) {
		_, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "nope", Path: "basic.edn"})
		if !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("traversal keeps its invalid-path classification", func(t *testing.T) {
		_, err := get("../../secret")
		if !errors.Is(err, service.ErrInvalidPath) {
			t.Fatalf("want ErrInvalidPath, got %v", err)
		}
	})
}

func TestDesignServiceCheckDesign(t *testing.T) {
	// A self-contained netlist fixture with known violations: two I2C nets with no pull-up
	// (i2c-pull-up), and a resistor on no net (unconnected-component). basic.edn is clean, so it
	// would not exercise the endpoint.
	fixture := `(edif CHK
  (edifVersion 2 0 0)
  (design CHK (cellRef TOP (libraryRef LIB)))
  (library LIB
    (cell TOP
      (view V (viewType NETLIST) (interface)
        (contents
          (instance U1 (viewRef V (cellRef IC)) (designator "U1"))
          (instance U2 (viewRef V (cellRef IC)) (designator "U2"))
          (instance R9 (viewRef V (cellRef RES)) (designator "R9"))
          (net SDA (joined (portRef 5 (instanceRef U1)) (portRef 5 (instanceRef U2))))
          (net SCL (joined (portRef 6 (instanceRef U1)) (portRef 6 (instanceRef U2)))))))))
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chk.edn"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newCheckSvc([]mounts.Mount{{Name: "t", Root: dir}})

	resp, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Mount: "t", Path: "chk.edn"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{} // "rule|subject" -> severity
	for _, f := range resp.GetFindings() {
		got[f.GetRule()+"|"+f.GetSubject().GetRef()] = f.GetSeverity()
	}
	for _, key := range []string{"i2c-pull-up|SDA", "i2c-pull-up|SCL", "unconnected-component|R9"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing finding %q; got %v", key, got)
		}
	}
	if got["i2c-pull-up|SDA"] != "error" {
		t.Errorf("i2c-pull-up severity = %q, want error", got["i2c-pull-up|SDA"])
	}

	t.Run("unknown mount classifies as not-found", func(t *testing.T) {
		_, err := svc.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Mount: "nope", Path: "chk.edn"})
		if !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("an EDIF schematic (.eds) checks via its netlist, not an invalid-argument dead end", func(t *testing.T) {
		edif := newCheckSvc(edifFixtureMounts(t)) // sample.eds: an EDIF schematic view carries a netlist
		_, err := edif.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Mount: "t", Path: "sample.eds"})
		if err != nil {
			t.Fatalf("a .eds now carries a netlist and should check, got %v", err)
		}
	})
}

// TestDesignServiceGetCheckReport pins the report pivot on the wire: sections come worst
// severity first with correct counts, findings group by rule, and each group carries the
// catalog Summary. The conformance fires.edn yields one finding each of error/warning/info,
// exercising the full ordering through the same loader path CheckDesign uses.
func TestDesignServiceGetCheckReport(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "conformance"))
	if err != nil {
		t.Fatal(err)
	}
	svc := newCheckSvc([]mounts.Mount{{Name: "t", Root: root}})
	resp, err := svc.GetCheckReport(context.Background(), &webapi.GetCheckReportRequest{Mount: "t", Path: "fires.edn"})
	if err != nil {
		t.Fatal(err)
	}
	rep := resp.GetReport()
	if rep.GetSource() != "fires.edn" || rep.GetRulesRun() == 0 {
		t.Errorf("report header = %q / %d rules, want fires.edn / >0", rep.GetSource(), rep.GetRulesRun())
	}
	var sevs []string
	for _, s := range rep.GetSections() {
		sevs = append(sevs, s.GetSeverity())
		if s.GetCount() != 1 {
			t.Errorf("section %s count = %d, want 1", s.GetSeverity(), s.GetCount())
		}
	}
	if len(sevs) != 3 || sevs[0] != "error" || sevs[1] != "warning" || sevs[2] != "info" {
		t.Errorf("sections = %v, want [error warning info]", sevs)
	}
	g := rep.GetSections()[0].GetRules()[0]
	if g.GetRule() != "i2c-pull-up" || g.GetSummary() == "" {
		t.Errorf("error group = %q (summary %q), want i2c-pull-up with its catalog summary", g.GetRule(), g.GetSummary())
	}
	if len(g.GetFindings()) != 1 || g.GetFindings()[0].GetSubject().GetRef() != "SDA" {
		t.Errorf("i2c-pull-up findings = %v, want [SDA]", g.GetFindings())
	}
}

func TestDesignServiceGetExpectations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "d.edn"), []byte("(edif D (edifVersion 2 0 0))"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "d.edn.expect.yaml"), []byte(`fires:
  single-pin-net: [STUB]
  decoupling-present:
    subjects: [VCC1]
    why: "VCC1 has no cap"
pending:
  net-naming-convention: [BAD]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := newCheckSvc([]mounts.Mount{{Name: "t", Root: dir}})

	resp, err := svc.GetExpectations(context.Background(), &webapi.GetExpectationsRequest{Mount: "t", Path: "d.edn"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{} // "rule|subject" -> pending
	for _, e := range resp.GetExpectations() {
		for _, s := range e.GetSubjects() {
			got[e.GetRule()+"|"+s] = e.GetPending()
		}
	}
	if p, ok := got["single-pin-net|STUB"]; !ok || p {
		t.Errorf("single-pin-net|STUB pending=%v ok=%v, want present & not pending", p, ok)
	}
	if p, ok := got["net-naming-convention|BAD"]; !ok || !p {
		t.Errorf("net-naming-convention|BAD pending=%v ok=%v, want present & pending", p, ok)
	}
	whyByRule := map[string]string{}
	for _, e := range resp.GetExpectations() {
		whyByRule[e.GetRule()] = e.GetWhy()
	}
	if whyByRule["decoupling-present"] != "VCC1 has no cap" {
		t.Errorf("decoupling-present why = %q, want the sidecar note on the wire", whyByRule["decoupling-present"])
	}
	if whyByRule["single-pin-net"] != "" {
		t.Errorf("short-form entry why = %q, want empty", whyByRule["single-pin-net"])
	}

	t.Run("no sidecar returns empty, not an error", func(t *testing.T) {
		clean := newCheckSvc(edifFixtureMounts(t)) // basic.edn has no sidecar
		r, err := clean.GetExpectations(context.Background(), &webapi.GetExpectationsRequest{Mount: "t", Path: "basic.edn"})
		if err != nil {
			t.Fatalf("missing sidecar should not error: %v", err)
		}
		if len(r.GetExpectations()) != 0 {
			t.Errorf("no sidecar should yield empty, got %d", len(r.GetExpectations()))
		}
	})
}

func TestDesignServiceKicadFaithful(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	svc := newDesignSvc([]mounts.Mount{{Name: "k", Root: root}})

	t.Run("kicad_sch renders faithfully with a packed sheet", func(t *testing.T) {
		d, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "k", Path: "geom.kicad_sch"})
		if err != nil {
			t.Fatal(err)
		}
		if d.GetLayout() != faithfulLayout {
			t.Fatalf("layout = %q, want faithful", d.GetLayout())
		}
		if len(d.GetSheets()) == 0 {
			t.Fatal("want at least one faithful sheet")
		}
		s, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Mount: "k", Path: "geom.kicad_sch"})
		if err != nil {
			t.Fatal(err)
		}
		if sh := s.GetPacked(); sh == nil || len(sh.GetPrimitives()) == 0 {
			t.Fatalf("want a packed sheet with primitives, got %+v", sh)
		}
	})

	t.Run("kicad_pcb falls back to grid", func(t *testing.T) {
		d, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "k", Path: "pcb.kicad_pcb"})
		if err != nil {
			t.Fatal(err)
		}
		if d.GetLayout() != "grid" {
			t.Errorf("kicad_pcb layout = %q, want grid (no faithful board render)", d.GetLayout())
		}
	})

	// The GetSheet symbols field is honored (WS7-031b): a grid render with FAITHFUL symbols draws
	// the design's own artwork, so its SVG differs from the GLYPH render. Before un-hardcoding the
	// source, both requests produced the same glyph SVG.
	t.Run("grid honors the symbols choice", func(t *testing.T) {
		render := func(sym webapi.SymbolSource) string {
			r, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{
				Mount: "k", Path: "geom.kicad_sch", Layout: "grid",
				Format: webapi.SheetFormat_SHEET_FORMAT_SVG, Symbols: sym,
			})
			if err != nil {
				t.Fatal(err)
			}
			return r.GetSvg()
		}
		if render(webapi.SymbolSource_SYMBOL_SOURCE_GLYPH) == render(webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL) {
			t.Error("FAITHFUL symbols produced identical SVG to GLYPH — the symbols field was ignored")
		}
	})
}

// TestDesignServiceGetLayoutReport covers the report RPC (WS7-029b): every component is classified with
// a kind, and because KiCad symbols are inline, a faithful request resolves them all as provided.
func TestDesignServiceGetLayoutReport(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	svc := newDesignSvc([]mounts.Mount{{Name: "k", Root: root}})
	get := func(sym webapi.SymbolSource) *webapi.ConversionReport {
		r, err := svc.GetLayoutReport(context.Background(), &webapi.GetLayoutReportRequest{Mount: "k", Path: "geom.kicad_sch", Symbols: sym})
		if err != nil {
			t.Fatal(err)
		}
		return r.GetReport()
	}

	glyph := get(webapi.SymbolSource_SYMBOL_SOURCE_GLYPH)
	if len(glyph.GetComponents()) == 0 {
		t.Fatal("glyph report has no components")
	}
	for _, c := range glyph.GetComponents() {
		if c.GetRefDes() == "" || c.GetKind() == "" {
			t.Errorf("malformed component report: %+v", c)
		}
	}

	provided := 0
	for _, c := range get(webapi.SymbolSource_SYMBOL_SOURCE_FAITHFUL).GetComponents() {
		if c.GetKind() == "provided" {
			provided++
		}
	}
	if provided == 0 {
		t.Error("faithful report: want provided symbols for inline KiCad, got none")
	}
}

func TestSymbolsFor(t *testing.T) {
	if got := symbolsFor(true); got != symbolsFaithful {
		t.Errorf("symbolsFor(true) = %q, want %q", got, symbolsFaithful)
	}
	if got := symbolsFor(false); got != symbolsGlyph {
		t.Errorf("symbolsFor(false) = %q, want %q", got, symbolsGlyph)
	}
}

func TestDesignServiceGetSheetFormats(t *testing.T) {
	svc := designFixtureSvc(t)
	req := func(format webapi.SheetFormat) *webapi.GetSheetRequest {
		return &webapi.GetSheetRequest{Mount: "t", Path: "sample.eds", Format: format}
	}

	t.Run("SVG format returns an svg document", func(t *testing.T) {
		resp, err := svc.GetSheet(context.Background(), req(webapi.SheetFormat_SHEET_FORMAT_SVG))
		if err != nil {
			t.Fatal(err)
		}
		if svg := resp.GetSvg(); !strings.Contains(svg, "<svg") {
			t.Fatalf("want an <svg> document, got %.60q", svg)
		}
	})

	t.Run("NATIVE format gates as no-tool", func(t *testing.T) {
		_, err := svc.GetSheet(context.Background(), req(webapi.SheetFormat_SHEET_FORMAT_NATIVE))
		if !errors.Is(err, service.ErrNativeNoTool) {
			t.Fatalf("want ErrNativeNoTool, got %v", err)
		}
	})
}

func TestDesignServiceMultiSheet(t *testing.T) {
	svc := designFixtureSvc(t)

	t.Run("GetDesign lists both sheets in order", func(t *testing.T) {
		resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "t", Path: "multisheet.eds"})
		if err != nil {
			t.Fatal(err)
		}
		sheets := resp.GetSheets()
		if len(sheets) != 2 || sheets[0].GetId() != "P1" || sheets[1].GetId() != "P2" {
			t.Fatalf("want [P1 P2], got %+v", sheets)
		}
		// EDIF pages are flat, so both sheets are top-level (no parent).
		if sheets[0].GetParentId() != "" || sheets[1].GetParentId() != "" {
			t.Errorf("want empty parent_id for flat EDIF pages, got %q/%q", sheets[0].GetParentId(), sheets[1].GetParentId())
		}
	})

	packed := func(sheet string) int {
		resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Mount: "t", Path: "multisheet.eds", Sheet: sheet})
		if err != nil {
			t.Fatalf("GetSheet %q: %v", sheet, err)
		}
		return len(resp.GetPacked().GetPrimitives())
	}

	t.Run("sheets select distinct content by id and index", func(t *testing.T) {
		p1, p2 := packed("P1"), packed("P2")
		if p1 == 0 || p2 == 0 || p1 == p2 {
			t.Fatalf("want distinct non-empty sheets, got P1=%d P2=%d primitives", p1, p2)
		}
		// The 0-based index selects the same sheet as the id.
		if byIndex := packed("1"); byIndex != p2 {
			t.Errorf("sheet index 1 = %d primitives, want P2's %d", byIndex, p2)
		}
	})
}

func TestDesignServiceKicadHierarchy(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	svc := newDesignSvc([]mounts.Mount{{Name: "k", Root: root}})
	resp, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "k", Path: "hier.kicad_sch"})
	if err != nil {
		t.Fatal(err)
	}
	sheets := resp.GetSheets()
	// The edge resolves the sub-sheet's Sheetfile relative to the root, so GetDesign returns
	// the root plus its child with the parent link set.
	if len(sheets) != 2 {
		t.Fatalf("sheets = %d, want 2 (root + sub)", len(sheets))
	}
	if sheets[0].GetParentId() != "" {
		t.Errorf("root parent_id = %q, want empty", sheets[0].GetParentId())
	}
	if sheets[1].GetName() != "Sub A" || sheets[1].GetParentId() != sheets[0].GetId() {
		t.Errorf("sub = (name=%q parent=%q), want (Sub A, %q)", sheets[1].GetName(), sheets[1].GetParentId(), sheets[0].GetId())
	}
}

// TestLayoutAxis covers the layout axis end to end through GetDesign; the pure availableLayouts /
// layoutForFile helpers moved to internal/service and are unit-tested there.
func TestLayoutAxis(t *testing.T) {
	t.Run("GetDesign reports available_layouts and grid re-derives", func(t *testing.T) {
		root, _ := filepath.Abs(filepath.Join("..", "..", "readers", "kicad", "testdata"))
		svc := newDesignSvc([]mounts.Mount{{Name: "k", Root: root}})
		faithful, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "k", Path: "geom.kicad_sch"})
		if err != nil {
			t.Fatal(err)
		}
		if !slicesEqual(faithful.GetAvailableLayouts(), []string{"faithful", "force", "grid", "layered", "orthogonal", "stress"}) {
			t.Errorf("available_layouts = %v", faithful.GetAvailableLayouts())
		}
		if faithful.GetLayout() != "faithful" {
			t.Errorf("default layout = %q, want faithful", faithful.GetLayout())
		}
		grid, err := svc.GetDesign(context.Background(), &webapi.GetDesignRequest{Mount: "k", Path: "geom.kicad_sch", Layout: "grid"})
		if err != nil {
			t.Fatal(err)
		}
		if grid.GetLayout() != "grid" {
			t.Errorf("requested grid, effective = %q", grid.GetLayout())
		}
	})
}

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

func TestDesignServiceGetSheet(t *testing.T) {
	svc := designFixtureSvc(t)

	t.Run("default format packs geometry", func(t *testing.T) {
		resp, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Mount: "t", Path: "sample.eds", Sheet: "0"})
		if err != nil {
			t.Fatal(err)
		}
		sheet := resp.GetPacked()
		if sheet == nil || len(sheet.GetVertices()) == 0 || len(sheet.GetPrimitives()) == 0 {
			t.Fatalf("want a packed sheet with vertices and primitives, got %+v", sheet)
		}
	})

	t.Run("empty selector picks the first sheet", func(t *testing.T) {
		if _, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Mount: "t", Path: "sample.eds"}); err != nil {
			t.Fatalf("empty selector errored: %v", err)
		}
	})

	t.Run("bad sheet selector classifies as not-found", func(t *testing.T) {
		_, err := svc.GetSheet(context.Background(), &webapi.GetSheetRequest{Mount: "t", Path: "sample.eds", Sheet: "no-such-sheet"})
		if !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
