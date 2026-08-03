package derive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/doc"
	"github.com/panyam/agni/datasheet/param"
)

func loadDocFixture(t *testing.T, name string) *docpb.Document {
	t.Helper()
	fh, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	dd, err := doc.Load(fh)
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(dd); err != nil {
		t.Fatalf("doc fixture invalid: %v", err)
	}
	return dd
}

func loadArtifacts(t *testing.T) ([]*derivepb.Recipe, []*derivepb.Patch) {
	t.Helper()
	rs, err := LoadRecipes(os.DirFS("testdata/recipes"))
	if err != nil {
		t.Fatalf("LoadRecipes: %v", err)
	}
	ps, err := LoadPatches(os.DirFS("testdata/patches"))
	if err != nil {
		t.Fatalf("LoadPatches: %v", err)
	}
	return rs, ps
}

func runFixture(t *testing.T) (*parampb.PartSpec, *derivepb.RunManifest) {
	t.Helper()
	fx := loadDocFixture(t, "bss138-raw-docir.textproto")
	recipes, patches := loadArtifacts(t)
	spec, manifest, err := Run(fx, recipes, patches, Identity{
		MPN: "BSS138", Manufacturer: "Fairchild Semiconductor", DeviceClass: "nfet",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return spec, manifest
}

func TestRunEmitsValidatedSpec(t *testing.T) {
	spec, manifest := runFixture(t)
	if err := param.Validate(spec); err != nil {
		t.Fatalf("derived spec must validate: %v", err)
	}
	if spec.Mpn != "BSS138" || spec.DeviceClass != "nfet" {
		t.Errorf("identity not carried: %s/%s", spec.Mpn, spec.DeviceClass)
	}
	if int(manifest.ParametersEmitted) != len(spec.Parameters) {
		t.Errorf("manifest count %d != emitted %d", manifest.ParametersEmitted, len(spec.Parameters))
	}
	if len(spec.Docs) != 1 || spec.Docs[0].Title == "" {
		t.Errorf("derived spec must cite its source document, got %v", spec.Docs)
	}
	for _, p := range spec.Parameters {
		if p.Prov.GetMethod() != "derive/v0" || p.Prov.GetConfidence() >= 1 {
			t.Errorf("%s: derived provenance must be method derive/v0 with confidence < 1 (only humans get 1.0), got %+v", p.Symbol, p.Prov)
		}
	}
}

// The title-attachment stage: the raw fixture's tables are untitled (the real
// producer's shape); their limit kinds can only come from headings attached from
// nearby text blocks.
func TestRunAttachesTitlesAndClassifies(t *testing.T) {
	spec, _ := runFixture(t)
	kinds := map[parampb.LimitKind]int{}
	for _, p := range spec.Parameters {
		kinds[p.LimitKind]++
	}
	if kinds[parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX] != 4 {
		t.Errorf("want 4 abs-max rows (VDSS, VGSS, ID, TJ/TSTG), got %d", kinds[parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX])
	}
	if kinds[parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC] != 4 {
		t.Errorf("want 4 characteristic rows (VGS(th) + 3x RDS(on)), got %d", kinds[parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC])
	}
}

// The patch stage: the raw fixture deliberately mis-encodes VDSS as 500; the patch
// (keyed by doc + pre-patch table content hash) corrects it to 50, and the manifest
// records the application.
func TestRunAppliesPatchLast(t *testing.T) {
	spec, manifest := runFixture(t)
	var vdss *parampb.Parameter
	for _, p := range spec.Parameters {
		if normalizeSymbol(p.Symbol) == "VDSS" {
			vdss = p
		}
	}
	if vdss == nil {
		t.Fatal("VDSS not derived")
	}
	if vdss.Value.GetMax() != 50 {
		t.Errorf("VDSS max = %v, want 50 (patch must correct the mis-parsed 500)", vdss.Value.GetMax())
	}
	if len(manifest.PatchesApplied) != 1 {
		t.Errorf("manifest must record the applied patch, got %v", manifest.PatchesApplied)
	}
}

// Coverage honesty: rows from a table with no test-conditions channel come out
// UNSPECIFIED (under-specified until a human verifies), never UNCONDITIONAL; rows
// with a captured conditions column come out COMPLETE, with unparsed parts kept as
// raw-only conditions.
func TestRunConditionCoverage(t *testing.T) {
	spec, _ := runFixture(t)
	for _, p := range spec.Parameters {
		switch p.LimitKind {
		case parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX:
			if p.ConditionCoverage != parampb.ConditionCoverage_CONDITION_COVERAGE_UNSPECIFIED {
				t.Errorf("%s: abs-max table has no conditions channel; coverage must stay UNSPECIFIED, got %v", p.Symbol, p.ConditionCoverage)
			}
			if !param.UnderSpecified(p) {
				t.Errorf("%s: derived-unverified abs-max row must be under-specified", p.Symbol)
			}
		case parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC:
			if p.ConditionCoverage != parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE {
				t.Errorf("%s: captured conditions column must yield COMPLETE, got %v", p.Symbol, p.ConditionCoverage)
			}
		}
	}
	var vgsth *parampb.Parameter
	for _, p := range spec.Parameters {
		if normalizeSymbol(p.Symbol) == "VGS(th)" {
			vgsth = p
		}
	}
	if vgsth == nil {
		t.Fatal("VGS(th) not derived")
	}
	var sawRaw bool
	for _, c := range vgsth.Conditions {
		if c.Eq == nil && c.Min == nil && c.Max == nil && c.Raw != "" {
			sawRaw = true
		}
	}
	if !sawRaw {
		t.Errorf("VGS(th)'s 'VDS = VGS' condition must be kept raw-only, got %v", vgsth.Conditions)
	}
	if param.MachineComparable(vgsth) {
		t.Errorf("a raw-only condition must make the row machine-incomparable")
	}
}

// The golden gate (docs/17: verified values are the regression corpus): every row of
// the hand-encoded WS10-001 fixture must have a derived row agreeing on (symbol,
// kind, min/typ/max, unit). Conditions are compared by the coverage tests above, not
// here (the hand encoding lifts footnote defaults into conditions; v0 does not).
func TestGoldenAgreementWithHandEncoded(t *testing.T) {
	spec, _ := runFixture(t)
	fh, err := os.Open(filepath.Join("..", "param", "testdata", "bss138.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	golden, err := param.Load(fh)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range golden.Parameters {
		found := false
		for _, got := range spec.Parameters {
			if normalizeSymbol(got.Symbol) == normalizeSymbol(want.Symbol) &&
				got.LimitKind == want.LimitKind &&
				eqPtr(got.Value.Min, want.Value.Min) &&
				eqPtr(got.Value.Typ, want.Value.Typ) &&
				eqPtr(got.Value.Max, want.Value.Max) &&
				got.Unit == want.Unit {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("hand-encoded row not reproduced by derivation: %s %v %v %s",
				want.Symbol, want.LimitKind, want.Value, want.Unit)
		}
	}
}

func TestRunGapsAndManifest(t *testing.T) {
	spec, manifest := runFixture(t)
	_ = spec
	if manifest.DocContentHash == "" || manifest.DeriveVersion == "" || manifest.DocProducer == "" {
		t.Errorf("manifest must pin the toolchain: %+v", manifest)
	}
	if len(manifest.Recipes) != 1 {
		t.Errorf("manifest must record the matched recipe, got %v", manifest.Recipes)
	}
	var kinds []string
	for _, g := range manifest.Gaps {
		kinds = append(kinds, g.Kind)
	}
	// The fixture's page-3 figure region is not a gap; but the unclassified filler
	// table on page 1 must be.
	if !strings.Contains(strings.Join(kinds, " "), "unclassified-table") {
		t.Errorf("the unclassified ordering-info table must appear in gaps, got %v", manifest.Gaps)
	}
}

// TI-shaped tables carry no Symbol column: the row label is an unlabeled column 0
// ("Maximum input voltage (VIN to GND)  |  MIN | MAX | UNIT"). The extractor falls
// back to column 0 as the parameter name; symbol stays empty, honestly.
func TestExtractSymbollessTable(t *testing.T) {
	d := &docpb.Document{
		ContentHash: "sha256:test", Producer: "hand", PageCount: 1, Title: "TI SHAPE",
		Pages: []*docpb.Page{{
			Number: 1, Width: 612, Height: 792,
			Tables: []*docpb.Table{{
				Id: "p1.t1", Title: "Absolute Maximum Ratings",
				Bbox: &docpb.BBox{X: 10, Y: 10, Width: 100, Height: 50},
				Rows: 2, Cols: 4, Confidence: 1,
				Cells: []*docpb.Cell{
					{Row: 0, Col: 1, Text: "MIN", IsHeader: true},
					{Row: 0, Col: 2, Text: "MAX", IsHeader: true},
					{Row: 0, Col: 3, Text: "UNIT", IsHeader: true},
					{Row: 1, Col: 0, Text: "Maximum input voltage (VIN to GND)"},
					{Row: 1, Col: 2, Text: "20"},
					{Row: 1, Col: 3, Text: "V"},
				},
			}},
		}},
	}
	d.Pages[0].Tables[0].ContentHash = doc.TableHash(d.Pages[0].Tables[0])
	recipes := []*derivepb.Recipe{{
		Name: "ti", DocTitlePattern: "TI SHAPE",
		Tables: []*derivepb.TableRule{{TitlePattern: "Absolute Maximum", LimitKind: "LIMIT_KIND_ABSOLUTE_MAX"}},
	}}
	spec, _, err := Run(d, recipes, nil, Identity{MPN: "LM1117"})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Parameters) != 1 {
		t.Fatalf("want 1 parameter from the symbol-less row, got %v", spec.Parameters)
	}
	p := spec.Parameters[0]
	if p.Name != "Maximum input voltage (VIN to GND)" || p.Symbol != "" || p.Value.GetMax() != 20 || p.Unit != "V" {
		t.Errorf("symbol-less row mis-extracted: %+v", p)
	}
}

// A patch may target a position with NO detected cell (real case: docling placed
// LM1117's abs-max 20 under MIN, leaving MAX empty; correcting means clearing one
// cell and writing another). Insert-if-absent plus empty-text-clears make a
// cell-placement error correctable with two patches.
func TestPatchInsertsAndClears(t *testing.T) {
	fx := loadDocFixture(t, "bss138-raw-docir.textproto")
	recipes, _ := loadArtifacts(t)
	abs, chr := fx.Pages[0].Tables[0], fx.Pages[1].Tables[0]
	patches := []*derivepb.Patch{
		// Rewrite of an existing cell (the fixture's mis-encoded 500).
		{Name: "fix-vdss", DocContentHash: fx.ContentHash, TableContentHash: abs.ContentHash,
			Row: 1, Col: 2, Text: "50", Note: "page prints 50"},
		// Insert at a position with NO detected cell: the RDS(on) VGS=10 row has an
		// empty Min column in the fixture grid.
		{Name: "insert-rds-min", DocContentHash: fx.ContentHash, TableContentHash: chr.ContentHash,
			Row: 2, Col: 3, Text: "0.5", Note: "demonstrates insert-if-absent"},
		// Clear an existing cell: empty text removes the VGS(th) Min bound.
		{Name: "clear-vgsth-min", DocContentHash: fx.ContentHash, TableContentHash: chr.ContentHash,
			Row: 1, Col: 3, Text: "", Note: "demonstrates clearing"},
	}
	spec, manifest, err := Run(fx, recipes, patches, Identity{MPN: "BSS138"})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.PatchesApplied) != 3 {
		t.Fatalf("all three patches must apply, got %v (gaps %v)", manifest.PatchesApplied, manifest.Gaps)
	}
	for _, p := range spec.Parameters {
		switch {
		case normalizeSymbol(p.Symbol) == "VDSS" && p.Value.GetMax() != 50:
			t.Errorf("VDSS after rewrite = %v, want max 50", p.Value)
		case normalizeSymbol(p.Symbol) == "RDS(on)" && p.Value.GetMax() == 3.5 && (p.Value.Min == nil || p.Value.GetMin() != 0.5):
			t.Errorf("RDS(on) VGS=10 row after insert = %v, want min 0.5", p.Value)
		case normalizeSymbol(p.Symbol) == "VGS(th)" && p.Value.Min != nil:
			t.Errorf("VGS(th) after clear = %v, want no min", p.Value)
		}
	}
}

func TestLoadPatchesRequiresNote(t *testing.T) {
	bad := fstest.MapFS{"p.textproto": {Data: []byte(
		"name: \"x\" doc_content_hash: \"sha256:aa\" table_content_hash: \"sha256:bb\" row: 1 col: 2 text: \"50\"\n")}}
	if _, err := LoadPatches(bad); err == nil || !strings.Contains(err.Error(), "note") {
		t.Fatalf("a correction without a why must not load, got %v", err)
	}
}
