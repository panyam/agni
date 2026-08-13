package derive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	derivepb "github.com/panyam/agni/gen/go/agni/v1/derive"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

func runPinFixture(t *testing.T) (*parampb.PartSpec, *derivepb.RunManifest) {
	t.Helper()
	fx := loadDocFixture(t, "txb0104-pintable-docir.textproto")
	recipes, patches := loadArtifacts(t)
	spec, manifest, err := Run(fx, recipes, patches, Identity{
		MPN: "TXB0104", Manufacturer: "Texas Instruments", DeviceClass: "level translator",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return spec, manifest
}

func numbersOf(spec *parampb.PartSpec, pinID string) map[string]string {
	out := map[string]string{}
	for _, p := range spec.Pins {
		if p.Id != pinID {
			continue
		}
		for _, n := range p.Numbers {
			out[n.PackageRef] = n.Number
		}
	}
	return out
}

// One header cell can name several packages sharing one column of designators, so the
// bodies a document ships in are not the number of columns it prints.
func TestPinTableDerivesPackagesFromHeaderCells(t *testing.T) {
	spec, manifest := runPinFixture(t)
	got := map[string]string{}
	for _, p := range spec.Packages {
		got[p.Id] = p.MpnSuffix
	}
	for _, want := range []string{"d", "pw", "bqa", "rgy", "rut", "gxu", "zxu", "nmn", "yzt"} {
		if _, ok := got[want]; !ok {
			t.Errorf("package %q not derived; got %v", want, got)
		}
	}
	if len(got) != 9 {
		t.Errorf("want 9 packages from 5 columns, got %d: %v", len(got), got)
	}
	if manifest.PackagesEmitted != 9 {
		t.Errorf("manifest.packages_emitted = %d, want 9", manifest.PackagesEmitted)
	}
	if manifest.PinsEmitted == 0 {
		t.Error("manifest.pins_emitted is 0; a pin table emitted no pins")
	}
}

// The golden gate for the pin path, mirroring TestGoldenAgreementWithHandEncoded: every
// pin of the hand-encoded WS10 fixture must have a derived pin agreeing on the packages
// that fixture declares. The hand encoding is the ORACLE; the derived spec is allowed to
// carry more (it reads all 13 rows and all 9 bodies), never to disagree.
func TestGoldenAgreementWithHandEncodedPins(t *testing.T) {
	derived, _ := runPinFixture(t)

	fh, err := os.Open(filepath.Join("..", "param", "testdata", "txb0104.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	oracle, err := param.Load(fh)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range oracle.Pins {
		got := numbersOf(derived, want.Id)
		if len(got) == 0 && len(want.Numbers) > 0 {
			t.Errorf("pin %q (%s): no derived pin with that id; derived ids %v",
				want.Id, want.Name, pinIDs(derived))
			continue
		}
		for _, n := range want.Numbers {
			if got[n.PackageRef] != n.Number {
				t.Errorf("pin %q package %q: derived %q, hand-encoded %q",
					want.Id, n.PackageRef, got[n.PackageRef], n.Number)
			}
		}
	}
}

func pinIDs(spec *parampb.PartSpec) []string {
	var out []string
	for _, p := range spec.Pins {
		out = append(out, p.Id)
	}
	return out
}

// The ambiguity this ticket turns on: "NC 6, 9" and a hypothetical "GND 2, 5, 7" are the
// same cell shape and mean opposite things. The split is keyed on the one function the
// document states in words, and BOTH outcomes are gapped so neither is trusted silently.
func TestNoConnectRowSplitsIntoSeparatePins(t *testing.T) {
	spec, manifest := runPinFixture(t)

	nc6, nc9 := numbersOf(spec, "nc6"), numbersOf(spec, "nc9")
	if nc6["pw"] != "6" || nc9["pw"] != "9" {
		t.Errorf("NC row must split into two pins: nc6=%v nc9=%v", nc6, nc9)
	}
	for _, p := range spec.Pins {
		if p.Id == "nc6" && p.Function != parampb.PinFunction_PIN_FUNCTION_NO_CONNECT {
			t.Errorf("nc6 function = %v, want NO_CONNECT (the type column says %q, the description says so in words)",
				p.Function, p.Attributes["function_raw"])
		}
	}
	var multi int
	for _, g := range manifest.Gaps {
		if g.Kind == "multi-designator-row" {
			multi++
			if !strings.Contains(g.Detail, "split into separate pins") {
				t.Errorf("gap does not say which way it went: %q", g.Detail)
			}
		}
	}
	if multi != 1 {
		t.Errorf("want exactly one multi-designator gap (the NC row), got %d", multi)
	}
}

// A pin absent from a body carries no PinNumber for it rather than an empty one, and the
// producer writes that absence as an ASCII hyphen rather than the printed em-dash.
func TestAbsentDesignatorYieldsNoNumber(t *testing.T) {
	spec, _ := runPinFixture(t)
	for _, id := range []string{"nc6", "nc9"} {
		got := numbersOf(spec, id)
		for _, body := range []string{"rut", "gxu", "yzt"} {
			if n, ok := got[body]; ok {
				t.Errorf("%s: NC is not in the %s body, but a number %q was recorded", id, body, n)
			}
		}
	}
}

// A row with a name and a description and no designator anywhere is legal in the contract
// and is what the real table's thermal-pad row is.
func TestRowWithNoDesignatorStillYieldsAPin(t *testing.T) {
	spec, _ := runPinFixture(t)
	var found *parampb.Pin
	for _, p := range spec.Pins {
		if strings.EqualFold(p.Name, "Thermal pad") {
			found = p
		}
	}
	if found == nil {
		t.Fatalf("thermal pad row yielded no pin; ids %v", pinIDs(spec))
	}
	if len(found.Numbers) != 0 {
		t.Errorf("thermal pad has no designator in any body, got %v", found.Numbers)
	}
	if found.Description == "" {
		t.Error("thermal pad description was dropped; it is the only thing the row says")
	}
}

// The type column declines to classify supply and ground rows on this real table, and
// this stage must not invent a classification the document did not make. Only the
// no-connect fallback is allowed, because the document states that one in words.
func TestTypeColumnIsNotSecondGuessed(t *testing.T) {
	spec, _ := runPinFixture(t)
	byID := map[string]*parampb.Pin{}
	for _, p := range spec.Pins {
		byID[p.Id] = p
	}
	if p := byID["vcca"]; p == nil || p.Function != parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED {
		t.Errorf("vcca function = %v, want UNSPECIFIED: the I/O column reads \"-\" for supplies", p.GetFunction())
	}
	if p := byID["a1"]; p == nil || p.Function != parampb.PinFunction_PIN_FUNCTION_BIDIRECTIONAL {
		t.Errorf("a1 function = %v, want BIDIRECTIONAL: its I/O column says so", p.GetFunction())
	}
}

// Under any axis but PACKAGE the designator columns are recorded and NOT read, so a
// variant-column table cannot mint packages named after part numbers.
func TestVariantAxisMintsNoPackages(t *testing.T) {
	fx := loadDocFixture(t, "txb0104-pintable-docir.textproto")
	recipes := []*derivepb.Recipe{{
		Name: "variant-axis", DocTitlePattern: "(?i)txb0104",
		PinTables: []*derivepb.PinTableRule{{
			TitlePattern: "(?i)pin functions",
			ColumnAxis:   derivepb.PinColumnAxis_PIN_COLUMN_AXIS_VARIANT,
		}},
	}}
	spec, manifest, err := Run(fx, recipes, nil, Identity{MPN: "TXB0104"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(spec.Packages) != 0 {
		t.Errorf("variant columns must not become packages, got %d", len(spec.Packages))
	}
	if len(spec.Pins) == 0 {
		t.Error("pins must still be extracted; only their numbering is unread")
	}
	for _, p := range spec.Pins {
		if len(p.Numbers) != 0 {
			t.Errorf("pin %q carries numbers under a non-package axis: %v", p.Id, p.Numbers)
		}
	}
	var gapped bool
	for _, g := range manifest.Gaps {
		if g.Kind == "pin-columns-uninterpreted" {
			gapped = true
		}
	}
	if !gapped {
		t.Error("columns were skipped with no gap; silence must never read as coverage")
	}
}

// Degrade-safety: a recipe with no pin_tables behaves exactly as before, and a document
// whose tables are all parameter tables emits no pins.
func TestRecipeWithoutPinTablesUnchanged(t *testing.T) {
	spec, manifest := runFixture(t)
	if len(spec.Pins) != 0 || len(spec.Packages) != 0 {
		t.Errorf("parameter-only run emitted %d pins and %d packages", len(spec.Pins), len(spec.Packages))
	}
	if manifest.PinsEmitted != 0 || manifest.PackagesEmitted != 0 {
		t.Errorf("counters non-zero on a parameter-only run: %d/%d", manifest.PinsEmitted, manifest.PackagesEmitted)
	}
}

// Splitting a designator cell on whitespace invents pins, because producers flatten a
// footnote marker into the cell as a trailing token. Only commas separate terminals.
func TestSplitDesignators(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"6, 9", []string{"6", "9"}},
		{"6,9", []string{"6", "9"}},
		{"8 2", []string{"8"}},     // pin 8 carrying footnote 2
		{"12 2", []string{"12"}},   // same, two-digit
		{"2, 3", []string{"2", "3"}},
		{"B2", []string{"B2"}},
		{"-", nil},
		{"—", nil},
		{"", nil},
	}
	for _, tc := range cases {
		got := splitDesignators(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("splitDesignators(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSplitPackageCodes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"D, PW", []string{"D", "PW"}},
		{"GXU, ZXU, NMN", []string{"GXU", "ZXU", "NMN"}},
		{"RUT", []string{"RUT"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := splitPackageCodes(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("splitPackageCodes(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// pinTableDoc builds a one-table document so a layout can be asserted without a fixture.
func pinTableDoc(rows int32, cols int32, cells ...*docpb.Cell) *docpb.Document {
	return &docpb.Document{
		ContentHash: "sha256:synthetic", SourceFormat: "pdf", Title: "SYNTH", Producer: "hand", PageCount: 1,
		Pages: []*docpb.Page{{
			Number: 1, Width: 612, Height: 792,
			TextBlocks: []*docpb.TextBlock{{
				Id: "p1.x1", Text: "Pin Functions", Kind: "section_header",
				Bbox: &docpb.BBox{X: 72, Y: 100, Width: 100, Height: 12},
			}},
			Tables: []*docpb.Table{{
				Id: "p1.t1", Rows: rows, Cols: cols, Cells: cells,
				Bbox: &docpb.BBox{X: 72, Y: 114, Width: 400, Height: 200}, Confidence: 1,
			}},
		}},
	}
}

func cell(row, col int32, text string) *docpb.Cell {
	return &docpb.Cell{Row: row, Col: col, Text: text}
}

func packageAxisRecipe() []*derivepb.Recipe {
	return []*derivepb.Recipe{{
		Name: "synth", DocTitlePattern: ".",
		PinTables: []*derivepb.PinTableRule{{
			TitlePattern: "(?i)pin functions",
			ColumnAxis:   derivepb.PinColumnAxis_PIN_COLUMN_AXIS_PACKAGE,
		}},
	}}
}

// A FLAT pin table ("PIN | NAME | FUNCTION") has one numbering column that names no
// package. Reading it as a package minted a body called "pin" and then collided every
// designator inside it, which failed validation on a real single-package part.
func TestFlatPinTableMintsNoPackage(t *testing.T) {
	d := pinTableDoc(3, 3,
		cell(0, 0, "PIN"), cell(0, 1, "NAME"), cell(0, 2, "FUNCTION"),
		cell(1, 0, "1"), cell(1, 1, "EN"), cell(1, 2, "Enable input."),
		cell(2, 0, "2"), cell(2, 1, "GND"), cell(2, 2, "Ground."),
	)
	spec, man, err := Run(d, packageAxisRecipe(), nil, Identity{MPN: "SYNTH"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, p := range spec.Packages {
		if p.Id == "pin" {
			t.Error(`minted a package called "pin" from the numbering column header`)
		}
	}
	if len(spec.Packages) != 0 {
		t.Errorf("a flat table names no package, got %d", len(spec.Packages))
	}
	if len(spec.Pins) != 2 {
		t.Fatalf("want both pins extracted, got %d", len(spec.Pins))
	}
	var gapped bool
	for _, g := range man.Gaps {
		if g.Kind == "pin-numbering-unattributed" {
			gapped = true
		}
	}
	if !gapped {
		t.Error("designators went unattributed with no gap; silence must never read as coverage")
	}
}

// Real per-package headers carry footnote markers and hyphenated codes. Dropping either
// loses packages silently, which is worse than failing.
func TestPackageCodesSurviveFootnotesAndHyphens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"LCCC (1)", []string{"LCCC"}},
		{"SOIC, SOT23-8, VSSOP, CDIP, PDIP, SO, TSSOP, CFP (1)",
			[]string{"SOIC", "SOT23-8", "VSSOP", "CDIP", "PDIP", "SO", "TSSOP", "CFP"}},
		{"D, PW", []string{"D", "PW"}},
		{"HTQFP", []string{"HTQFP"}},
	}
	for _, tc := range cases {
		got := splitPackageCodes(tc.in)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("splitPackageCodes(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// A pin table spanning several pages arrives as several tables that restate rows. Each
// restatement was emitted as a new pin claiming a designator the first already held,
// which fails param.Validate. Same name AND a shared designator means same terminal.
func TestContinuationTableMergesRestatedPins(t *testing.T) {
	d := pinTableDoc(3, 3,
		cell(0, 0, "PIN"), cell(1, 0, "NAME"), cell(1, 1, "HTQFP"), cell(1, 2, "DESCRIPTION"),
		cell(2, 0, "RX_ER"), cell(2, 1, "54"), cell(2, 2, "Receive error."),
	)
	d.Pages[0].Tables[0].Cells[0].ColSpan = 2
	// The continuation: same terminal restated, plus a body the first statement omitted.
	d.Pages[0].Tables = append(d.Pages[0].Tables, &docpb.Table{
		Id: "p1.t2", Rows: 3, Cols: 4, Confidence: 1,
		Bbox: &docpb.BBox{X: 72, Y: 320, Width: 400, Height: 100},
		Cells: []*docpb.Cell{
			{Row: 0, Col: 0, Text: "PIN", ColSpan: 3},
			cell(1, 0, "NAME"), cell(1, 1, "HTQFP"), cell(1, 2, "VQFN"), cell(1, 3, "DESCRIPTION"),
			cell(2, 0, "RX_ER"), cell(2, 1, "54"), cell(2, 2, "40"), cell(2, 3, "Receive error."),
		},
	})
	d.Pages[0].TextBlocks = append(d.Pages[0].TextBlocks, &docpb.TextBlock{
		Id: "p1.x2", Text: "Pin Functions", Kind: "section_header",
		Bbox: &docpb.BBox{X: 72, Y: 306, Width: 100, Height: 12},
	})

	spec, _, err := Run(d, packageAxisRecipe(), nil, Identity{MPN: "SYNTH"})
	if err != nil {
		t.Fatalf("Run rejected a restated pin: %v", err)
	}
	var rxer int
	for _, p := range spec.Pins {
		if p.Name == "RX_ER" {
			rxer++
		}
	}
	if rxer != 1 {
		t.Errorf("a restated terminal must merge, got %d pins named RX_ER", rxer)
	}
	got := numbersOf(spec, "rx_er")
	if got["htqfp"] != "54" || got["vqfn"] != "40" {
		t.Errorf("merge must keep the union of both statements, got %v", got)
	}
}

// The merge must not undo the no-connect split: nc6 and nc9 share a name and no leg.
func TestMergeDoesNotCollapseTheNoConnectSplit(t *testing.T) {
	spec, _ := runPinFixture(t)
	var ncs int
	for _, p := range spec.Pins {
		if p.Name == "NC" {
			ncs++
		}
	}
	if ncs != 2 {
		t.Errorf("NC must stay two terminals, got %d", ncs)
	}
}
