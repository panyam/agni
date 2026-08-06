// Package results reads and writes the check-result document (agni.v1.checks.CheckResults), the
// artifact half of the checks contract (WS3-103).
//
// The document is meant to be self-contained: everything needed to render a run is inside it, so a
// report can be archived, mailed, diffed against another revision, or handed to a reviewer with no
// design file, no rule catalog, and no engine build present. That property is only real if the
// rendering path is shared rather than reimplemented, which is why the severity pivot lives here and
// both the live service and a reloaded document call it.
//
// The package is deliberately low in the stack: it reads the generated contract and check's rule
// type, and nothing above (C17). A results document is evidence about a design, not a service
// response, so nothing here knows about transport, mounts, or files.
package results

import (
	"fmt"
	"sort"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"google.golang.org/protobuf/encoding/protojson"
)

// Schema is the document schema version stamped into every document this package writes, and the
// only value Parse accepts. It changes when a consumer that understood the old shape would
// misread the new one; an additive field is not that, so it is not a version bump.
const Schema = "agni.checks.results/v1"

// Producer is the producer name a native engine run stamps. A foreign checker's import stamps its
// own, which is the whole point of recording it: two documents are only comparable once you know
// which tool made each.
const Producer = "agni"

// SeverityRank orders severities for report sections and fail-on gating; higher is worse. An unknown
// severity ranks above error so a provider's custom level is never silently dropped below a CI gate
// or buried at the bottom of a report.
func SeverityRank(s string) int {
	switch s {
	case "info":
		return 0
	case "warning":
		return 1
	case "error":
		return 2
	}
	return 3
}

// RuleRecords snapshots the rules a run evaluated into the document's lean catalog form: identity,
// severity, one-line summary, and the classification tags. The long-form rule prose is deliberately
// left behind — it belongs to the engine's catalog surface, and copying it into every document would
// repeat the same markdown across every archived run.
func RuleRecords(rules []*check.Rule) []*checkspb.RuleRecord {
	out := make([]*checkspb.RuleRecord, 0, len(rules))
	for _, r := range rules {
		out = append(out, &checkspb.RuleRecord{
			Name:     r.Name,
			Severity: r.Severity,
			Summary:  r.Summary,
			Tags:     r.Tags,
		})
	}
	return out
}

// Pivot is THE severity-organized report pivot (WS3-022): sections worst-severity first (an unknown
// severity leads, then error, warning, info), empty severities omitted, findings grouped by rule in
// input order, each group stamped with the rule's summary so a report reads without the tool at hand.
//
// It takes findings already in wire form, and a rule -> summary lookup rather than a rule catalog, so
// the same function serves a live run (summaries from the catalog that ran) and a reloaded document
// (summaries from its own snapshot). rulesRun is carried rather than derived from the findings,
// because it is what distinguishes a clean design from a run that checked nothing.
//
// The pivot is idempotent under its own flattening: re-pivoting the findings of a report it produced
// yields the same report, since grouping is stable and section order is recomputed from the same
// severities. That is what lets a document written from a report round-trip.
func Pivot(source string, fs []*checkspb.Finding, summaries map[string]string, rulesRun int) *checkspb.CheckReport {
	bySeverity := map[string][]*checkspb.Finding{}
	for _, f := range fs {
		bySeverity[f.GetSeverity()] = append(bySeverity[f.GetSeverity()], f)
	}
	order := make([]string, 0, len(bySeverity))
	for s := range bySeverity {
		order = append(order, s)
	}
	sort.Slice(order, func(i, j int) bool {
		ri, rj := SeverityRank(order[i]), SeverityRank(order[j])
		if ri != rj {
			return ri > rj
		}
		return order[i] < order[j]
	})

	rep := &checkspb.CheckReport{Source: source, RulesRun: int32(rulesRun)}
	for _, sev := range order {
		sfs := bySeverity[sev]
		section := &checkspb.CheckReport_SeveritySection{Severity: sev, Count: int32(len(sfs))}
		groups := map[string]*checkspb.CheckReport_RuleGroup{}
		for _, f := range sfs {
			g := groups[f.GetRule()]
			if g == nil {
				g = &checkspb.CheckReport_RuleGroup{Rule: f.GetRule(), Summary: summaries[f.GetRule()]}
				groups[f.GetRule()] = g
				section.Rules = append(section.Rules, g)
			}
			g.Findings = append(g.Findings, f)
		}
		rep.Sections = append(rep.Sections, section)
	}
	return rep
}

// Report rebuilds a document's severity pivot from the document alone: the findings it carries, the
// summaries in its catalog snapshot, and the size of that snapshot as the rules-run count. This is
// the reason the catalog is in the document at all.
func Report(doc *checkspb.CheckResults) *checkspb.CheckReport {
	summaries := make(map[string]string, len(doc.GetCatalog()))
	for _, r := range doc.GetCatalog() {
		summaries[r.GetName()] = r.GetSummary()
	}
	return Pivot(doc.GetDesign().GetSource(), doc.GetFindings(), summaries, len(doc.GetCatalog()))
}

// Marshal encodes a document as indented protojson, the same encoding the datasheet workbench uses
// for a PartSpec sibling: a results document is a file a human opens, greps, and diffs, so a text
// encoding is worth more than a compact one. Unpopulated fields are omitted, because an archived
// artifact should carry what was true rather than a full skeleton of what was not.
func Marshal(doc *checkspb.CheckResults) ([]byte, error) {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse decodes a document and rejects one this build cannot faithfully read.
//
// An unknown schema version is an error rather than a best-effort read: a results document exists so
// that silence is never mistaken for coverage, and half-reading a future document would produce
// exactly that — a findings list shorter than the run that made it, with nothing to say so. Unknown
// FIELDS within a known schema are tolerated (protojson's default), since those are additive by the
// versioning rule above.
func Parse(b []byte) (*checkspb.CheckResults, error) {
	doc := &checkspb.CheckResults{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, doc); err != nil {
		return nil, fmt.Errorf("results document: %w", err)
	}
	if got := doc.GetMeta().GetSchema(); got != Schema {
		if got == "" {
			return nil, fmt.Errorf("results document: no meta.schema (want %s); this does not look like a check-result document", Schema)
		}
		return nil, fmt.Errorf("results document: schema %s is not %s; this build cannot read it faithfully", got, Schema)
	}
	return doc, nil
}
