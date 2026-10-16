package main

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// The DEFAULT text output states what the run looked at. A findings list cannot, because
// twenty-nine rules finding nothing and twenty-nine rules that each examined the wrong thing print
// an identical line, and "N rule(s) run" is a proxy for coverage that is not coverage. The
// considered set rides on the same response, so stating it costs nothing, and it is stated by
// default because coverage must not be something a reader has to know to ask for.
func TestDefaultCheckStatesItsCoverage(t *testing.T) {
	out := runCheck(t, "testdata/conformance/showcase.fires.kicad_sch")

	m := regexp.MustCompile(`(\d+) subject\(s\) considered by (\d+) rule\(s\)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("default output states no coverage:\n%s", out)
	}
	if n, _ := strconv.Atoi(m[1]); n == 0 {
		t.Error("a coverage line claiming zero subjects should not be printed at all")
	}
	if !strings.Contains(out, "--verdicts") {
		t.Error("the coverage line must point at the flag that shows the detail")
	}

	// The rows themselves stay behind the flag, since they are several times the volume of the findings
	// list here and far more on a real board, so the default states the claim and the flag shows the
	// evidence.
	if len(strings.Split(runCheck(t, "--verdicts", "testdata/conformance/showcase.fires.kicad_sch"), "\n")) <=
		len(strings.Split(out, "\n")) {
		t.Error("--verdicts should be the fuller output; the default must not already be dumping rows")
	}
}

// The coverage claim counts only rules that STATED a considered set and names the rest, so a reader
// can tell a covered rule from a quiet one. The count of rules it cannot vouch for is over rules
// that REPORTED SOMETHING without stating a set, never over every rule that ran, because most rules
// that run on a given board simply have no subject in scope, and calling those "violations only"
// would invent a coverage hole out of rules that correctly had nothing to say.
func TestCoverageOnlyBlamesRulesThatActuallyReported(t *testing.T) {
	verdict := func(rule string, o checkspb.Outcome) *checkspb.Verdict {
		return &checkspb.Verdict{Rule: rule, Outcome: o}
	}
	for _, tc := range []struct {
		name     string
		fs       []check.Finding
		vs       []*checkspb.Verdict
		want     []string
		unwanted []string
	}{
		{
			name: "a rule that reported without stating a set is named",
			fs:   []check.Finding{{Rule: "legacy"}, {Rule: "legacy"}, {Rule: "stated"}},
			vs:   []*checkspb.Verdict{verdict("stated", checkspb.Outcome_OUTCOME_PASS)},
			want: []string{"1 subject(s) considered by 1 rule(s)", "1 rule(s) reported violations without stating"},
		},
		{
			name:     "a quiet rule is not blamed, because it reported nothing at all",
			fs:       []check.Finding{{Rule: "stated"}},
			vs:       []*checkspb.Verdict{verdict("stated", checkspb.Outcome_OUTCOME_FAIL)},
			want:     []string{"1 subject(s) considered by 1 rule(s)"},
			unwanted: []string{"violations without stating"},
		},
		{
			name: "not-considered is counted apart from the judged outcomes",
			vs: []*checkspb.Verdict{
				verdict("r", checkspb.Outcome_OUTCOME_PASS),
				verdict("r", checkspb.Outcome_OUTCOME_NOT_CONSIDERED),
			},
			want: []string{"1 subject(s) considered by 1 rule(s), 1 not considered"},
		},
		{
			name:     "no verdicts at all claims no coverage rather than zero coverage",
			fs:       []check.Finding{{Rule: "legacy"}},
			unwanted: []string{"considered", "--verdicts"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			// Through the verdict report, as the command counts it (agni issue 1045).
			resp := &webapi.CheckDesignResponse{Verdicts: tc.vs}
			for _, f := range tc.fs {
				resp.Findings = append(resp.Findings, &checkspb.Finding{Rule: f.Rule, Subject: &checkspb.Subject{Kind: "net", Ref: "N"}})
			}
			writeCoverage(&b, coverageOf(resp, nil))
			for _, w := range tc.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("want %q in:\n%s", w, b.String())
				}
			}
			for _, u := range tc.unwanted {
				if strings.Contains(b.String(), u) {
					t.Errorf("did not want %q in:\n%s", u, b.String())
				}
			}
		})
	}
}

// A stored results document has no field for a considered set, so replaying one must state no
// coverage rather than inventing a number from the findings it does carry.
func TestResultsReplayClaimsNoCoverage(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/r.json"
	runCheck(t, "--results-out", path, "testdata/conformance/showcase.fires.kicad_sch")

	cmd := resultsCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agni results: %v", err)
	}
	if strings.Contains(out.String(), "subject(s) considered") {
		t.Errorf("a results document cannot carry a considered set, so it must claim none:\n%s", out.String())
	}
}

// TestCoverageLineMatchesTheVerdictsFooter: the default output's coverage line and --verdicts' footer
// count one run one way, since both come from the verdict report (agni issue 1045). They used to be
// two counts, one of which ignored the catalog's StatesConsideredSet.
func TestCoverageLineMatchesTheVerdictsFooter(t *testing.T) {
	const design = "testdata/conformance/showcase.fires.kicad_sch"
	line := regexp.MustCompile(`(\d+) subject\(s\) considered by (\d+) rule\(s\)(?:, (\d+) not considered)?`).FindStringSubmatch(runCheck(t, design))
	foot := regexp.MustCompile(`(\d+) verdicts across (\d+) rule\(s\)(?:.*?, (\d+) not-considered)?`).FindStringSubmatch(runCheck(t, "--verdicts", design))
	if line == nil || foot == nil {
		t.Fatalf("coverage line %v, footer %v; want both", line, foot)
	}
	n := func(s string) int { v, _ := strconv.Atoi(s); return v }
	if n(line[2]) != n(foot[2]) || n(line[1])+n(line[3]) != n(foot[1]) || n(line[3]) != n(foot[3]) {
		t.Errorf("coverage line %q disagrees with the footer %q", line[0], foot[0])
	}
	if n(foot[1]) < 20 {
		t.Fatalf("the fixture stated %s verdicts, too few to compare", foot[1])
	}
}

// TestSpecLibAnswersThroughTheService: `query --speclib` is answered by the QueryService's projection
// rather than one the CLI built, so its json carries what a design query's does, a kind per column
// and the work it cost (agni issue 1045).
func TestSpecLibAnswersThroughTheService(t *testing.T) {
	out := runCLI(t, queryCmd(), "--speclib", "--params", filepath.Join("..", "..", "examples", "tutorial-project", "params"),
		"param.max(?m, ?s, ?v) => ?m, ?s, ?v", "--format", "json")
	var resp struct {
		Columns     []string `json:"columns"`
		ColumnKinds []string `json:"columnKinds"`
		Rows        []any    `json:"rows"`
		Work        string   `json:"work"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Rows) < 3 {
		t.Fatalf("the corpus answered %d rows, too few to say anything", len(resp.Rows))
	}
	if len(resp.ColumnKinds) != len(resp.Columns) || resp.Work == "" || resp.Work == "0" {
		t.Errorf("columnKinds %v for columns %v, work %q; want the service's projection", resp.ColumnKinds, resp.Columns, resp.Work)
	}
}
