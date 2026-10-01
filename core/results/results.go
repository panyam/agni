// Package results reads and writes the check-result document (agni.v1.checks.CheckResults), the
// artifact half of the checks contract (WS3-103).
//
// The document is self-contained, so a report renders with no design file, no rule catalog and no
// engine build present. That holds only while the rendering path is shared, which is why the severity
// pivot lives here and both the live service and a reloaded document call it. See
// docsite/content/architecture/checks-contract.md#the-document.
//
// The package sits low in the stack, reading only the generated contract and check's rule type
// (C17). Nothing here knows about transport, mounts, or files.
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
// own, since two documents are only comparable once you know which tool made each.
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
// severity, one-line summary, and the classification tags. The long-form rule prose stays in the
// engine's catalog, rather than being repeated across every archived run.
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
// It takes wire-form findings and a rule -> summary lookup rather than a catalog, so a live run and a
// reloaded document both call it. rulesRun is passed in rather than derived from the findings, because
// it is what distinguishes a clean design from a run that checked nothing.
//
// Re-pivoting the findings of a report it produced yields the same report, which is what lets a
// document written from a report round-trip.
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
// why the catalog is in the document at all.
func Report(doc *checkspb.CheckResults) *checkspb.CheckReport {
	summaries := make(map[string]string, len(doc.GetCatalog()))
	for _, r := range doc.GetCatalog() {
		summaries[r.GetName()] = r.GetSummary()
	}
	return Pivot(doc.GetDesign().GetSource(), doc.GetFindings(), summaries, len(doc.GetCatalog()))
}

// Marshal encodes a document as indented protojson, the encoding the datasheet workbench uses for a
// PartSpec sibling, because a results document is a file a human opens, greps, and diffs. Unpopulated
// fields are omitted.
func Marshal(doc *checkspb.CheckResults) ([]byte, error) {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Parse decodes a document and rejects one this build cannot faithfully read.
//
// An unknown schema version is an error rather than a best-effort read, because half-reading a future
// document yields a findings list shorter than the run that made it, with nothing to say so. Unknown
// FIELDS within a known schema are tolerated (protojson's default), since those are additive by the
// versioning rule on Schema. See docsite/content/architecture/checks-contract.md#versioning.
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
