package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const combinedReportFixture = "testdata/combined-report"

// `check --format html --query-sets` writes one file carrying the run's checks and every query set
// the design's project keeps, the set's rows answered over the same read (agni issue 1190).
func TestCheckHTMLCarriesTheProjectsQuerySets(t *testing.T) {
	page := runCheck(t, "--format", "html", "--query-sets", combinedReportFixture+"/designs/demo-board")
	rules, queries := strings.Index(page, "single-pin-net"), strings.Index(page, `<section class="query-set">`)
	if rules < 0 || queries < rules {
		t.Fatalf("want the checks and then a Queries section, rule at %d and queries at %d", rules, queries)
	}
	section := page[queries:]
	for _, want := range []string{"Queries: Audit demo", "<h2>Pins and their nets</h2>", "4 result(s)", "<td class=\"subject\">OUT</td>"} {
		if !strings.Contains(section, want) {
			t.Errorf("the Queries section lacks %q", want)
		}
	}
	if plain := runCheck(t, "--format", "html", combinedReportFixture+"/designs/demo-board"); strings.Contains(plain, "query-set") {
		t.Error("a check report asked for no query sets carries a Queries section")
	}
}

// The rows come from the set the project keeps now, so editing its YAML changes the report.
func TestTheQuerySectionFollowsTheSetFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(combinedReportFixture)); err != nil {
		t.Fatal(err)
	}
	set := filepath.Join(dir, "queries", "audit-demo.yaml")
	b, err := os.ReadFile(set)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), "pin.net(?part, ?pin, ?net) =>", `pin.net(?part, ?pin, ?net), ?net != "SIG" =>`, 1)
	if edited == string(b) {
		t.Fatal("the fixture's query changed shape; update this test")
	}
	if err := os.WriteFile(set, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}
	page := runCheck(t, "--format", "html", "--query-sets", filepath.Join(dir, "designs", "demo-board"))
	section := page[strings.Index(page, `<section class="query-set">`):]
	if !strings.Contains(section, "2 result(s)") || strings.Contains(section, ">SIG<") {
		t.Errorf("the Queries section does not follow the edited set:\n%s", section)
	}
}

func TestQuerySetsNeedTheHTMLReport(t *testing.T) {
	for _, format := range []string{"csv", "json", "text"} {
		cmd := checkCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--verdicts", "--format", format, "--query-sets", combinedReportFixture + "/designs/demo-board"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--format html") {
			t.Errorf("--format %s --query-sets: got %v, want a refusal naming --format html", format, err)
		}
	}
}
