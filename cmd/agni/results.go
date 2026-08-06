package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/results"
	"github.com/panyam/agni/core/review"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/version"
	"github.com/spf13/cobra"
)

// This file is the CLI edge of the checks results contract (WS3-103): writing a run to a
// self-contained document, and rendering one back.
//
// The rendering side deliberately reuses the SAME writers the live commands use. A results document
// that rendered through a second set of writers would be a plausible-looking artifact that quietly
// disagreed with the tool, which is the failure this contract exists to prevent — so the parity is
// structural (one writer) rather than asserted (two writers held equal by a test).

// resultsCmd renders a written check-result document. It is the read half of --results-out, and the
// proof that the document is self-contained: it loads no design, composes no catalog, and runs no
// rule, so anything it can render came out of the file.
func resultsCmd() *cobra.Command {
	var format string
	var coverage bool
	cmd := &cobra.Command{
		Use:   "results <file>",
		Short: "Render a written check-result document",
		Long: "Render a check-result document written by `check --results-out` or `review --results-out`. " +
			"The document is self-contained: rendering reads only the file, so a report can be archived, " +
			"shared, or read on a machine that has neither the design nor this engine's rule catalog.\n\n" +
			"A document from a check run renders as text | json | markdown | report; one from a review " +
			"run renders as markdown | json, matching what each command emits live.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			doc, err := results.Parse(b)
			if err != nil {
				return fmt.Errorf("%s: %w", args[0], err)
			}
			if doc.GetManifest() != "" {
				return renderReviewResults(cmd.OutOrStdout(), doc, format, coverage)
			}
			if coverage {
				return fmt.Errorf("%s: --coverage applies to a review document; this one is a check run", args[0])
			}
			return renderCheckResults(cmd.OutOrStdout(), doc, format)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "output format; defaults to text for a check document and markdown for a review one")
	cmd.Flags().BoolVar(&coverage, "coverage", false, "for a review document, emit the per-area coverage rollup instead of the per-item report")
	return cmd
}

// renderCheckResults writes a check document through the same writers `agni check` uses, so the two
// outputs are identical by construction rather than by convention.
func renderCheckResults(w io.Writer, doc *checkspb.CheckResults, format string) error {
	if format == "" {
		format = "text"
	}
	switch format {
	case "text":
		writeCheckText(w, findingsFromProto(doc.GetFindings()), len(doc.GetCatalog()))
		return nil
	case "json":
		return writeCheckDesignJSON(w, &webapi.CheckDesignResponse{Findings: doc.GetFindings()})
	case "markdown":
		return writeCheckMarkdown(w, results.Report(doc))
	case "report":
		return writeCheckReportJSON(w, results.Report(doc))
	}
	return fmt.Errorf("unknown --format %q for a check document (want: text, json, markdown, report)", format)
}

// renderReviewResults rebuilds the review view-model from the document and renders it with the same
// renderers the live command uses. The reconstruction is the document's own areas and items, so a
// rendered review needs neither the manifest nor the design that produced it.
func renderReviewResults(w io.Writer, doc *checkspb.CheckResults, format string, coverage bool) error {
	if format == "" {
		format = "markdown"
	}
	rep := review.Report{Manifest: doc.GetManifest(), Design: doc.GetDesign().GetSource()}
	for _, pa := range doc.GetAreas() {
		ar := review.AreaResult{Area: review.Area{Name: pa.GetName()}}
		for _, pi := range pa.GetItems() {
			ar.Items = append(ar.Items, review.ItemResult{
				Item:     review.Item{ID: pi.GetId(), Title: pi.GetTitle(), Note: pi.GetNote()},
				Outcome:  review.Outcome(pi.GetOutcome()),
				Findings: findingsFromProto(pi.GetFindings()),
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
		out, err = review.RenderJSON(rep)
	default:
		return fmt.Errorf("unknown --format %q for a review document (want: markdown, json)", format)
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, out)
	return err
}

// resultsDoc assembles a document from one run. Callers supply what only they know — the findings,
// the rules that ran, and which overlay tiers were attached — and this stamps the producer identity
// and the design's revision hash.
//
// The hash is computed from the source file here rather than by the check service, because the
// service is deliberately filesystem-free: it resolves an opaque mount/path key through an injected
// loader and never reads bytes itself (C13). Hashing is the producing edge's job for the same reason
// reading the design is.
func resultsDoc(source string, rules []*check.Rule, findings []*checkspb.Finding, run *checkspb.RunConfig) *checkspb.CheckResults {
	return &checkspb.CheckResults{
		Meta: &checkspb.ResultsMeta{
			Schema:          results.Schema,
			Producer:        results.Producer,
			ProducerVersion: version.Version(),
			CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		},
		Design:   &checkspb.DesignRef{Source: source, ContentHash: hashSource(source)},
		Run:      run,
		Catalog:  results.RuleRecords(rules),
		Findings: findings,
	}
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
// source is not an error here: the run itself already succeeded, so failing the whole command over a
// provenance field would be worse than a document that honestly records no hash (the field's doc
// comment allows exactly that).
//
// It hashes the ENTRY file only. A hierarchical design's sub-sheets and a project's sidecars are not
// covered, so a matching hash means "the same entry file", not "the same design" — enough to catch a
// document read against a since-edited file, and not claimed to be more.
func hashSource(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
