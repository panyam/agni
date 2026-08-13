package check

import (
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// DatasheetCitationOf builds the structured datasheet Citation for one seeded parameter: it resolves
// the SourceDoc title from the parameter's doc_ref and copies the page, section, method, and
// confidence. It is the shared core of both the string Citation() and the typed Finding.DatasheetProv.
func DatasheetCitationOf(spec *parampb.PartSpec, p *parampb.Parameter) *DatasheetCitation {
	return DatasheetCitationOfProv(spec, p.GetProv())
}

// DatasheetCitationOfProv is the same build from a bare ParamProvenance, for the rows that carry one
// but are not parameters: a Pin's declaration and a PinRelation's bound. It exists for the reason
// PinCitation does on the string side — provenance is provenance, and duplicating the doc_ref
// resolution per row type is how the two drift.
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
