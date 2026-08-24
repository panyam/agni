package main

import (
	"io"

	"github.com/panyam/agni/internal/artifact"

	"github.com/panyam/agni/core/check"
	rpt "github.com/panyam/agni/core/report"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/service"
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

// linkablePath is the path half of a viewer URL, and "" whenever a link would be a guess.
//
// THE MOUNT HAS TO BE ONE THE OPERATOR NAMED. A bare file path that no declared mount covers is
// minted one locally, from the enclosing project or as "local" (see cliWorkspace.mint), and that name
// means nothing on a server the operator did not start with it. Emitting a URL from it produces a
// link that resolves nowhere, which reads as a broken tool rather than a mismatched setup, so a
// minted mount gets no link at all. That is the correct answer rather than a gap (agni issue 392).
//
// It asks the WORKSPACE whether the mount was declared, rather than whether the argument was spelled
// mount://. The spelling was a proxy for the same question and answered a narrower one: a plain path
// through a --mount the operator passed got no link either, though its mount is exactly as real as
// the spelled form's (agni issue 459). Since agni.yaml carries a mount table the CLI and a server
// both read, that gap became the common case rather than an edge one.
//
// What it does NOT verify is that the server's mount of that name has the same root. Nothing ever
// did, including the spelled form, and the content hash on the URL is the mitigation: the viewer can
// say the link was computed against different bytes rather than silently highlight the wrong pin.
//
// The operator asking for links with --url-base is not the same as the operator naming the mount,
// which is why both are required and neither implies the other.
func linkablePath(ws *cliWorkspace, designURI string) string {
	if ws == nil {
		return ""
	}
	u, err := artifact.Parse(designURI)
	if err != nil || u.Mount == "" || u.Path == "" {
		return ""
	}
	if !ws.Declared(u.Mount) {
		return ""
	}
	return u.Mount + "/" + u.Path
}
