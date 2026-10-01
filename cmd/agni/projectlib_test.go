package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

const gatewayDesign = "../../examples/tutorial-project/designs/gateway"

// TestProjectLibraryAnswersInsideItsProject covers agni issue 773 end to end. The tutorial project's
// lib/house.dl defines house.pmic_rail and house.pmic_probe_point, and a query on a design in that
// project calls them as it calls any relation. Of the PMIC's three rails only PMIC_MAIN_12V0 has no
// test point.
func TestProjectLibraryAnswersInsideItsProject(t *testing.T) {
	out := runCLI(t, queryCmd(), gatewayDesign, "house.pmic_rail(?n), not house.pmic_probe_point(?n) => ?n")
	if !strings.Contains(out, "PMIC_MAIN_12V0") || strings.Contains(out, "PMIC_CORE_3V3") || !strings.Contains(out, "1 result") {
		t.Errorf("want PMIC_MAIN_12V0 alone, the one PMIC rail with no test point:\n%s", out)
	}
}

// TestProjectLibraryIsUnknownOutsideItsProject is the other half: a design in no project gets the
// shipped vocabulary alone, so one team's members never leak into another's queries.
func TestProjectLibraryIsUnknownOutsideItsProject(t *testing.T) {
	cmd := queryCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"../../examples/common/designs/probe-coverage.edn", "house.pmic_rail(?n) => ?n"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `unknown module "house"`) {
		t.Errorf("err = %v, want house unknown outside the project that defines it", err)
	}
}

// TestRelationsListsAProjectMember checks drill-down with --design: the project's member lists with
// its signature, its definition, and its optional page from lib/docs.
func TestRelationsListsAProjectMember(t *testing.T) {
	out := runCLI(t, queryCmd(), "--relations", "house.pmic_rail", "--design", gatewayDesign)
	for _, want := range []string{"house.pmic_rail(n: net)", "derived, defined in module house", `str.prefix(?n, "PMIC_")`, "### What it is"} {
		if !strings.Contains(out, want) {
			t.Errorf("--relations house.pmic_rail --design lacks %q:\n%s", want, out)
		}
	}
	shipped := runCLI(t, queryCmd(), "--relations", "net")
	if strings.Contains(shipped, "house.pmic_rail") {
		t.Errorf("--relations without --design listed a project member:\n%s", shipped)
	}
}

// TestABrokenProjectModuleNamesItsFile builds a project whose library reads a relation nothing
// defines. The read fails rather than answering without the library, and the error locates the file.
func TestABrokenProjectModuleNamesItsFile(t *testing.T) {
	projectWithLib(t, map[string]string{
		"house.dl": "broken(?n: net) :- net.no_such_relation(?n);\n",
	})
	err := runQueryErr(t, "designs/board", "entity(?n, \"net\") => ?n")
	// The module is named by its path, which is its file's name, and the directory it was read from
	// by agni, so together they locate lib/house.dl (panyam/jaala#30 would name the file outright).
	if err == nil || !strings.Contains(err.Error(), "no_such_relation") || !strings.Contains(err.Error(), `module "house"`) || !strings.Contains(err.Error(), "/lib") {
		t.Errorf("err = %v, want the broken module refused, naming its module and library directory", err)
	}
}

// TestAProjectMemberCannotReplaceAShippedOne refuses a project module defining a path the shipped
// library already does, naming the file and the path.
func TestAProjectMemberCannotReplaceAShippedOne(t *testing.T) {
	projectWithLib(t, map[string]string{
		"net.dl": "has_test_point(?n: net) :- entity(?n, \"net\");\n",
	})
	err := runQueryErr(t, "designs/board", "entity(?n, \"net\") => ?n")
	if err == nil || !strings.Contains(err.Error(), "net.dl defines net.has_test_point, which agni already defines") {
		t.Errorf("err = %v, want the collision refused with the file and the path named", err)
	}
}

// projectWithLib writes a minimal project holding one design (a copy of the probe-coverage fixture)
// and the given lib/ files, and runs the rest of the test from inside it.
func projectWithLib(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	board, err := os.ReadFile("../../examples/common/designs/probe-coverage.edn")
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("project.yaml", "name: scratch\n")
	write("designs/board/design.yaml", "name: board\nentry: board.edn\n")
	write("designs/board/board.edn", string(board))
	for name, body := range files {
		write(filepath.Join("lib", name), body)
	}
	t.Chdir(dir)
	return dir
}

func runQueryErr(t *testing.T, args ...string) error {
	t.Helper()
	cmd := queryCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

// TestLibFlagAnswersAsTheInlineRequestDoes is the C32 parity test for agni issue 788. `--lib` reads a
// directory and sends its modules inline, so the CLI and a client sending the same modules to the
// service must get the same rows, on a design that belongs to no project and so has no library of its
// own. Without the flag the member does not exist.
func TestLibFlagAnswersAsTheInlineRequestDoes(t *testing.T) {
	const (
		lib    = "../../examples/tutorial-project/lib"
		design = "../../examples/common/designs/probe-coverage.edn"
		q      = "net.has_test_point(?n), not house.pmic_probe_point(?n) => ?n"
	)
	if err := runQueryErr(t, design, q); err == nil || !strings.Contains(err.Error(), `unknown module "house"`) {
		t.Fatalf("without --lib: err = %v, want house unknown", err)
	}
	cli := runCLI(t, queryCmd(), design, q, "--lib", lib, "--format", "csv")

	cfg := &webapi.AnalysisConfig{}
	if err := addLibraries(cfg, []string{lib}); err != nil {
		t.Fatal(err)
	}
	uri, err := cliArgURI(design)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewQueryService(&localLoader{loader: newLoader()}, nil, cliProjects())
	resp, err := svc.RunQuery(context.Background(), &webapi.RunQueryRequest{Uri: uri, Query: q, Overlay: &webapi.OverlayConfig{Config: cfg}})
	if err != nil {
		t.Fatalf("RunQuery with the modules inline: %v", err)
	}
	var fromService []string
	for _, r := range resp.GetRows() {
		fromService = append(fromService, r.GetCells()[0])
	}
	var fromCLI []string
	for _, line := range strings.Split(strings.TrimSpace(cli), "\n")[1:] {
		fromCLI = append(fromCLI, strings.Split(line, ",")[0])
	}
	if len(fromCLI) == 0 || strings.Join(fromCLI, " ") != strings.Join(fromService, " ") {
		t.Errorf("--lib answered %v, the inline request %v; want the same non-empty rows", fromCLI, fromService)
	}
}

// TestAChecklistQueryCallsTheProjectLibrary is agni issue 779 end to end, on both surfaces. The
// tutorial project's review.yaml binds P6 to an inline query over its own lib/house.dl, and the item
// fails on PMIC_MAIN_12V0, the one PMIC rail with no test point, whether `agni review` runs it or a
// served CreateReview does (C32). Before this the query could not compile, since a rule from a
// manifest saw only the shipped vocabulary.
func TestAChecklistQueryCallsTheProjectLibrary(t *testing.T) {
	cli := runReview(t, gatewayDesign, "--format", "json")
	if !strings.Contains(cli, `"PMIC rail PMIC_MAIN_12V0 has no test point"`) {
		t.Errorf("agni review: P6 did not report PMIC_MAIN_12V0:\n%.2000s", cli)
	}

	ms := []mounts.Mount{{Name: "tut", Root: "../../examples/tutorial-project"}}
	loader := &osLoader{mounts: ms, loader: newLoader()}
	svc := service.NewReviewService(loader, service.NewMemReviewStore(), check.DefaultCatalog(), nil, nil, service.ReviewEnv{}, "", testProjectResolver(ms))
	u, err := artifact.Parse("mount://tut/review.yaml")
	if err != nil {
		t.Fatal(err)
	}
	man, err := loader.Manifest(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	rv, err := svc.CreateReview(context.Background(), &webapi.CreateReviewRequest{
		DesignUri: "mount://tut/designs/gateway",
		Manifest:  service.ManifestProto(man),
	})
	if err != nil {
		t.Fatalf("CreateReview: %v", err)
	}
	var found bool
	for _, a := range rv.GetResults().GetAreas() {
		for _, it := range a.GetItems() {
			if it.GetId() != "P6" {
				continue
			}
			found = true
			if it.GetOutcome() != "fail" || len(it.GetFindings()) != 1 || it.GetFindings()[0].GetSubject().GetRef() != "PMIC_MAIN_12V0" {
				t.Errorf("served P6 = %s with %d findings, want fail on PMIC_MAIN_12V0", it.GetOutcome(), len(it.GetFindings()))
			}
		}
	}
	if !found {
		t.Error("the served review has no P6 item")
	}
}

// TestReviewLibFlagReachesAChecklistQuery: `agni review --lib` sends a library with the run (agni
// issue 788), so a checklist's inline query on a design that belongs to no project can call it.
// Without the flag the checklist is refused before anything runs, naming the module, rather than
// running with the item undecided.
func TestReviewLibFlagReachesAChecklistQuery(t *testing.T) {
	checklist := filepath.Join(t.TempDir(), "review.yaml")
	if err := os.WriteFile(checklist, []byte(`name: scratch
areas:
  - name: Test access
    items:
      - id: "T1"
        title: every probed net keeps its probe
        query:
          match: 'net.has_test_point(?n), not house.pmic_probe_point(?n) => ?n'
          subject: n
          kind: net
          message: '{n} is probed but is no PMIC probe point'
`), 0o644); err != nil {
		t.Fatal(err)
	}
	const design = "../../examples/common/designs/probe-coverage.edn"
	if _, errOut, err := runReviewCapturing(t, design, "--checklist", checklist); err == nil || !strings.Contains(err.Error()+errOut, `"house"`) {
		t.Errorf("without --lib: err = %v, want the checklist refused naming house", err)
	}
	out := runReview(t, design, "--checklist", checklist, "--lib", "../../examples/tutorial-project/lib", "--format", "json")
	for _, n := range []string{"FB", "VIN", "VOUT"} {
		if !strings.Contains(out, n+" is probed but is no PMIC probe point") {
			t.Errorf("with --lib: no finding for %s:\n%.1500s", n, out)
		}
	}
}
