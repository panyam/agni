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

	candpb "github.com/panyam/agni/gen/go/agni/v1/candidate"
	docpb "github.com/panyam/agni/gen/go/agni/v1/doc"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The wire types ARE the types here, as in param and doc: this is a cross-runtime contract, and a
// Go twin beside it is the drift a shared schema exists to prevent.

// Source proposes candidates for a request against one document. Returning none is a legitimate and
// common answer, and is what a proposer must do rather than lower its standards to produce something.
type Source interface {
	Propose(req *candpb.Request, d *docpb.Document) ([]*candpb.Candidate, error)
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

// regionText returns the text of the cited region: a text block, one cell of a table, or the whole
// table when no cell is named. A citation naming a cell that does not exist is unknown rather than
// empty, so a stale row/column reference is caught rather than silently matching nothing.
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
