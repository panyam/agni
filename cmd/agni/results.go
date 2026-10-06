package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/fshost"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// This file is the CLI edge of the checks results contract (WS3-103): writing a run to a
// self-contained document, and rendering one back.
//
// Rendering goes through the SAME writers the live commands use, so a replayed document cannot
// disagree with the tool. The parity is one writer rather than two held equal by a test.

// resultsCmd renders a written check-result document, the read half of --results-out. It loads no
// design, composes no catalog and runs no rule, so anything it renders came out of the file.
func resultsCmd() *cobra.Command {
	var format, compare string
	var coverage bool
	cmd := &cobra.Command{
		Use:   "results <file>",
		Short: "Render a written check-result document",
		Long: "Render a check-result document written by `check --results-out` or `review --results-out`. " +
			"The document is self-contained: rendering reads only the file, so a report can be archived, " +
			"shared, or read on a machine that has neither the design nor this engine's rule catalog.\n\n" +
			"A document from a check run renders as text | json | markdown | report; one from a review " +
			"run renders as markdown | json, matching what each command emits live.\n\n" +
			"--compare turns it into a differential harness: given another document (typically a vendor " +
			"report brought in with `agni import-results`), it reports which entities each run flagged — " +
			"ours only, theirs only, both — so a foreign checker becomes a gate rather than something a " +
			"person reads side by side.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// As in review, the render switch tests coverage before format, so an explicit --format
			// would be silently discarded.
			if coverageShadowsFormat(cmd, coverage) {
				return fmt.Errorf("results: --coverage emits the per-area rollup, which renders as markdown "+
					"only, so --format %q would be discarded. Pass one or the other", format)
			}
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			doc, err := results.Parse(b)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			if compare != "" {
				ob, err := os.ReadFile(compare)
				if err != nil {
					return err
				}
				other, err := results.Parse(ob)
				if err != nil {
					return fmt.Errorf("%s: %w", compare, err)
				}
				return results.WriteComparison(cmd.OutOrStdout(), results.Compare(doc, other))
			}
			if doc.GetManifest() != "" {
				return renderReviewResults(cmd.OutOrStdout(), "", doc, format, coverage)
			}
			if coverage {
				return fmt.Errorf("%s: --coverage applies to a review document; this one is a check run", args[0])
			}
			return renderCheckResults(cmd.OutOrStdout(), doc, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "output format; defaults to text for a check document and markdown for a review one")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "for a review document, emit the per-area coverage rollup instead of the per-item report")
	cmd.Flags().StringVar(&compare, "compare", "", "compare against another results document (e.g. an imported vendor report) and print the three-way entity split instead of a report")
	return cmd
}

// renderCheckResults writes a check document through the same writers `agni check` uses, so the two
// outputs match by construction.
func renderCheckResults(w io.Writer, doc *checkspb.CheckResults, format string) error {
	if format == "" {
		format = "text"
	}
	switch format {
	case "text":
		// Passes no verdicts, because a results document has no field for a considered set (see
		// OUT_OF_SCOPE.md). A replay states no coverage rather than inventing one from its findings.
		writeCheckText(w, findingsFromProto(doc.GetFindings()), len(doc.GetCatalog()), nil)
		return nil
	case "json":
		// Skipped travels back too, because `check --format json` emits it and a re-render must
		// reproduce that output byte for byte (#250).
		return writeCheckDesignJSON(w, &webapi.CheckDesignResponse{
			Findings: doc.GetFindings(),
			Skipped:  skippedFromDoc(doc.GetSkipped()),
		})
	case "csv":
		return writeCheckCSV(w, doc.GetFindings())
	case "markdown":
		return writeCheckMarkdown(w, results.Report(doc))
	case "report":
		return writeCheckReportJSON(w, results.Report(doc))
	}
	return fmt.Errorf("unknown --format %q for a check document (want: text, json, csv, markdown, report)", format)
}

// renderReviewResults rebuilds the review view-model from the document and renders it with the same
// renderers the live command uses. The reconstruction is the document's own areas and items, so a
// rendered review needs neither the manifest nor the design that produced it.
// renderReviewResults renders a review document. Its json is the Review resource the rpc returns
// (C31, agni issue 734), named when the run has a name and summarized from the document.
func renderReviewResults(w io.Writer, name string, doc *checkspb.CheckResults, format string, coverage bool) error {
	if format == "" {
		format = "markdown"
	}
	rep := review.Report{Manifest: doc.GetManifest(), Design: displayName(doc.GetDesign().GetSource())}
	for _, pa := range doc.GetAreas() {
		ar := review.AreaResult{Area: review.Area{Name: pa.GetName()}}
		for _, pi := range pa.GetItems() {
			ar.Items = append(ar.Items, review.ItemResult{
				Item:     review.Item{ID: pi.GetId(), Title: pi.GetTitle(), Note: pi.GetNote()},
				Outcome:  review.Outcome(pi.GetOutcome()),
				Findings: findingsFromProto(pi.GetFindings()),
				Unmet:    unmetFromProto(pi.GetUnmet()),
			})
		}
		rep.Areas = append(rep.Areas, ar)
	}
	var out string
	var err error
	switch {
	case coverage:
		out = review.RenderCoverageMarkdown(rep)
	case format == "markdown":
		out = review.RenderMarkdown(rep)
	case format == "json":
		out, err = protoJSON(service.ReviewOf(name, doc))
	default:
		return fmt.Errorf("unknown --format %q for a review document (want: markdown, json)", format)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, out)
	return err
}

// writeResults marshals a document to path.
func writeResults(path string, doc *checkspb.CheckResults) error {
	b, err := results.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// hashSource returns "sha256:<hex>" over a file's bytes, or "" when it cannot be read. An unreadable
// source is not an error, since the run already succeeded, and DesignRef.content_hash allows an empty
// hash.
//
// It hashes the ENTRY file only, not a hierarchical design's sub-sheets or a project's sidecars, so a
// matching hash means the same entry file and not the same design.
func hashSource(path string) string {
	return fshost.ContentHash(os.DirFS(filepath.Dir(path)), filepath.Base(path))
}

// displayName shows an artifact URI to a person as its mount-relative path.
//
// A stored document records the full URI so it names WHICH design it scored after it leaves the
// machine, while a terminal report shows the path the user just typed (#179). The PATH rather than
// the base name, because a rollup lists several designs and two gateway.edn files in different
// folders would render identically.
//
// A value that is not a URI passes through unchanged, so a document written before this renders.
func displayName(s string) string {
	if u, err := artifact.Parse(s); err == nil {
		if u.Path != "" {
			return u.Path
		}
		return u.Mount
	}
	return s
}

// forDisplay returns a copy of a check report with every source rendered for reading. It copies
// because the caller's document is what gets stored, and shortening names in place would write the
// un-portable form to disk on the next --results-out.
func forDisplay(rep *checkspb.CheckReport) *checkspb.CheckReport {
	out := proto.CloneOf(rep)
	out.Source = displayName(rep.GetSource())
	for _, s := range out.GetSections() {
		for _, g := range s.GetRules() {
			for _, f := range g.GetFindings() {
				if p := f.GetProvenance(); p.GetSourceFile() != "" {
					p.SourceFile = displayName(p.GetSourceFile())
				}
			}
		}
	}
	return out
}

// skippedFromDoc is service.SkippedRuleDocs in the other direction, for re-rendering a stored document.
func skippedFromDoc(in []*checkspb.SkippedRule) []*webapi.SkippedRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]*webapi.SkippedRule, len(in))
	for i, s := range in {
		out[i] = &webapi.SkippedRule{Name: s.GetName(), Reason: s.GetReason()}
	}
	return out
}

// protoJSON renders a wire message the way every command's --format json does (C31): protojson,
// indented, with unpopulated fields emitted so a consumer sees a zero rather than an absent key.
func protoJSON(m proto.Message) (string, error) {
	b, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
