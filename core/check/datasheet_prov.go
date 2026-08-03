package check

import (
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// datasheetCitationOf builds the structured datasheet citation for one seeded parameter: it resolves
// the SourceDoc title from the parameter's doc_ref and copies the page, section, method, and
// confidence. It is the shared core of both the string citation() and the typed Finding.DatasheetProv.
func datasheetCitationOf(spec *parampb.PartSpec, p *parampb.Parameter) *DatasheetCitation {
	return &DatasheetCitation{
		Doc:        docTitle(spec, p.GetProv().GetDocRef()),
		DocRef:     p.GetProv().GetDocRef(),
		Page:       p.GetProv().GetPage(),
		Section:    p.GetProv().GetTableOrFigure(),
		Method:     p.GetProv().GetMethod(),
		Confidence: p.GetProv().GetConfidence(),
	}
}

// docTitle resolves a doc_ref to its SourceDoc title within a spec; "" when the id names no doc, which
// a caller renders as "unknown source".
func docTitle(spec *parampb.PartSpec, docRef string) string {
	for _, d := range spec.GetDocs() {
		if d.GetId() == docRef {
			return d.GetTitle()
		}
	}
	return ""
}

// DatasheetProvFor resolves the structured datasheet citation for a component's parameter by symbol,
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
			return datasheetCitationOf(spec, p)
		}
	}
	return nil
}
