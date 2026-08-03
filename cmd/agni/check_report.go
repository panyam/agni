package main

import (
	"fmt"
	"io"

	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
	"google.golang.org/protobuf/encoding/protojson"
)

// failsAtProto reports whether any finding sits at or above the --fail-on threshold, the CI-gate
// predicate over the wire findings a service call returns. Ranking is service.SeverityRank, so a
// custom severity (rank above error) always trips an "error" gate rather than sliding under it.
func failsAtProto(fs []*webapi.Finding, threshold string) bool {
	t := service.SeverityRank(threshold)
	for _, f := range fs {
		if service.SeverityRank(f.GetSeverity()) >= t {
			return true
		}
	}
	return false
}

// reportFindings flattens every finding out of a CheckReport's severity sections, so --fail-on gates
// on the same run the markdown/report output renders (no second check pass).
func reportFindings(rep *webapi.CheckReport) []*webapi.Finding {
	var out []*webapi.Finding
	for _, s := range rep.GetSections() {
		for _, g := range s.GetRules() {
			out = append(out, g.GetFindings()...)
		}
	}
	return out
}

// writeCheckMarkdown renders the CheckReport proto as the shareable markdown report
// (WS3-022): a severity summary table, then a section per severity (the proto's order, worst
// first), findings grouped by rule under a heading that carries the catalog Summary so the
// report reads without the tool. Rendering from the proto — not from raw findings — keeps
// this and the web report panel showing one canonical pivot.
func writeCheckMarkdown(w io.Writer, rep *webapi.CheckReport) error {
	fmt.Fprintf(w, "# agni check — %s\n\n", rep.GetSource())
	total := 0
	for _, s := range rep.GetSections() {
		total += int(s.GetCount())
	}
	if total == 0 {
		fmt.Fprintf(w, "No findings (%d rule(s) run).\n", rep.GetRulesRun())
		return nil
	}
	fmt.Fprintln(w, "| severity | findings |")
	fmt.Fprintln(w, "|---|---|")
	for _, s := range rep.GetSections() {
		fmt.Fprintf(w, "| %s | %d |\n", s.GetSeverity(), s.GetCount())
	}
	fmt.Fprintf(w, "\n%d finding(s), %d rule(s) run.\n", total, rep.GetRulesRun())

	for _, s := range rep.GetSections() {
		fmt.Fprintf(w, "\n## %s\n", s.GetSeverity())
		for _, g := range s.GetRules() {
			heading := g.GetRule()
			if g.GetSummary() != "" {
				heading += " — " + g.GetSummary()
			}
			fmt.Fprintf(w, "\n### %s\n", heading)
			for _, f := range g.GetFindings() {
				src := ""
				if f.GetProvenance().GetSourceFile() != "" {
					src = " (" + f.GetProvenance().GetSourceFile() + ")"
				}
				fmt.Fprintf(w, "- `%s` — %s%s\n", f.GetSubject().GetRef(), f.GetMessage(), src)
			}
		}
	}
	return nil
}

// writeCheckReportJSON emits the report as a GetCheckReportResponse in protojson form, the
// same wire shape the RPC returns (mirroring writeCheckJSON's contract for the findings
// array), so CI tooling parses one shape whether it shells out or calls the API.
func writeCheckReportJSON(w io.Writer, rep *webapi.CheckReport) error {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(&webapi.GetCheckReportResponse{Report: rep})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
