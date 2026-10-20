package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/feedback"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/version"
	"github.com/panyam/agni/service"
)

// feedbackRepo is where a trial's prefilled issue goes.
const feedbackRepo = "panyam/agni"

// trialCmd walks what a reviewer does with a design and reports how it went as a feedback.Report,
// which holds nothing from the design (agni issue 1057). It is how a release is tried on a machine
// whose boards we never see: the compose bundle runs it as its `trial` service.
func trialCmd() *cobra.Command {
	var format, outPath string
	cmd := &cobra.Command{
		Use:   "trial <design>",
		Short: "Try agni on a design and write a report that holds nothing from it",
		Long: "trial reads the design, runs every rule, scores the project's checklists, answers its query\n" +
			"sets, traces a signal and saves the check report, timing each step. It writes the result as a\n" +
			"report of counts, timings and agni's own rule names, with no net name, reference designator,\n" +
			"part number, design name or path in it, and prints a link that opens it as a GitHub issue for\n" +
			"you to read and submit.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "md" && format != "json" {
				return fmt.Errorf("--format %s: want md or json", format)
			}
			uri, err := cliArgURI(args[0])
			if err != nil {
				return err
			}
			rep, err := runTrial(cmd.Context(), uri)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if outPath != "" && outPath != "-" {
				f, err := os.Create(outPath) // #nosec G304 -- writes the report file the user named
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
				defer fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s\n", absForNote(outPath))
			}
			if err := writeTrial(out, format, rep); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\nRead the report, then send it: %s\n", rep.IssueURL(feedbackRepo))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "md (for a person to read) | json")
	outFileFlag(cmd, &outPath)
	return cmd
}

func writeTrial(w io.Writer, format string, rep feedback.Report) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	_, err := io.WriteString(w, rep.Markdown())
	return err
}

// runTrial walks the design through the in-process services, the ones `agni serve` answers with,
// and builds the report from their answers. A step that fails is recorded and the walk goes on, so
// one broken step still reports the rest.
func runTrial(ctx context.Context, uri string) (feedback.Report, error) {
	rep := feedback.Report{Version: version.Version(), Platform: platform()}
	catalog, byName, err := composeReviewInputs("")
	if err != nil {
		return rep, err
	}
	shipped := service.ShippedRules()
	ll := &localLoader{loader: newLoader()}
	design := service.NewDesignService(ll, nil, render.Style{}, cliProjects())
	checks := service.NewCheckService(ll, catalog, nil, "", nil, cliProjects())
	reviews := service.NewReviewService(ll, nil, catalog, byName, nil, service.ReviewEnv{ProducerVersion: version.Version()}, "", cliProjects())
	queries := service.NewQueryService(ll, nil, cliProjects())
	snaps := service.NewSnapshotService(ll, cliProjects(), nil, "")
	// The design is ingested in the read step, and every later step asks about that snapshot.
	var snap string
	defer func() {
		if snap != "" {
			_, _ = snaps.DeleteSnapshot(context.WithoutCancel(ctx), &webapi.DeleteSnapshotRequest{Name: snap})
		}
	}()

	step := func(s feedback.Step, run func() (feedback.Status, error)) {
		start := time.Now()
		status, err := run()
		if err != nil {
			status = feedback.StatusFailed
		}
		rep.Steps = append(rep.Steps, feedback.StepResult{Step: s, Status: status, Duration: time.Since(start), Error: feedback.ClassifyError(err)})
	}

	step(feedback.StepRead, func() (feedback.Status, error) {
		s, err := snaps.Ingest(ctx, &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri}})
		if err != nil {
			return "", err
		}
		snap = s.GetName()
		d, err := design.GetDesign(ctx, &webapi.GetDesignRequest{Snapshot: snap})
		if err != nil {
			return "", err
		}
		rep.Shape = feedback.Shape{
			Format:           d.GetSourceFormat(),
			Components:       int(d.GetComponentCount()),
			Nets:             int(d.GetNetCount()),
			Sheets:           len(d.GetSheets()),
			UnexpandedBlocks: len(d.GetUnexpandedHierarchy()),
		}
		for _, s := range d.GetSheets() {
			rep.Shape.HasBoard = rep.Shape.HasBoard || s.GetId() == "board"
		}
		return feedback.StatusOK, nil
	})

	var run *webapi.CheckDesignResponse
	step(feedback.StepCheck, func() (feedback.Status, error) {
		var err error
		if run, err = checks.CheckDesign(ctx, &webapi.CheckDesignRequest{Snapshot: snap}); err != nil {
			return "", err
		}
		tallyRules(&rep, run, catalog, shipped)
		return feedback.StatusOK, nil
	})

	step(feedback.StepReview, func() (feedback.Status, error) {
		lists, err := reviews.ListChecklists(ctx, &webapi.ListChecklistsRequest{Snapshot: snap})
		if err != nil {
			return "", err
		}
		if len(lists.GetChecklists()) == 0 {
			return feedback.StatusSkipped, nil
		}
		for _, c := range lists.GetChecklists() {
			rv, err := reviews.CreateReview(ctx, &webapi.CreateReviewRequest{Snapshot: snap, Manifest: c.GetManifest(), ValidateOnly: true})
			if err != nil {
				return "", err
			}
			rep.Review.Checklists++
			var outcomes []review.Outcome
			for _, a := range rv.GetResults().GetAreas() {
				for _, it := range a.GetItems() {
					outcomes = append(outcomes, review.Outcome(it.GetOutcome()))
				}
			}
			t := review.TallyOf(outcomes...)
			rep.Review.Items += t.Total
			rep.Review.Answered += t.Answered()
			rep.Review.Pass += t.Pass
			rep.Review.Fail += t.Fail
		}
		return feedback.StatusOK, nil
	})

	step(feedback.StepQuerySets, func() (feedback.Status, error) {
		sets, err := queries.ListQuerySets(ctx, &webapi.ListQuerySetsRequest{Snapshot: snap})
		if err != nil {
			return "", err
		}
		if len(sets.GetQuerySets()) == 0 {
			return feedback.StatusSkipped, nil
		}
		for _, s := range sets.GetQuerySets() {
			resp, err := queries.RunQueries(ctx, &webapi.RunQueriesRequest{Snapshot: snap, Set: s.GetSet(), OmitLocations: true})
			if err != nil {
				return "", err
			}
			rep.Queries.Sets++
			for _, r := range resp.GetResults() {
				rep.Queries.Queries++
				if r.GetError() != "" {
					rep.Queries.Failed++
				} else {
					rep.Queries.Answered++
				}
			}
		}
		return feedback.StatusOK, nil
	})

	step(feedback.StepTrace, func() (feedback.Status, error) {
		// Through the first resistor the design has, end to end, which any board with a resistor can
		// answer. Its name is used for the request and kept nowhere.
		ans, err := queries.RunQuery(ctx, &webapi.RunQueryRequest{Snapshot: snap, Query: `component.class(?r, "resistor") => ?r`, Limit: 1, OmitLocations: true})
		if err != nil {
			return "", err
		}
		if len(ans.GetRows()) == 0 || len(ans.GetRows()[0].GetCells()) == 0 {
			return feedback.StatusSkipped, nil
		}
		ref := ans.GetRows()[0].GetCells()[0]
		if _, err := design.TraceDesign(ctx, &webapi.TraceDesignRequest{
			Snapshot: snap, From: &webapi.TraceEndpoint{RefDes: ref, Pin: "1"}, To: &webapi.TraceEndpoint{RefDes: ref, Pin: "2"},
		}); err != nil {
			return "", err
		}
		return feedback.StatusOK, nil
	})

	step(feedback.StepSaveReport, func() (feedback.Status, error) {
		if run == nil {
			return feedback.StatusSkipped, nil
		}
		_, err := checks.RenderCheckReport(ctx, &webapi.RenderCheckReportRequest{Snapshot: snap, Run: run, Format: webapi.CheckReportFormat_CHECK_REPORT_FORMAT_HTML})
		if err != nil {
			return "", err
		}
		return feedback.StatusOK, nil
	})
	return rep, nil
}

// tallyRules counts the run per rule. A shipped rule is named; any other rule, which a project or
// its overlay composed, is counted in ProjectRules without its name.
func tallyRules(rep *feedback.Report, run *webapi.CheckDesignResponse, catalog *check.Catalog, shipped map[string]bool) {
	byRule := map[string]*feedback.RuleTally{}
	project := map[string]bool{}
	get := func(name string) *feedback.RuleTally {
		if t, ok := byRule[name]; ok {
			return t
		}
		sr, ok := feedback.Shipped(name, shipped)
		if !ok {
			return nil
		}
		t := &feedback.RuleTally{Rule: sr}
		if r := catalog.Lookup(name); r != nil {
			t.Severity = r.Severity
		}
		byRule[name] = t
		return t
	}
	for _, f := range run.GetFindings() {
		t := get(f.GetRule())
		if t == nil {
			project[f.GetRule()] = true
			rep.ProjectRules.Findings++
			continue
		}
		t.Findings++
		if f.GetInconclusive() {
			t.Inconclusive++
		}
	}
	for _, v := range run.GetVerdicts() {
		t := get(v.GetRule())
		if t == nil {
			continue
		}
		switch service.OutcomeWord(v.GetOutcome()) {
		case "pass":
			t.Pass++
		case "fail":
			t.Fail++
		}
	}
	rep.ProjectRules.Rules = len(project)
	for _, t := range byRule {
		if t.Findings+t.Pass+t.Fail > 0 {
			rep.Rules = append(rep.Rules, *t)
		}
	}
}

// platform names the OS and architecture, and whether the process runs in a container, which the
// compose bundle's images do.
func platform() string {
	p := runtime.GOOS + "/" + runtime.GOARCH
	if _, err := os.Stat("/.dockerenv"); err == nil {
		p += ", container"
	} else if _, err := os.Stat("/run/.containerenv"); err == nil {
		p += ", container"
	}
	return p
}
