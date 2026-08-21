package main

import (
	"io"
	"strings"

	"github.com/panyam/agni/internal/artifact"
	"time"

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
func writeVerdictHTML(w io.Writer, resp *webapi.CheckDesignResponse, rules []*check.Rule,
	source, contentHash, urlBase, mountPath string) error {
	return rpt.HTML(w, buildVerdictReport(resp, rules, rpt.Report{
		Design:      source,
		Generated:   time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		ContentHash: contentHash,
		URLBase:     urlBase,
		MountPath:   mountPath,
	}))
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
// THE MOUNT HAS TO BE ONE THE OPERATOR NAMED. A bare file path is minted a mount locally, from the
// enclosing project or as "local" (see cliWorkspace.mint), and that name means nothing on a server
// the operator did not start with it. Emitting a URL from it produces a link that resolves nowhere,
// which reads as a broken tool rather than a mismatched setup, so an argument that was not written as
// mount:// gets no link at all. That is the correct answer rather than a gap (agni issue 392).
//
// The operator asking for links with --url-base is not the same as the operator telling us the mount
// is real, which is why both are required and neither implies the other.
func linkablePath(arg, designURI string) string {
	if !strings.HasPrefix(arg, "mount://") {
		return ""
	}
	u, err := artifact.Parse(designURI)
	if err != nil || u.Mount == "" || u.Path == "" {
		return ""
	}
	return u.Mount + "/" + u.Path
}
