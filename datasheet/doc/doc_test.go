package doc

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/datasheet/param"
)

func cloneTable(t *docpb.Table) *docpb.Table {
	return proto.Clone(t).(*docpb.Table)
}

func readFixture(t *testing.T, name string) *docpb.Document {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	d, err := Load(f)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return d
}

func TestFixtureValidates(t *testing.T) {
	d := readFixture(t, "bss138-docir.textproto")
	if err := Validate(d); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestQueryHelpers(t *testing.T) {
	d := readFixture(t, "bss138-docir.textproto")

	abs := TablesMatching(d, regexp.MustCompile(`Absolute Maximum`))
	if len(abs) != 1 || abs[0].Id != "p1.t1" {
		t.Fatalf("TablesMatching(Absolute Maximum): want [p1.t1], got %v", abs)
	}

	if tbl := TableByID(d, "p2.t1"); tbl == nil || tbl.Title != "Electrical Characteristics" {
		t.Fatalf("TableByID(p2.t1) = %v", tbl)
	}
	if fig := FigureByID(d, "p3.f1"); fig == nil || !strings.Contains(fig.Caption, "On-Resistance Variation") {
		t.Fatalf("FigureByID(p3.f1) = %v", fig)
	}

	ec := TableByID(d, "p2.t1")
	if got := CellText(ec, 1, 0); got != "VGS(th)" {
		t.Errorf("CellText(1,0) = %q, want VGS(th)", got)
	}
	// RDS(on) is a merged cell spanning its three condition rows; it lives at its
	// top-left position and CellAt returns nil elsewhere in the span.
	rds := CellAt(ec, 2, 0)
	if rds == nil || rds.Text != "RDS(on)" || rds.RowSpan != 3 {
		t.Fatalf("CellAt(2,0) = %+v, want RDS(on) with row_span 3", rds)
	}
	if CellAt(ec, 3, 0) != nil {
		t.Errorf("CellAt(3,0) inside the merged span must be nil (cells appear once, at top-left)")
	}

	if txt := PageText(d, 1); !strings.Contains(txt, "Absolute Maximum Ratings") {
		t.Errorf("PageText(1) should contain the page-1 heading, got %q", txt)
	}
}

// The ticket's acceptance check: every provenance entry in the WS10-001 param
// fixture (page + table label) must resolve to a region in this document's doc-IR.
func TestParamProvenanceResolves(t *testing.T) {
	d := readFixture(t, "bss138-docir.textproto")
	pf, err := os.Open(filepath.Join("..", "param", "testdata", "bss138.textproto"))
	if err != nil {
		t.Fatalf("open param fixture: %v", err)
	}
	defer pf.Close()
	spec, err := param.Load(pf)
	if err != nil {
		t.Fatalf("load param fixture: %v", err)
	}
	for _, p := range spec.Parameters {
		if tbl := FindTableForProv(d, p.Prov.Page, p.Prov.TableOrFigure); tbl == nil {
			t.Errorf("prov (page %d, %q) of %s does not resolve to a doc-IR table",
				p.Prov.Page, p.Prov.TableOrFigure, p.Symbol)
		}
	}
}

// A recipe fragment (title pattern -> LimitKind) evaluated over the doc-IR: the
// consumer surface WS10-002's recipes will use.
func TestRecipeFragmentClassifiesTables(t *testing.T) {
	d := readFixture(t, "bss138-docir.textproto")
	recipe := []struct {
		pattern *regexp.Regexp
		kind    parampb.LimitKind
	}{
		{regexp.MustCompile(`(?i)absolute maximum`), parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX},
		{regexp.MustCompile(`(?i)electrical characteristics`), parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC},
	}
	got := map[string]parampb.LimitKind{}
	for _, page := range d.Pages {
		for _, tbl := range page.Tables {
			for _, r := range recipe {
				if r.pattern.MatchString(tbl.Title) {
					got[tbl.Id] = r.kind
				}
			}
		}
	}
	want := map[string]parampb.LimitKind{
		"p1.t1": parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX,
		"p2.t1": parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
	}
	for id, k := range want {
		if got[id] != k {
			t.Errorf("table %s classified as %v, want %v", id, got[id], k)
		}
	}
}

func TestTableHashIgnoresLayoutButNotContent(t *testing.T) {
	d := readFixture(t, "bss138-docir.textproto")
	tbl := TableByID(d, "p1.t1")
	if TableHash(tbl) != tbl.ContentHash {
		t.Fatalf("fixture content_hash out of date: want %s", TableHash(tbl))
	}
	// bbox and detection confidence are derivation artifacts, not content.
	moved := cloneTable(tbl)
	moved.Bbox = &docpb.BBox{X: 1, Y: 2, Width: 3, Height: 4}
	moved.Confidence = 0.5
	if TableHash(moved) != TableHash(tbl) {
		t.Errorf("moving a table or changing confidence must not change its content hash")
	}
	edited := cloneTable(tbl)
	for _, c := range edited.Cells {
		if c.Text == "50" {
			c.Text = "60"
		}
	}
	if TableHash(edited) == TableHash(tbl) {
		t.Errorf("changing a cell value must change the content hash")
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*docpb.Document)
		want string
	}{
		{"missing content hash", func(d *docpb.Document) { d.ContentHash = "" }, "content_hash"},
		{"duplicate region id", func(d *docpb.Document) { d.Pages[0].Tables[0].Id = "p2.t1" }, "duplicate"},
		{"page number out of range", func(d *docpb.Document) { d.Pages[0].Number = 9 }, "page"},
		{"cell outside grid", func(d *docpb.Document) {
			t0 := d.Pages[0].Tables[0]
			t0.Cells[0].Col = t0.Cols + 1
		}, "grid"},
		{"zero confidence", func(d *docpb.Document) { d.Pages[0].Tables[0].Confidence = 0 }, "confidence"},
		{"stale table hash", func(d *docpb.Document) { d.Pages[0].Tables[0].Cells[1].Text = "edited" }, "content_hash"},
	}
	for _, tc := range cases {
		d := readFixture(t, "bss138-docir.textproto")
		tc.mut(d)
		err := Validate(d)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want error mentioning %q", tc.name, err, tc.want)
		}
	}
}
