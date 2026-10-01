package candidate

import (
	candpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/candidate"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	"github.com/panyam/agni/datasheet/docindex"
)

// RetrievalSource proposes candidates by SEARCHING the document rather than by reading values out of
// it. It is the simplest Source.
//
// It never proposes a Value, and that is not a gap to fill later. Retrieval knows which passage is
// about a symbol and nothing about what the passage says, so a number guessed from a ranked hit would
// be a confident wrong answer.
//
// An author still lands on the row instead of scrolling 58 pages, and every candidate is a real
// region quoted verbatim, so there is nothing invented for the fabrication check to catch.
type RetrievalSource struct {
	// MaxHits bounds what a person is asked to look at. Zero means 3, since twenty passages move the
	// search rather than do it.
	MaxHits int
	// Confidence is stamped on every candidate. Outside (0, 1) it falls back to 0.3, because "this
	// passage mentions your symbol" is weak evidence for a specific value.
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
