// Package candidate sits between "something proposed a fact" and "a person accepted it" (agni
// issue 261). A blocked check names the fact it wanted, and a proposer can shorten the search for it,
// but only if what it proposes can be checked faster than it could be found.
//
// # A candidate without a resolvable citation is refused, not down-weighted
//
// A pattern sweep over a real corpus ran 71% precision on the easy form and 17% on the hard one. At
// those rates an author who cannot check an answer instantly will wave it through. A wrong seeded
// value is a confident wrong answer, and a design review then relies on it. A blocked check is at
// least visibly blocked. So Validate accepts or rejects a citation and never scores one. "I could
// not cite this precisely" is a legitimate answer from a proposer, and "a value, roughly from page
// 12" is not.
//
// # Fabrication is detectable
//
// A candidate quotes the document verbatim, and Validate checks that the quote occurs in the region
// it claims, so an invented sentence fails without anyone reading the page. An unreliable proposer
// can waste a person's time but cannot manufacture evidence.
//
// # The manual path does not depend on any of this
//
// Source is an interface with no privileged implementation, and a deployment with no proposer still
// authors facts by hand. param.ParamProvider works the same way. Both are pluggable, tolerate being
// absent, and are never required.
package candidate

import (
	"errors"
	"fmt"
	"strings"

	candpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/candidate"
	docpb "github.com/panyam/agni/datasheet/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The wire types ARE the types here, as in param and doc, because this is a cross-runtime contract
// and a Go twin would drift from it.

// Source proposes candidates for a request against one document. Returning none is a common answer.
// A proposer must return none rather than lower its standards to produce something.
type Source interface {
	Propose(req *candpb.Request, d *docpb.Document) ([]*candpb.Candidate, error)
}

// Refusal reasons, distinguished so a caller can tell a proposer that is misconfigured from one that
// is behaving correctly and simply found nothing worth standing behind.
var (
	// ErrNoCitation means the candidate names no region, so nobody can check it.
	ErrNoCitation = errors.New("candidate has no citation")
	// ErrRegionUnknown means the cited region does not exist in this document. A proposer reading a
	// different revision produces this, so it is a refusal and not a warning.
	ErrRegionUnknown = errors.New("cited region does not exist in this document")
	// ErrQuoteNotFound means the quote does not occur in the region it cites. This is the
	// fabrication check.
	ErrQuoteNotFound = errors.New("quoted text does not occur in the cited region")
	// ErrConfidenceRange means the confidence is outside (0, 1], or a machine proposer claimed 1.0.
	// A proposer must not emit a value that nobody stands behind. Only a human verification can
	// claim certainty.
	ErrConfidenceRange = errors.New("confidence outside (0, 1], or 1.0 claimed without human verification")
)

// Validate accepts or rejects a candidate against the document it cites, with no middle verdict (see
// the package doc).
//
// The quote check normalises whitespace and case before comparing, and nothing else. A producer's
// line breaking is not part of the claim. A proposer that paraphrases fails.
func Validate(c *candpb.Candidate, d *docpb.Document) error {
	if c.GetCitation().GetRegionId() == "" || strings.TrimSpace(c.GetCitation().GetQuote()) == "" {
		return fmt.Errorf("%w: region %q quote %q", ErrNoCitation, c.GetCitation().GetRegionId(), c.GetCitation().GetQuote())
	}
	if c.GetConfidence() <= 0 || c.GetConfidence() >= 1 {
		return fmt.Errorf("%w: %v", ErrConfidenceRange, c.GetConfidence())
	}
	text, ok := regionText(d, c.GetCitation())
	if !ok {
		return fmt.Errorf("%w: page %d region %q", ErrRegionUnknown, c.GetCitation().GetPage(), c.GetCitation().GetRegionId())
	}
	if !strings.Contains(squash(text), squash(c.GetCitation().GetQuote())) {
		return fmt.Errorf("%w: region %q", ErrQuoteNotFound, c.GetCitation().GetRegionId())
	}
	return nil
}

// regionText returns the text of the cited region. The region is a text block, one cell of a table,
// or the whole table when no cell is named. A citation naming a cell that does not exist is unknown
// rather than empty, so a stale row/column reference is caught rather than silently matching
// nothing.
func regionText(d *docpb.Document, cit *candpb.Citation) (string, bool) {
	for _, pg := range d.GetPages() {
		if pg.GetNumber() != cit.GetPage() {
			continue
		}
		for _, tb := range pg.GetTextBlocks() {
			if tb.GetId() == cit.GetRegionId() {
				return tb.GetText(), true
			}
		}
		for _, t := range pg.GetTables() {
			if t.GetId() != cit.GetRegionId() {
				continue
			}
			if cit.GetRow() < 0 && cit.GetCol() < 0 {
				var all []string
				for _, c := range t.GetCells() {
					all = append(all, c.GetText())
				}
				return strings.Join(all, " "), true
			}
			for _, c := range t.GetCells() {
				if c.GetRow() == cit.GetRow() && c.GetCol() == cit.GetCol() {
					return c.GetText(), true
				}
			}
			return "", false
		}
	}
	return "", false
}

func squash(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// Accept turns a validated candidate into a corpus parameter. It stamps provenance that points back
// at the exact region the candidate came from.
//
// It re-validates rather than trusting the caller, because this is the one door between a proposal
// and a corpus.
//
// The result is NOT verified. It records the proposer's confidence and method. A human confirmation
// is a separate act on a separate artifact, so "a machine proposed this" never reads as "a person
// checked this".
func Accept(c *candpb.Candidate, d *docpb.Document, docRef string) (*parampb.Parameter, error) {
	if err := Validate(c, d); err != nil {
		return nil, err
	}
	region := c.GetCitation().GetRegionId()
	if c.GetCitation().GetRow() >= 0 {
		region = fmt.Sprintf("%s r%dc%d", region, c.GetCitation().GetRow(), c.GetCitation().GetCol())
	}
	return &parampb.Parameter{
		Symbol:    c.GetRequest().GetSymbol(),
		LimitKind: c.GetLimitKind(),
		Value:     c.GetValue(),
		Unit:      c.GetUnit(),
		Attributes: map[string]string{
			// The verbatim evidence travels with the fact, so a later reviewer re-checks the claim
			// without re-opening the document.
			"quote": c.GetCitation().GetQuote(),
		},
		Prov: &parampb.ParamProvenance{
			DocRef: docRef, Page: c.GetCitation().GetPage(), TableOrFigure: region,
			Method: c.GetSource(), Confidence: c.GetConfidence(),
		},
	}, nil
}
