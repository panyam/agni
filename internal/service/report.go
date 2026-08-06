package service

import (
	"context"
	"sort"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// SeverityRank orders severities for report sections and fail-on gating; higher is worse.
// An unknown severity ranks above error so a provider's custom level is never silently
// dropped below a CI gate or buried at the bottom of a report.
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

// CheckReportProto assembles the severity-organized CheckReport (WS3-022) from one run:
// sections worst first (custom severities lead, then error, warning, info), empty severities
// omitted, findings grouped by rule in run order, each group stamped with the rule's catalog
// Summary from the rules that ran (the injected catalog, so an embedder's rules resolve too).
// This is THE report pivot: the CLI's markdown/report formats and the web report panel render
// this proto rather than re-deriving the grouping on each side.
func CheckReportProto(source string, fs []check.Finding, rules []*check.Rule) *webapi.CheckReport {
	summaries := make(map[string]string, len(rules))
	for _, r := range rules {
		summaries[r.Name] = r.Summary
	}

	bySeverity := map[string][]check.Finding{}
	for _, f := range fs {
		bySeverity[f.Severity] = append(bySeverity[f.Severity], f)
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

	rep := &webapi.CheckReport{Source: source, RulesRun: int32(len(rules))}
	for _, sev := range order {
		sfs := bySeverity[sev]
		section := &webapi.CheckReport_SeveritySection{Severity: sev, Count: int32(len(sfs))}
		groups := map[string]*webapi.CheckReport_RuleGroup{}
		for _, f := range sfs {
			g := groups[f.Rule]
			if g == nil {
				g = &webapi.CheckReport_RuleGroup{Rule: f.Rule, Summary: summaries[f.Rule]}
				groups[f.Rule] = g
				section.Rules = append(section.Rules, g)
			}
			g.Findings = append(g.Findings, FindingProto(f))
		}
		rep.Sections = append(rep.Sections, section)
	}
	return rep
}

// GetCheckReport runs the same selected rules CheckDesign runs and returns their severity
// pivot. A separate RPC (not a client-side regrouping of CheckDesign) so every consumer
// renders the one canonical report shape. Findings carry the same sheet annotation as
// CheckDesign's (WS9-024), so the report panel shares the sheet-navigation join.
func (s *CheckService) GetCheckReport(ctx context.Context, req *webapi.GetCheckReportRequest) (*webapi.GetCheckReportResponse, error) {
	ov, err := ComposeOverlay(req.GetOverlay())
	if err != nil {
		return nil, err
	}
	m, err := BuildModel(ctx, s.loader, req.GetMount(), req.GetPath(), "", s.specs, ov.ReadOptions()...)
	if err != nil {
		return nil, err
	}
	rules := ov.Catalog(s.catalog).Filter(check.Facets{Names: req.GetRules()})
	rep := CheckReportProto(req.GetPath(), check.Run(m, rules), rules)
	AnnotateReport(rep, BuildGeometry(ctx, s.loader, req.GetMount(), req.GetPath()), m)
	return &webapi.GetCheckReportResponse{Report: rep}, nil
}

// AnnotateReport fills the sheet membership of every finding nested in a CheckReport, the
// report-shaped counterpart of AnnotateSheets (which takes a flat slice). It flattens the
// report's severity sections and rule groups to one slice and annotates in place, so the CLI's
// `check --format report` and the GetCheckReport RPC share one annotation path rather than two
// copies of the walk. Both g and d nil is a no-op (findings keep empty sheets), same as
// AnnotateSheets; a net-only channel (design set, geometry nil) still annotates net subjects.
func AnnotateReport(rep *webapi.CheckReport, g *geom.SchematicGeometry, m NetSource) {
	var all []*webapi.Finding
	for _, sec := range rep.GetSections() {
		for _, rg := range sec.GetRules() {
			all = append(all, rg.GetFindings()...)
		}
	}
	AnnotateSheets(all, g, m)
}
