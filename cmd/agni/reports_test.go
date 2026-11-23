package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
)

// reportKeys is a report's findings as findingKeys writes a check's, sorted.
func reportKeys(r *webapi.Report) []string {
	var keys []string
	for _, f := range r.GetResults().GetFindings() {
		keys = append(keys, f.GetRule()+"|"+f.GetSubject().GetKind()+":"+f.GetSubject().GetRef()+"|"+f.GetSeverity())
	}
	slices.Sort(keys)
	return keys
}

// Two reports on one design against two packs are both kept, each with its own findings. Replacing
// the design's files marks both as run on earlier files, changing one pack marks only its report,
// and all of it is there after a restart (agni issue 1704). The report under the project's pack holds
// the findings agni check gives on the folder.
func TestReportsKeepEachRunAndSayWhatChanged(t *testing.T) {
	ctx := context.Background()
	store, uploads := t.TempDir(), t.TempDir()
	srv := designServer(t, store, uploads)
	project := tutorialProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	packs := webapiconnect.NewPackServiceClient(http.DefaultClient, srv.URL)
	strict := func(pattern string) *webapi.Pack {
		return &webapi.Pack{Name: "packs/strict", Config: &webapi.AnalysisConfig{Conventions: &configpb.NamingConvention{Name: "strict",
			Rules: []*configpb.NamingRule{{Name: "no-pmic", Severity: "warning", Why: "a test convention", Allow: []string{pattern}}}}}}
	}
	if _, err := packs.CreatePack(ctx, connect.NewRequest(&webapi.CreatePackRequest{PackId: "strict", Pack: strict("^CAN")})); err != nil {
		t.Fatal(err)
	}
	design := filepath.Join(project, "boards", "gw")
	if out, err := runDesigns(t, srv.URL, "push", design, "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}

	out, err := runReports(t, srv.URL, "create", "designs/gw", "--pack", "gateway", "--id", "under-gateway")
	if err != nil || !strings.Contains(out, "designs/gw/reports/under-gateway\tpacks/gateway\t") || !strings.HasSuffix(strings.TrimSpace(out), "current") {
		t.Fatalf("reports create --pack gateway = %q, %v; want the report named, current", out, err)
	}
	if out, err := runReports(t, srv.URL, "create", "gw", "--pack", "strict", "--id", "under-strict"); err != nil {
		t.Fatalf("reports create --pack strict: %v\n%s", err, out)
	}

	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	list := func(designs webapiconnect.DesignServiceClient) map[string]*webapi.Report {
		t.Helper()
		resp, err := designs.ListReports(ctx, connect.NewRequest(&webapi.ListReportsRequest{Parent: "designs/gw", View: webapi.ReportView_REPORT_VIEW_FULL}))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]*webapi.Report{}
		for _, r := range resp.Msg.GetReports() {
			out[strings.TrimPrefix(r.GetName(), "designs/gw/reports/")] = r
		}
		return out
	}
	got := list(designs)
	gw, st := got["under-gateway"], got["under-strict"]
	if len(got) != 2 || gw == nil || st == nil {
		t.Fatalf("reports = %v; want under-gateway and under-strict", got)
	}
	if local := findingKeys(t, "check", design); !slices.Equal(reportKeys(gw), local) {
		t.Errorf("the report under packs/gateway = %d findings, check on the folder = %d; want the same set\nonly in the report: %v\nonly on the folder: %v",
			len(reportKeys(gw)), len(local), minus(reportKeys(gw), local), minus(local, reportKeys(gw)))
	}
	if !hasRulePrefix(reportKeys(st), "strict/no-pmic") || hasRulePrefix(reportKeys(st), "gateway/") || hasRulePrefix(reportKeys(gw), "strict/") {
		t.Errorf("each report holds its own pack's convention findings; got gateway %v, strict %v", reportKeys(gw), reportKeys(st))
	}
	for _, r := range got {
		if r.GetDesignChanged() || len(r.GetPacksChanged()) > 0 || r.GetDesignHash() == "" || len(r.GetPackHashes()) != 1 {
			t.Errorf("%s = changed %v %v, hashes %q %v; want current, with the hashes it ran on", r.GetName(), r.GetDesignChanged(), r.GetPacksChanged(), r.GetDesignHash(), r.GetPackHashes())
		}
	}

	if _, err := packs.UpdatePack(ctx, connect.NewRequest(&webapi.UpdatePackRequest{Pack: strict("^(CAN|PMIC)"), UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"config"}}})); err != nil {
		t.Fatal(err)
	}
	got = list(designs)
	if !slices.Equal(got["under-strict"].GetPacksChanged(), []string{"packs/strict"}) || len(got["under-gateway"].GetPacksChanged()) != 0 {
		t.Errorf("after packs/strict changed, packs_changed = %v and %v; want packs/strict on its report alone",
			got["under-strict"].GetPacksChanged(), got["under-gateway"].GetPacksChanged())
	}

	if out, err := runDesigns(t, srv.URL, "put", "gw", "netlist", gatewayRevB); err != nil {
		t.Fatalf("designs put: %v\n%s", err, out)
	}
	restarted := designServer(t, store, uploads)
	again := list(webapiconnect.NewDesignServiceClient(http.DefaultClient, restarted.URL))
	if len(again) != 2 {
		t.Fatalf("after a restart, reports = %v; want both", again)
	}
	for id, r := range again {
		if !r.GetDesignChanged() {
			t.Errorf("after the netlist was replaced, %s says it ran on the current files; want design_changed", id)
		}
		if len(r.GetResults().GetFindings()) == 0 {
			t.Errorf("after a restart, %s has no findings; want the run as kept", id)
		}
	}
	if out, err := runReports(t, restarted.URL, "list", "gw"); err != nil || strings.Count(out, "run on earlier design files") != 2 {
		t.Errorf("reports list after the change = %q, %v; want both marked as run on earlier design files", out, err)
	}
}

// A report naming a project applies the project's default packs first, and a pack the request also
// names applies once (agni issue 1704).
func TestAReportAppliesItsProjectsDefaultPacks(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	project := tutorialProject(t)
	if out, err := runPacks(t, srv.URL, "push", project); err != nil {
		t.Fatalf("packs push: %v\n%s", err, out)
	}
	if out, err := runDesigns(t, srv.URL, "push", filepath.Join(project, "boards", "gw"), "--id", "gw"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}
	projects := webapiconnect.NewProjectServiceClient(http.DefaultClient, srv.URL)
	if _, err := projects.CreateProject(ctx, connect.NewRequest(&webapi.CreateProjectRequest{ProjectId: "lab", Project: &webapi.Project{Designs: []string{"designs/gw"}}})); err != nil {
		t.Fatal(err)
	}
	if out, err := runProjects(t, srv.URL, "packs", "lab", "gateway"); err != nil || !strings.Contains(out, "projects/lab\tpacks/gateway") {
		t.Fatalf("projects packs = %q, %v; want packs/gateway listed", out, err)
	}
	if out, err := runProjects(t, srv.URL, "get", "lab"); err != nil || !strings.Contains(out, "  design\tdesigns/gw\n") || !strings.Contains(out, "  pack\tpacks/gateway\n") {
		t.Errorf("projects get = %q, %v; want its design and its default pack", out, err)
	}
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	resp, err := designs.CreateReport(ctx, connect.NewRequest(&webapi.CreateReportRequest{Parent: "designs/gw", Project: "projects/lab", Report: &webapi.Report{Packs: []string{"packs/gateway"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if r := resp.Msg; !slices.Equal(r.GetPacks(), []string{"packs/gateway"}) || r.GetProject() != "projects/lab" || !hasRulePrefix(reportKeys(r), "gateway/") {
		t.Errorf("report = packs %v, project %q, findings %v; want packs/gateway once, from projects/lab, its convention applied", r.GetPacks(), r.GetProject(), reportKeys(r))
	}
	for name, req := range map[string]*webapi.CreateReportRequest{
		"a mounted design": {Parent: "designs/none"},
		"a missing pack":   {Parent: "designs/gw", Report: &webapi.Report{Packs: []string{"packs/none"}}},
		"a bad pack name":  {Parent: "designs/gw", Report: &webapi.Report{Packs: []string{"projects/lab"}}},
	} {
		if _, err := designs.CreateReport(ctx, connect.NewRequest(req)); err == nil {
			t.Errorf("CreateReport with %s succeeded; want it refused", name)
		}
	}
	if _, err := designs.ListReports(ctx, connect.NewRequest(&webapi.ListReportsRequest{Parent: "designs/gw", Filter: `project = "projects/lab"`})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ListReports with a filter = %v; want INVALID_ARGUMENT, as none is implemented", err)
	}
	if _, err := designs.DeleteDesign(ctx, connect.NewRequest(&webapi.DeleteDesignRequest{Name: "designs/gw"})); err != nil {
		t.Fatal(err)
	}
	if left, err := designs.ListReports(ctx, connect.NewRequest(&webapi.ListReportsRequest{Parent: "designs/gw"})); err != nil || len(left.Msg.GetReports()) != 0 {
		t.Errorf("after DeleteDesign, reports = %v, %v; want none", left, err)
	}
}

// A listing leaves each report's run out and says how many findings it holds, and a get, or a
// listing that asks for the full view, sends the run (agni issue 1753).
func TestAReportListingLeavesTheRunOut(t *testing.T) {
	ctx := context.Background()
	srv := designServer(t, t.TempDir(), t.TempDir())
	if out, err := runDesigns(t, srv.URL, "push", filepath.Join(tutorialProject(t), "boards", "gw"), "--id", "gw", "--no-pack"); err != nil {
		t.Fatalf("designs push: %v\n%s", err, out)
	}
	if out, err := runReports(t, srv.URL, "create", "designs/gw", "--id", "r"); err != nil {
		t.Fatalf("reports create: %v\n%s", err, out)
	}
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	full, err := designs.GetReport(ctx, connect.NewRequest(&webapi.GetReportRequest{Name: "designs/gw/reports/r"}))
	if err != nil {
		t.Fatal(err)
	}
	n := len(full.Msg.GetResults().GetFindings())
	if n == 0 || int(full.Msg.GetFindingCount()) != n || len(full.Msg.GetResults().GetVerdicts()) == 0 {
		t.Fatalf("GetReport = %d findings, count %d, %d verdicts; want the whole run, counted", n, full.Msg.GetFindingCount(), len(full.Msg.GetResults().GetVerdicts()))
	}
	list, err := designs.ListReports(ctx, connect.NewRequest(&webapi.ListReportsRequest{Parent: "designs/gw"}))
	if err != nil || len(list.Msg.GetReports()) != 1 {
		t.Fatalf("ListReports = %v, %v; want one report", list, err)
	}
	if r := list.Msg.GetReports()[0]; r.GetResults() != nil || int(r.GetFindingCount()) != n {
		t.Errorf("a listed report = results %v, count %d; want no results and the count %d", r.GetResults() != nil, r.GetFindingCount(), n)
	}
	basic, err := designs.GetReport(ctx, connect.NewRequest(&webapi.GetReportRequest{Name: "designs/gw/reports/r", View: webapi.ReportView_REPORT_VIEW_BASIC}))
	if err != nil || basic.Msg.GetResults() != nil || int(basic.Msg.GetFindingCount()) != n {
		t.Errorf("GetReport BASIC = results %v, count %d, %v; want no results and the count", basic.Msg.GetResults() != nil, basic.Msg.GetFindingCount(), err)
	}
	if out, err := runReports(t, srv.URL, "list", "gw"); err != nil || !strings.Contains(out, fmt.Sprintf("\t%d findings\t", n)) {
		t.Errorf("reports list = %q, %v; want the count %d", out, err, n)
	}
}

func runReports(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	return runSub(t, reportsCmd(), url, args...)
}

func runProjects(t *testing.T, url string, args ...string) (string, error) {
	t.Helper()
	return runSub(t, projectsCmd(), url, args...)
}

// runSub runs one remote subcommand group with args and --remote url, returning what it printed.
func runSub(t *testing.T, cmd *cobra.Command, url string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--remote", url))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}
