package candidate

import (
	candpb "github.com/panyam/agni/gen/go/agni/v1/candidate"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	"github.com/panyam/agni/datasheet/docindex"
)

// RetrievalSource proposes candidates by SEARCHING the document rather than by reading values out of
// it. It is the first Source and deliberately the dumbest one.
//
// It never proposes a Value. That is not a limitation to be fixed later, it is what the mechanism can
// honestly claim: retrieval knows which passage is about a symbol and nothing whatever about what the
// passage says. Guessing a number from a ranked hit would be exactly the confident-wrong-answer this
// package exists to keep out of a corpus.
//
// What it does deliver is most of the work: an author lands on the row instead of scrolling 58 pages,
// and every candidate is a real region of the real document, so the fabrication check cannot fail
// because there is nothing invented to fail it.
type RetrievalSource struct {
	// MaxHits bounds what a person is asked to look at. Zero means 3: a proposer that returns twenty
	// passages has moved the search rather than done it.
	MaxHits int
	// Confidence is stamped on every candidate. Modest by construction, and never 1.0, because
	// "this passage mentions your symbol" is weak evidence for a specific value.
	Confidence float64
}

// Propose searches for the requested symbol and offers the best-matching regions, quoted verbatim.
// Returning nothing is a normal outcome for a document that does not discuss the symbol.
func (s RetrievalSource) Propose(req *candpb.Request, d *docpb.Document) ([]*candpb.Candidate, error) {
	max := s.MaxHits
	if max <= 0 {
		max = 3
	}
	conf := s.Confidence
	if conf <= 0 || conf >= 1 {
		conf = 0.3
	}
	var out []*candpb.Candidate
	for _, h := range docindex.Build(d).Search(req.GetSymbol(), max) {
		out = append(out, &candpb.Candidate{
			Request: req,
			Citation: &candpb.Citation{
				Page: h.Page, RegionId: h.RegionID, Row: h.Row, Col: h.Col, Quote: h.Text,
			},
			Source: "retrieval/v0", Confidence: conf,
		})
	}
	return out, nil
}
