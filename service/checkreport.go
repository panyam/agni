package service

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/report"
	"github.com/panyam/agni/core/results"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// WithEnv sets the deployment provenance a written document records: the producer version, and
// whether profiles and intent were composed into the catalog. It is the same ReviewEnv a review run
// records, so a check document and a review document from one deployment state the same build.
func (s *CheckService) WithEnv(env ReviewEnv) *CheckService {
	s.env = env
	return s
}

// RenderCheckReport writes a run the caller already holds as a file to keep (agni issue 127). It runs
// no rules. The JSON form is the CheckResults document `agni check --results-out` writes, and the
// HTML form is the page `agni check --verdicts --format html` writes, and both CLI paths call this,
// so a report saved from the viewer and one the CLI wrote differ only in their timestamps.
//
// The request's uri, overlay, rules, board_uri and as_named have to be the ones the run used. They
// decide the catalog the report describes and the netlist tier whose content hash it records, and
// nothing here can check them against the run.
func (s *CheckService) RenderCheckReport(ctx context.Context, req *webapi.RenderCheckReportRequest) (*webapi.RenderCheckReportResponse, error) {
	u, err := ParseArtifactURI(req.GetUri())
	if err != nil {
		return nil, err
	}
	if req.GetRun() == nil {
		return nil, fmt.Errorf("%w: RenderCheckReport needs the run to write", ErrInvalidArgument)
	}
	ov, err := s.projects.Overlay(ctx, u, req.GetOverlay(), s.baseConvention)
	if err != nil {
		return nil, err
	}
	cat, err := ov.Catalog(s.catalog)
	if err != nil {
		return nil, err
	}
	board, err := optionalArtifactURI(req.GetBoardUri())
	if err != nil {
		return nil, err
	}
	nu, _, _, err := s.projects.TierURIs(ctx, u, board, req.GetAsNamed())
	if err != nil {
		return nil, err
	}
	// The hash is provenance, so an unreadable source records none, as a review's does. It is the
	// netlist tier's, since a design folder does not hash.
	hash, err := s.loader.DesignHash(ctx, nu)
	if err != nil {
		hash = ""
	}
	run := req.GetRun()
	name := reportBaseName(u.Path)
	switch req.GetFormat() {
	case webapi.CheckReportFormat_CHECK_REPORT_FORMAT_RESULTS_JSON:
		doc := &checkspb.CheckResults{
			Meta: &checkspb.ResultsMeta{
				Schema:          results.Schema,
				Producer:        results.Producer,
				ProducerVersion: s.env.ProducerVersion,
				CreatedAt:       time.Now().UTC().Format(time.RFC3339),
				// A native run records what it could NOT check as well as what it found. See
				// docsite/content/architecture/checks-contract.md#the-outcome-vocabulary-names-every-way-a-question-went-unanswered.
				CoverageAxis: true,
			},
			Design: &checkspb.DesignRef{Source: req.GetUri(), ContentHash: hash},
			// Provenance comes off the resolved overlay the run used, as a review's does. See
			// docsite/content/architecture/checks-contract.md#provenance-is-read-off-the-resolved-overlay.
			Run: RunConfigProto(ov.Provenance(RunProvenance{
				Params:      s.specs != nil,
				Profiles:    s.env.Profiles,
				Intent:      s.env.Intent,
				Conventions: req.GetOverlay().GetConfig().GetConventions().GetName(),
			}), 0),
			Catalog:  results.RuleRecords(cat.Filter(check.Facets{Names: req.GetRules()})),
			Skipped:  SkippedRuleDocs(run.GetSkipped()),
			Findings: run.GetFindings(),
		}
		b, err := results.Marshal(doc)
		if err != nil {
			return nil, err
		}
		return &webapi.RenderCheckReportResponse{Content: b, ContentType: "application/json", Filename: name + ".agni-check.json"}, nil
	case webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML:
		links := req.GetLinks()
		mountPath := ""
		if links.GetDesignUri() != "" {
			lu, err := ParseArtifactURI(links.GetDesignUri())
			if err != nil {
				return nil, err
			}
			mountPath = strings.TrimPrefix(lu.String(), "mount://")
		}
		meta := report.Report{
			Design:        req.GetUri(),
			Generated:     time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
			ContentHash:   hash,
			URLBase:       strings.TrimSuffix(links.GetUrlBase(), "/"),
			MountPath:     mountPath,
			LinksWithheld: links.GetWithheld(),
		}
		var buf bytes.Buffer
		// The composed catalog, not the selection, so a rule an overlay added carries its prose like a
		// built-in does (agni issue 411).
		if err := report.HTML(&buf, VerdictReport(run, cat.Rules(), meta)); err != nil {
			return nil, err
		}
		return &webapi.RenderCheckReportResponse{Content: buf.Bytes(), ContentType: "text/html; charset=utf-8", Filename: name + ".agni-check.html"}, nil
	default:
		return nil, fmt.Errorf("%w: RenderCheckReport needs a format", ErrInvalidArgument)
	}
}

// VerdictReport aggregates one run into the report model the HTML page and the CLI's verdict text
// both render, so the two cannot disagree about what the run held or in what order (agni issue 380).
// rules is the composed catalog the run read, overlay rules included.
func VerdictReport(run *webapi.CheckDesignResponse, rules []*check.Rule, meta report.Report) report.Report {
	verdicts := make([]check.Verdict, 0, len(run.GetVerdicts()))
	for _, v := range run.GetVerdicts() {
		verdicts = append(verdicts, VerdictFromProto(v))
	}
	findings := make([]check.Finding, 0, len(run.GetFindings()))
	for _, f := range run.GetFindings() {
		findings = append(findings, check.Finding{
			Subject:  check.Entity{Kind: f.GetSubject().GetKind(), Ref: f.GetSubject().GetRef(), Pin: f.GetSubject().GetPin()},
			Rule:     f.GetRule(),
			Severity: f.GetSeverity(),
			Message:  f.GetMessage(),
		})
	}
	return report.Build(verdicts, findings, rules, meta)
}

// SkippedRuleDocs carries a run's skipped rules into a document's form.
func SkippedRuleDocs(in []*webapi.SkippedRule) []*checkspb.SkippedRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]*checkspb.SkippedRule, len(in))
	for i, s := range in {
		out[i] = &checkspb.SkippedRule{Name: s.GetName(), Reason: s.GetReason()}
	}
	return out
}

// reportBaseName names a saved report after the design: its file without the extension, or its
// folder when the uri names one.
func reportBaseName(p string) string {
	b := path.Base(strings.TrimSuffix(p, "/"))
	if b == "." || b == "/" || b == "" {
		return "design"
	}
	if ext := path.Ext(b); ext != "" && ext != b {
		b = strings.TrimSuffix(b, ext)
	}
	return b
}
