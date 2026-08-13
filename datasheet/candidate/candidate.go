// Package candidate is the seam between "something proposed a fact" and "a person accepted it".
//
// It exists because the path from a blocked check to a seeded value is otherwise entirely manual: a
// check reports which fact it wanted (the unmet dependency), and someone reads 58 pages. A proposer
// can shorten that, but only if what it proposes can be checked faster than it could be found.
//
// # A candidate without a resolvable citation is refused, not down-weighted
//
// This is the rule the whole package exists to enforce, and it is a measured decision rather than a
// cautious one. A pattern sweep over a real corpus ran 71% precision on the easy form and 17% on the
// hard one. At those rates an author who cannot check an answer instantly will wave it through, and a
// wrong fact wearing a verified badge is strictly worse than no fact at all: a blocked check is
// honest, a wrong seeded value is a confident wrong answer that a design review then rests on.
//
// So Validate does not score a citation, it accepts or rejects one. "I could not cite this precisely"
// is a legitimate answer from a proposer; "here is a value, roughly from page 12" is not.
//
// # Fabrication is structurally detectable
//
// A candidate quotes the document verbatim, and Validate checks that the quote actually occurs in the
// region it claims. A proposer that invents a plausible sentence fails that check without anyone
// reading the page. This is what makes an unreliable proposer safe to put in front of a person: it
// can waste their time, but it cannot manufacture evidence.
//
// # The manual path does not depend on any of this
//
// Source is an interface with no privileged implementation. A deployment with no proposer configured
// still authors facts by hand exactly as before, which is the posture param.ParamProvider already
// takes for the datasheet tier: pluggable, absent-tolerant, never required.
package candidate

import (
	"errors"
	"fmt"
	"strings"

	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// Request is the fact being asked for. It is deliberately the same shape as a check's unmet
// dependency, so the thing that reported the gap and the thing that proposes a fix speak about it in
// the same terms rather than through a translation nobody owns.
type Request struct {
	MPN    string
	Symbol string
}

// Citation locates a candidate in the document precisely enough to highlight, and quotes it.
//
// RegionID is a doc-IR text block or table id. Row and Col locate a table cell and are -1 otherwise.
// Quote is the region's text VERBATIM, which is what makes the claim checkable rather than merely
// plausible.
type Citation struct {
	Page     int32
	RegionID string
	Row, Col int32
	Quote    string
}

// Candidate is a proposed fact, its evidence, and who proposed it. Value may be nil: a proposer that
// found the right passage and could not read a number from it has still done most of the work, and
// saying so is more useful than guessing a value.
type Candidate struct {
	Request   Request
	Citation  Citation
	Value     *parampb.RangeValue
	Unit      string
	LimitKind parampb.LimitKind
	// Source names what proposed this, and becomes the provenance method on acceptance, so a seeded
	// corpus records which mechanism produced each value rather than flattening them all to "machine".
	Source string
	// Confidence is the proposer's own estimate, in (0, 1]. It NEVER reaches 1.0 through this path:
	// only a human verification earns that, which is the posture derive already takes.
	Confidence float64
}

// Source proposes candidates for a request against one document. Returning none is a legitimate and
// common answer, and is what a proposer must do rather than lower its standards to produce something.
type Source interface {
	Propose(req Request, d *docpb.Document) ([]Candidate, error)
}

// Refusal reasons, distinguished so a caller can tell a proposer that is misconfigured from one that
// is behaving correctly and simply found nothing worth standing behind.
var (
	// ErrNoCitation: the candidate names no region, so nobody can check it.
	ErrNoCitation = errors.New("candidate has no citation")
	// ErrRegionUnknown: the cited region does not exist in this document. A proposer reading a
	// different revision produces exactly this, which is why it is a refusal and not a warning.
	ErrRegionUnknown = errors.New("cited region does not exist in this document")
	// ErrQuoteNotFound: the quote does not occur in the region it cites. This is the fabrication
	// check: an invented sentence fails it without anyone opening the page.
	ErrQuoteNotFound = errors.New("quoted text does not occur in the cited region")
	// ErrConfidenceRange: confidence outside (0, 1], or 1.0 claimed by a machine proposer. A value
	// nobody stands behind must not be emitted, and only a human verification earns certainty.
	ErrConfidenceRange = errors.New("confidence outside (0, 1], or 1.0 claimed without human verification")
)

// Validate accepts or rejects a candidate against the document it cites. There is no middle verdict
// on purpose: see the package doc.
//
// The quote check normalises whitespace before comparing, because a producer's line breaking is not
// part of the claim, but it does not normalise anything else. A proposer that paraphrases fails, and
// should.
func Validate(c Candidate, d *docpb.Document) error {
	if c.Citation.RegionID == "" || strings.TrimSpace(c.Citation.Quote) == "" {
		return fmt.Errorf("%w: region %q quote %q", ErrNoCitation, c.Citation.RegionID, c.Citation.Quote)
	}
	if c.Confidence <= 0 || c.Confidence >= 1 {
		return fmt.Errorf("%w: %v", ErrConfidenceRange, c.Confidence)
	}
	text, ok := regionText(d, c.Citation)
	if !ok {
		return fmt.Errorf("%w: page %d region %q", ErrRegionUnknown, c.Citation.Page, c.Citation.RegionID)
	}
	if !strings.Contains(squash(text), squash(c.Citation.Quote)) {
		return fmt.Errorf("%w: region %q", ErrQuoteNotFound, c.Citation.RegionID)
	}
	return nil
}

// regionText returns the text of the cited region: a text block, one cell of a table, or the whole
// table when no cell is named. A citation naming a cell that does not exist is unknown rather than
// empty, so a stale row/column reference is caught rather than silently matching nothing.
func regionText(d *docpb.Document, cit Citation) (string, bool) {
	for _, pg := range d.GetPages() {
		if pg.GetNumber() != cit.Page {
			continue
		}
		for _, tb := range pg.GetTextBlocks() {
			if tb.GetId() == cit.RegionID {
				return tb.GetText(), true
			}
		}
		for _, t := range pg.GetTables() {
			if t.GetId() != cit.RegionID {
				continue
			}
			if cit.Row < 0 && cit.Col < 0 {
				var all []string
				for _, c := range t.GetCells() {
					all = append(all, c.GetText())
				}
				return strings.Join(all, " "), true
			}
			for _, c := range t.GetCells() {
				if c.GetRow() == cit.Row && c.GetCol() == cit.Col {
					return c.GetText(), true
				}
			}
			return "", false
		}
	}
	return "", false
}

func squash(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// Accept turns a validated candidate into the parameter a corpus holds, stamping provenance that
// points back at the exact region it came from.
//
// It re-validates rather than trusting the caller to have done it: this is the one door between a
// proposal and a corpus, and a check that can be skipped by forgetting a call is not a check.
//
// The result is deliberately NOT verified. It carries the proposer's confidence and method, and a
// human confirmation is a separate act on a separate artifact; conflating "a machine proposed this
// and a person has not looked" with "a person checked this" is the failure this seam exists to
// prevent.
func Accept(c Candidate, d *docpb.Document, docRef string) (*parampb.Parameter, error) {
	if err := Validate(c, d); err != nil {
		return nil, err
	}
	region := c.Citation.RegionID
	if c.Citation.Row >= 0 {
		region = fmt.Sprintf("%s r%dc%d", region, c.Citation.Row, c.Citation.Col)
	}
	return &parampb.Parameter{
		Symbol:    c.Request.Symbol,
		LimitKind: c.LimitKind,
		Value:     c.Value,
		Unit:      c.Unit,
		Attributes: map[string]string{
			// The verbatim evidence travels with the fact, so a later reviewer re-checks the claim
			// without re-opening the document.
			"quote": c.Citation.Quote,
		},
		Prov: &parampb.ParamProvenance{
			DocRef: docRef, Page: c.Citation.Page, TableOrFigure: region,
			Method: c.Source, Confidence: c.Confidence,
		},
	}, nil
}
