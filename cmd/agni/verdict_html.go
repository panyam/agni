package main

import (
	"io"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// writeVerdictHTML renders the run as a self-contained report page.
//
// It takes the COMPOSED catalog rather than the built-in one, so an overlay's own rules carry their
// prose here exactly as a built-in does. The catalog is also the only place StatesConsideredSet can
// come from: the verdict list alone cannot distinguish a converted rule that found no subjects from
// one that was never converted, and guessing would make the report claim coverage the run never had.
func writeVerdictHTML(w io.Writer, resp *webapi.CheckDesignResponse, rules []*check.Rule, meta rpt.Report) error {
	return rpt.HTML(w, buildVerdictReport(resp, rules, meta))
}

// buildVerdictReport aggregates one run into the shared report model, which BOTH renderers take. It
// is one function rather than one per format so the html page and the terminal cannot disagree about
// what the run contained or what order to meet it in, which is the drift agni issue 380 describes.
//
// It takes the COMPOSED catalog rather than the built-in one, so an overlay's own rules carry their
// prose exactly as a built-in does.
func buildVerdictReport(resp *webapi.CheckDesignResponse, rules []*check.Rule, meta rpt.Report) rpt.Report {
	verdicts := make([]check.Verdict, 0, len(resp.GetVerdicts()))
	for _, v := range resp.GetVerdicts() {
		verdicts = append(verdicts, service.VerdictFromProto(v))
	}
	return rpt.Build(verdicts, findingsFromProtos(resp.GetFindings()), rules, meta)
}

// findingsFromProtos carries the few fields a report row needs back across the wire. It is local to
// the CLI rather than a service export because it is a presentation adapter: the report shows a
// finding's sentence and its subject, not its provenance or its datasheet citations.
func findingsFromProtos(fs []*checkspb.Finding) []check.Finding {
	out := make([]check.Finding, 0, len(fs))
	for _, f := range fs {
		out = append(out, check.Finding{Subject: check.Entity{Kind: f.GetSubject().GetKind(), Ref: f.GetSubject().GetRef(), Pin: f.GetSubject().GetPin()}, Rule: f.GetRule(), Severity: f.GetSeverity(), Message: f.GetMessage()})
	}
	return out
}
