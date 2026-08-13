package candidate

import (
	"errors"
	"testing"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func testDoc() *docpb.Document {
	return &docpb.Document{
		Title: "ACME-1", PageCount: 1,
		Pages: []*docpb.Page{{Number: 1,
			TextBlocks: []*docpb.TextBlock{{Id: "p1.x1", Text: "Recommended Operating Conditions apply over VCC."}},
			Tables: []*docpb.Table{{Id: "p1.t1", Rows: 2, Cols: 2, Cells: []*docpb.Cell{
				{Row: 0, Col: 0, Text: "SYMBOL"}, {Row: 0, Col: 1, Text: "MAX"},
				{Row: 1, Col: 0, Text: "VCC"}, {Row: 1, Col: 1, Text: "3.6"},
			}}},
		}},
	}
}

func good() Candidate {
	return Candidate{
		Request:  Request{MPN: "ACME-1", Symbol: "VCC"},
		Citation: Citation{Page: 1, RegionID: "p1.t1", Row: 1, Col: 1, Quote: "3.6"},
		Value:    &parampb.RangeValue{Max: f(3.6)}, Unit: "V",
		LimitKind: parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING,
		Source:    "test/v0", Confidence: 0.8,
	}
}

func f(v float64) *float64 { return &v }

func TestValidateAcceptsACitedCandidate(t *testing.T) {
	if err := Validate(good(), testDoc()); err != nil {
		t.Fatalf("a candidate quoting the cell it cites must validate: %v", err)
	}
}

// The fabrication check. A proposer that invents a plausible sentence fails without anyone opening
// the document, which is what makes an unreliable proposer safe to put in front of a person.
func TestValidateRejectsAnInventedQuote(t *testing.T) {
	c := good()
	c.Citation.Quote = "4.2"
	if err := Validate(c, testDoc()); !errors.Is(err, ErrQuoteNotFound) {
		t.Errorf("want ErrQuoteNotFound, got %v", err)
	}
}

func TestValidateRejectsUncheckableCandidates(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Candidate)
		want error
	}{
		{"no region", func(c *Candidate) { c.Citation.RegionID = "" }, ErrNoCitation},
		{"no quote", func(c *Candidate) { c.Citation.Quote = "  " }, ErrNoCitation},
		{"region not in this document", func(c *Candidate) { c.Citation.RegionID = "p9.t9" }, ErrRegionUnknown},
		{"cell that does not exist", func(c *Candidate) { c.Citation.Row = 7 }, ErrRegionUnknown},
		{"page that does not exist", func(c *Candidate) { c.Citation.Page = 4 }, ErrRegionUnknown},
		{"no confidence", func(c *Candidate) { c.Confidence = 0 }, ErrConfidenceRange},
		{"certainty claimed by a machine", func(c *Candidate) { c.Confidence = 1 }, ErrConfidenceRange},
	}
	for _, tc := range cases {
		c := good()
		tc.mut(&c)
		if err := Validate(c, testDoc()); !errors.Is(err, tc.want) {
			t.Errorf("%s: want %v, got %v", tc.name, tc.want, err)
		}
	}
}

// Line breaking is the producer's, not the claim's, so whitespace is normalised. Nothing else is:
// a paraphrase is not a quote.
func TestQuoteMatchingIgnoresOnlyWhitespace(t *testing.T) {
	c := good()
	c.Citation.RegionID, c.Citation.Row, c.Citation.Col = "p1.x1", -1, -1
	c.Citation.Quote = "Recommended   Operating\nConditions"
	if err := Validate(c, testDoc()); err != nil {
		t.Errorf("re-wrapped quote must still match: %v", err)
	}
	c.Citation.Quote = "Recommended Operating Settings"
	if err := Validate(c, testDoc()); !errors.Is(err, ErrQuoteNotFound) {
		t.Errorf("a paraphrase must not pass as a quote, got %v", err)
	}
}

// Accept is the one door between a proposal and a corpus, so it re-validates rather than trusting
// the caller to have done it.
func TestAcceptRevalidatesAndStampsProvenance(t *testing.T) {
	bad := good()
	bad.Citation.Quote = "invented"
	if _, err := Accept(bad, testDoc(), "acme1"); err == nil {
		t.Error("Accept must not admit a candidate that Validate would reject")
	}

	p, err := Accept(good(), testDoc(), "acme1")
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if p.Symbol != "VCC" || p.GetValue().GetMax() != 3.6 {
		t.Errorf("value lost in translation: %+v", p)
	}
	if p.Prov.GetDocRef() != "acme1" || p.Prov.GetPage() != 1 {
		t.Errorf("provenance must point back at the region: %+v", p.Prov)
	}
	if p.Prov.GetMethod() != "test/v0" {
		t.Errorf("method must record WHICH mechanism proposed it, got %q", p.Prov.GetMethod())
	}
	if p.Prov.GetConfidence() >= 1 {
		t.Error("an accepted proposal is not a verified fact; only a human earns 1.0")
	}
	if p.Attributes["quote"] == "" {
		t.Error("the evidence must travel with the fact so a reviewer re-checks without the document")
	}
}

// Retrieval knows which passage is about a symbol and nothing about what it says. Proposing a value
// would be the confident wrong answer this package exists to keep out.
func TestRetrievalSourceProposesRegionsNeverValues(t *testing.T) {
	got, err := RetrievalSource{}.Propose(Request{MPN: "ACME-1", Symbol: "VCC"}, testDoc())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("VCC appears in this document; retrieval should find it")
	}
	for _, c := range got {
		if c.Value != nil {
			t.Errorf("retrieval must not invent a value: %+v", c.Value)
		}
		if err := Validate(c, testDoc()); err != nil {
			t.Errorf("a retrieved candidate quotes a real region and must validate: %v", err)
		}
	}
}

// A document that does not discuss the symbol yields nothing, which is the correct answer and not a
// reason to lower the bar.
func TestRetrievalSourceReturnsNothingRatherThanReaching(t *testing.T) {
	got, err := RetrievalSource{}.Propose(Request{Symbol: "thermalimpedance"}, testDoc())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("want no candidates, got %+v", got)
	}
}
