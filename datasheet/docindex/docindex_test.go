package docindex

import (
	"strings"
	"testing"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
)

func doc() *docpb.Document {
	return &docpb.Document{
		Title: "TXB0104", PageCount: 2,
		Pages: []*docpb.Page{
			{Number: 1, TextBlocks: []*docpb.TextBlock{
				{Id: "p1.x1", Text: "Recommended Operating Conditions"},
				{Id: "p1.x2", Text: "The device operates over a wide supply voltage range."},
			}, Tables: []*docpb.Table{{
				Id: "p1.t1", Rows: 3, Cols: 3,
				Cells: []*docpb.Cell{
					{Row: 0, Col: 0, Text: "SYMBOL"}, {Row: 0, Col: 1, Text: "MIN"}, {Row: 0, Col: 2, Text: "MAX"},
					// The subscript arrives flattened, exactly as a real producer emits it.
					{Row: 1, Col: 0, Text: "V CCA"}, {Row: 1, Col: 1, Text: "1.2"}, {Row: 1, Col: 2, Text: "3.6"},
					{Row: 2, Col: 0, Text: "V CCB"}, {Row: 2, Col: 1, Text: "1.65"}, {Row: 2, Col: 2, Text: "5.5"},
				},
			}}},
			{Number: 2, TextBlocks: []*docpb.TextBlock{
				{Id: "p2.x1", Text: "Absolute Maximum Ratings over supply voltage"},
			}},
		},
	}
}

// Every hit has to resolve to something a viewer can highlight, or verification is a hunt and a
// reviewer waves through whatever they are shown.
func TestHitsLocatePrecisely(t *testing.T) {
	for _, h := range Build(doc()).Search("supply voltage", 10) {
		if h.RegionID == "" {
			t.Errorf("hit with no region cannot be highlighted: %+v", h)
		}
		if h.Page == 0 {
			t.Errorf("hit with no page: %+v", h)
		}
		if h.Text == "" {
			t.Errorf("hit with no text cannot be quoted: %+v", h)
		}
	}
}

// The producer flattens subscripts with an injected space, so the symbol as PRINTED is not the
// symbol as STORED. Searching what a person reads on the page has to find it.
func TestFindsAFlattenedSubscript(t *testing.T) {
	got := Build(doc()).Search("VCCA", 5)
	if len(got) == 0 {
		t.Fatal(`searching "VCCA" found nothing; the doc-IR spells it "V CCA"`)
	}
	if got[0].RegionID != "p1.t1" || got[0].Row != 1 {
		t.Errorf("want the VCCA row of the table, got %+v", got[0])
	}
}

// A value cell is meaningless alone: "3.6" is not a fact. The row label and column header are what
// make it one, so they are indexed with the cell and returned beside it.
func TestCellCarriesItsRowAndColumnContext(t *testing.T) {
	got := Build(doc()).Search("VCCA max", 5)
	if len(got) == 0 {
		t.Fatal("no hit")
	}
	var found bool
	for _, h := range got[:min(3, len(got))] {
		if h.Text == "3.6" {
			found = true
			if !strings.Contains(h.Context, "MAX") {
				t.Errorf("the cell must carry the header that makes it a fact, got %q", h.Context)
			}
		}
	}
	if !found {
		t.Errorf(`"VCCA max" should reach the value cell; got %+v`, got[:min(3, len(got))])
	}
}

// A term on every page discriminates nothing; a rare one is what makes a hit specific.
func TestRareTermsOutrankCommonOnes(t *testing.T) {
	got := Build(doc()).Search("absolute maximum", 5)
	if len(got) == 0 || got[0].RegionID != "p2.x1" {
		t.Errorf("want the absolute-maximum heading first, got %+v", got)
	}
}

func TestEmptyAndUnmatchedQueries(t *testing.T) {
	ix := Build(doc())
	if got := ix.Search("", 5); got != nil {
		t.Errorf("empty query must match nothing, got %d", len(got))
	}
	if got := ix.Search("thermal impedance junction", 5); len(got) != 0 {
		t.Errorf("a query the document does not discuss must match nothing, got %+v", got)
	}
	if got := Build(&docpb.Document{}).Search("anything", 5); got != nil {
		t.Errorf("an empty document indexes to nothing, got %d", len(got))
	}
}

// The index is derived: rebuilding from the same doc-IR gives the same answers, so a stale index is
// a performance problem and never a correctness one.
func TestRebuildIsDeterministic(t *testing.T) {
	a, b := Build(doc()).Search("supply voltage", 10), Build(doc()).Search("supply voltage", 10)
	if len(a) != len(b) {
		t.Fatalf("different result counts: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].RegionID != b[i].RegionID || a[i].Row != b[i].Row || a[i].Score != b[i].Score {
			t.Errorf("rebuild differs at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}
