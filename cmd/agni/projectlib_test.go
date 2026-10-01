package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
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
