// Package foreign imports a check-result document from another tool's report (WS3-104).
//
// It is NOT a formats registry entry. Every registry capability answers a question about a DESIGN
// file, and a results file describes a design it does not contain, so it could never answer
// ReadDesign. The Loader produces a model; this produces evidence ABOUT one.
//
// The imported document stays visibly WEAKER than a native run. A vendor report is a flat violation
// list with no not-applicable, no needs-data, no coverage axis and no per-item traceability, and
// manufacturing that structure would turn "this tool said nothing about X" into "X is fine". So
// meta.coverage_axis is false, and the import reports its own residue. See
// docsite/content/architecture/checks-contract.md#importing-another-tools-results.
package foreign

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Producer names are stamped into meta.producer so two documents are comparable only once a reader
// knows which tool made each.
const (
	ProducerKiCadDRC = "kicad-cli pcb drc"
	ProducerKiCadERC = "kicad-cli sch erc"
)

// RulePrefixDRC and RulePrefixERC namespace a foreign checker's rule names. A vendor's `type` string
// is carried VERBATIM behind a prefix rather than translated into one of ours, because mapping
// `copper_edge_clearance` onto a rule of ours would assert an equivalence nobody verified. The prefix
// keeps a foreign finding visibly foreign in any report it lands in.
const (
	RulePrefixDRC = "kicad-drc/"
	RulePrefixERC = "kicad-erc/"
)

// kicadReport is the shape both kicad-cli JSON reports share. DRC fills violations,
// unconnected_items and schematic_parity; ERC fills sheets. The two schemas populate disjoint
// sections, so one struct reads both and lets Read sniff the kind from the document itself.
type kicadReport struct {
	Schema       string `json:"$schema"`
	Source       string `json:"source"`
	KiCadVersion string `json:"kicad_version"`
	Units        string `json:"coordinate_units"`

	Violations       []kicadViolation `json:"violations"`
	UnconnectedItems []kicadViolation `json:"unconnected_items"`
	SchematicParity  []kicadViolation `json:"schematic_parity"`
	Sheets           []struct {
		Path       string           `json:"path"`
		Violations []kicadViolation `json:"violations"`
	} `json:"sheets"`
}

type kicadViolation struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Items       []struct {
		Description string `json:"description"`
		UUID        string `json:"uuid"`
	} `json:"items"`
}

// ReadKiCad reads a kicad-cli DRC or ERC JSON report into a results document, choosing between them
// from the report's own `$schema`. now is passed in so a caller can produce a byte-stable document.
//
// The findings it returns carry no subject, because attaching them to entities needs a model, which
// this does not have and should not need. Call Join for that.
func ReadKiCad(r io.Reader, now time.Time) (*checkspb.CheckResults, error) {
	var rep kicadReport
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return nil, fmt.Errorf("kicad report: %w", err)
	}
	producer, prefix, err := kindOf(rep)
	if err != nil {
		return nil, err
	}
	doc := &checkspb.CheckResults{
		Meta: &checkspb.ResultsMeta{
			Schema:          results.Schema,
			Producer:        producer,
			ProducerVersion: rep.KiCadVersion,
			CreatedAt:       now.UTC().Format(time.RFC3339),
			// A flat violation list distinguishes nothing beyond "reported" and "not reported".
			CoverageAxis: false,
		},
		Design: &checkspb.DesignRef{Source: rep.Source},
	}
	for _, v := range rep.Violations {
		doc.Findings = append(doc.Findings, findings(v, prefix, rep.Source)...)
	}
	for _, v := range rep.UnconnectedItems {
		doc.Findings = append(doc.Findings, findings(v, prefix, rep.Source)...)
	}
	for _, v := range rep.SchematicParity {
		doc.Findings = append(doc.Findings, findings(v, prefix, rep.Source)...)
	}
	for _, sh := range rep.Sheets {
		for _, v := range sh.Violations {
			doc.Findings = append(doc.Findings, findings(v, prefix, rep.Source)...)
		}
	}
	return doc, nil
}

// kindOf decides DRC vs ERC from the report's declared schema, falling back to which sections are
// populated. An unrecognized document is an error rather than an empty import, since zero findings
// would read like a design the tool found nothing wrong with.
func kindOf(rep kicadReport) (producer, prefix string, err error) {
	switch {
	case strings.Contains(rep.Schema, "drc"):
		return ProducerKiCadDRC, RulePrefixDRC, nil
	case strings.Contains(rep.Schema, "erc"):
		return ProducerKiCadERC, RulePrefixERC, nil
	case len(rep.Sheets) > 0:
		return ProducerKiCadERC, RulePrefixERC, nil
	case len(rep.Violations)+len(rep.UnconnectedItems) > 0:
		return ProducerKiCadDRC, RulePrefixDRC, nil
	}
	return "", "", fmt.Errorf("kicad report: not a recognizable DRC or ERC report (no $schema, no sheets, no violations)")
}

// findings turns one violation into one finding PER ITEM it names. A KiCad clearance violation names
// two items and both are implicated, so collapsing them to one finding would silently pick a side.
// The description repeats across the items of one violation, since it is one problem seen from each end.
func findings(v kicadViolation, prefix, source string) []*checkspb.Finding {
	rule := prefix + v.Type
	sev := severity(v.Severity)
	if len(v.Items) == 0 {
		return []*checkspb.Finding{{
			Rule:     rule,
			Severity: sev,
			Message:  v.Description,
			Subject:  &checkspb.Subject{},
			// itemDescription is empty, so Join will class this as a violation with no items.
			Provenance: &ir.Provenance{SourceFile: source},
		}}
	}
	out := make([]*checkspb.Finding, 0, len(v.Items))
	for _, it := range v.Items {
		msg := v.Description
		if it.Description != "" {
			msg = v.Description + " — " + it.Description
		}
		out = append(out, &checkspb.Finding{Subject: &checkspb.Subject{}, Rule: rule, Severity: sev, Message: msg, Provenance: &ir.Provenance{
			SourceFile:   source,
			NativeId:     it.UUID,
			NativeIdKind: "kicad-uuid",
		}})
	}
	return out
}

// severity maps KiCad's levels onto ours. "exclusion" is a violation the user acknowledged and
// suppressed, so it becomes info. Dropping it would lose the evidence, and passing it through verbatim
// would sort it to the top, since SeverityRank puts an unrecognized level ABOVE error.
func severity(s string) string {
	switch strings.ToLower(s) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "exclusion", "ignore", "info", "debug":
		return "info"
	}
	return "warning"
}

// summarize builds the ImportSummary from the per-finding join outcomes, bucketing the residue by
// class and keeping a few verbatim examples of each so a reader can judge whether it is benign.
func summarize(total, joined int, unjoined map[string][]string) *checkspb.ImportSummary {
	s := &checkspb.ImportSummary{Findings: int32(total), Joined: int32(joined)}
	reasons := make([]string, 0, len(unjoined))
	for r := range unjoined {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	for _, r := range reasons {
		ex := unjoined[r]
		sort.Strings(ex)
		ex = dedupe(ex)
		if len(ex) > maxExamples {
			ex = ex[:maxExamples]
		}
		s.Unjoined = append(s.Unjoined, &checkspb.UnjoinedReason{
			Reason:   r,
			Count:    int32(len(unjoined[r])),
			Examples: ex,
		})
	}
	return s
}

// maxExamples bounds the residue examples to enough to recognize a shape, and too few to become a
// second copy of the findings list.
const maxExamples = 3

func dedupe(in []string) []string {
	out := in[:0:0]
	var prev string
	for i, s := range in {
		if i == 0 || s != prev {
			out = append(out, s)
		}
		prev = s
	}
	return out
}
