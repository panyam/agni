package check

import (
	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// DatasheetCitationOf builds the structured datasheet Citation for one seeded parameter. It resolves
// the SourceDoc title from the parameter's doc_ref, copies the page, section, method, and confidence,
// and derives the verification state against the revision that SourceDoc records. Both the string
// Citation() and the typed Finding.DatasheetProv are built on it.
//
// Verification is derived here rather than stored on the finding because it is a fact about the
// document as the corpus holds it right now. Computing it at citation time means a re-seed changes
// every subsequent answer without anything having to be re-stamped.
func DatasheetCitationOf(spec *parampb.PartSpec, p *parampb.Parameter) *DatasheetCitation {
	c := DatasheetCitationOfProv(spec, p.GetProv())
	c.Verification = string(param.VerificationOfIn(spec, p))
	c.VerifiedRevision = p.GetVerification().GetDocRevision()
	return c
}

// DatasheetCitationOfProv is the same build from a bare ParamProvenance, for the rows that carry one
// but are not parameters, such as a Pin's declaration and a PinRelation's bound. Like PinCitation on
// the string side, it keeps the doc_ref resolution in one place so the row types cannot drift.
func DatasheetCitationOfProv(spec *parampb.PartSpec, prov *parampb.ParamProvenance) *DatasheetCitation {
	return &DatasheetCitation{
		Doc:        DocTitle(spec, prov.GetDocRef()),
		DocRef:     prov.GetDocRef(),
		Page:       prov.GetPage(),
		Section:    prov.GetTableOrFigure(),
		Method:     prov.GetMethod(),
		Confidence: prov.GetConfidence(),
	}
}

// DocTitle resolves a doc_ref to its SourceDoc title within a spec; "" when the id names no doc, which
// a caller renders as "unknown source".
func DocTitle(spec *parampb.PartSpec, docRef string) string {
	for _, d := range spec.GetDocs() {
		if d.GetId() == docRef {
			return d.GetTitle()
		}
	}
	return ""
}

// DatasheetProvFor resolves the structured datasheet Citation for a component's parameter by symbol,
// so a datalog-authored rule (query.RuleFromQuery) can carry the same doc/page/section/method/
// confidence the built-in datasheet rules attach directly. refDes is the finding's component subject.
// Nil when the component has no seeded spec, or the spec has no parameter with that symbol.
func DatasheetProvFor(m Model, refDes, symbol string) *DatasheetCitation {
	spec := m.PartSpec(refDes)
	if spec == nil {
		return nil
	}
	for _, p := range spec.GetParameters() {
		if p.GetSymbol() == symbol {
			return DatasheetCitationOf(spec, p)
		}
	}
	return nil
}
