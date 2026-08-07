package review

import (
	"encoding/json"

	"github.com/panyam/agni/core/check"
)

// jsonReport is the stable, tooling-facing projection of a Report. It carries the FULL finding list per
// item (unlike RenderMarkdown, which caps the Detail cell) so a downstream renderer — a customer's
// folder-per-design report, the future web report — can show every finding and, eventually, deep-link
// each one to the viewer highlighted on its subject net/component. It is a hand-authored DTO rather than
// a marshal of the internal structs so the wire shape stays decoupled from check.Finding's proto
// provenance and can evolve independently.
type jsonReport struct {
	Manifest string     `json:"manifest"`
	Design   string     `json:"design"`
	Areas    []jsonArea `json:"areas"`
}

type jsonArea struct {
	Name  string     `json:"name"`
	Items []jsonItem `json:"items"`
}

type jsonItem struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Outcome  string        `json:"outcome"`
	Note     string        `json:"note,omitempty"`
	Findings []jsonFinding `json:"findings,omitempty"`
}

// jsonFinding is one finding flattened for tooling. Subject (+ Kind) is the entity a viewer deep-link
// highlights; SourceFile is which design file it was found in.
type jsonFinding struct {
	Rule       string          `json:"rule"`
	Severity   string          `json:"severity,omitempty"`
	Kind       string          `json:"kind"`
	Subject    string          `json:"subject"`
	Pin        string          `json:"pin,omitempty"`
	Message    string          `json:"message"`
	SourceFile string          `json:"source_file,omitempty"`
	Datasheets []jsonDatasheet `json:"datasheets,omitempty"`
}

// jsonDatasheet is the datasheet-side provenance of a finding: where a datasheet-backed value came
// from, so the report can show the source (and flag a low confidence) as its own column instead of
// only inside the message. Present only for a datasheet-backed finding.
type jsonDatasheet struct {
	Doc        string  `json:"doc,omitempty"`
	DocRef     string  `json:"doc_ref,omitempty"`
	Page       int32   `json:"page,omitempty"`
	Section    string  `json:"section,omitempty"`
	Method     string  `json:"method,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

// RenderJSON emits the report as indented JSON with the full finding list for every item. It is the
// tooling surface behind `agni review --format json`; the markdown renderers are the human surfaces.
func RenderJSON(r Report) (string, error) {
	out := jsonReport{Manifest: r.Manifest, Design: r.Design}
	for _, a := range r.Areas {
		ja := jsonArea{Name: a.Area.Name}
		for _, it := range a.Items {
			ji := jsonItem{
				ID:      it.Item.ID,
				Title:   it.Item.Title,
				Outcome: string(it.Outcome),
				Note:    JoinNonEmpty(it.Note, it.Item.Note),
			}
			for _, f := range it.Findings {
				ji.Findings = append(ji.Findings, jsonFinding{
					Rule:       f.Rule,
					Severity:   f.Severity,
					Kind:       f.Kind,
					Subject:    f.Subject,
					Pin:        f.Pin,
					Message:    f.Message,
					SourceFile: sourceFile(f),
					Datasheets: datasheetProv(f),
				})
			}
			ja.Items = append(ja.Items, ji)
		}
		out.Areas = append(out.Areas, ja)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// sourceFile returns the finding's source design file, or "" when provenance is absent.
func sourceFile(f check.Finding) string {
	if f.Prov == nil {
		return ""
	}
	return f.Prov.GetSourceFile()
}

// datasheetProv projects a finding's structured datasheet citations into the JSON DTO, or nil when
// the finding is not datasheet-backed (so the key is omitted). A connection-aware rule contributes
// one entry per part whose datasheet the conclusion rests on (WS3-028).
func datasheetProv(f check.Finding) []jsonDatasheet {
	if len(f.DatasheetProv) == 0 {
		return nil
	}
	out := make([]jsonDatasheet, 0, len(f.DatasheetProv))
	for _, c := range f.DatasheetProv {
		if c == nil {
			continue
		}
		out = append(out, jsonDatasheet{
			Doc:        c.Doc,
			DocRef:     c.DocRef,
			Page:       c.Page,
			Section:    c.Section,
			Method:     c.Method,
			Confidence: c.Confidence,
		})
	}
	return out
}
