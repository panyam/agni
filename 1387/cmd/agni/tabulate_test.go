package main

import (
	"encoding/csv"
	"strings"
	"testing"

	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const tabulateDesign = "testdata/conformance/showcase.fires.kicad_sch"

// tabulate runs `agni tabulate -` over req and returns its tables by name.
func tabulate(t *testing.T, req *webapi.TabulateRequest) map[string]*webapi.Table {
	t.Helper()
	b, err := protojson.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	cmd := tabulateCmd()
	cmd.SetIn(strings.NewReader(string(b)))
	out := runCLI(t, cmd, "-")
	var resp webapi.TabulateResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("tabulate printed no TabulateResponse: %v\n%.300s", err, out)
	}
	byName := map[string]*webapi.Table{}
	for _, tb := range resp.GetTables() {
		byName[tb.GetName()] = tb
	}
	return byName
}

// sameAsCSV fails unless the csv carries exactly the table's header and cells, the cells escaped as
// the csv encoder escapes them.
func sameAsCSV(t *testing.T, what, csvText string, tb *webapi.Table) {
	t.Helper()
	recs, err := csv.NewReader(strings.NewReader(csvText)).ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	want := [][]string{{}}
	for _, c := range tb.GetColumns() {
		want[0] = append(want[0], c.GetName())
	}
	for _, r := range tb.GetRows() {
		row := make([]string, 0, len(r.GetCells()))
		for _, c := range r.GetCells() {
			row = append(row, rpt.SanitizeCell(c))
		}
		want = append(want, row)
	}
	if len(want) < 3 {
		t.Fatalf("%s: the table has %d rows, too few to tell one order from another", what, len(want)-1)
	}
	if got, w := strings.Join(flat(recs), "\x00"), strings.Join(flat(want), "\x00"); got != w {
		t.Errorf("%s: the csv and the tabulated table differ\ncsv:   %.400q\ntable: %.400q", what, got, w)
	}
}

func flat(rows [][]string) []string {
	var out []string
	for _, r := range rows {
		out = append(out, strings.Join(r, "\x01"))
	}
	return out
}

// Every csv the CLI prints for an answer is the table the TableService projects from it, so a
// spreadsheet built by a client and the csv of the same run carry one set of rows (agni issue 862).
func TestCheckCSVIsTheTabulatedTable(t *testing.T) {
	var resp webapi.CheckDesignResponse
	if err := protojson.Unmarshal([]byte(runCheck(t, "--verdicts", "--format", "json", tabulateDesign)), &resp); err != nil {
		t.Fatal(err)
	}
	for _, order := range [][]string{nil, {"rule", "-subject"}} {
		tables := tabulate(t, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: &resp}, OrderBy: order})
		args := []string{"--format", "csv", tabulateDesign}
		if order != nil {
			args = append(args, "--order-by", strings.Join(order, ","))
		}
		sameAsCSV(t, "findings "+strings.Join(order, ","), runCheck(t, args...), tables["findings"])
	}
	verdictOrder := []string{"-outcome", "subjects"}
	tables := tabulate(t, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: proto.Clone(&resp).(*webapi.CheckDesignResponse)}, OrderBy: verdictOrder})
	sameAsCSV(t, "verdicts", runCheck(t, "--verdicts", "--format", "csv", "--order-by", strings.Join(verdictOrder, ","), tabulateDesign), tables["verdicts"])
}

func TestQueryCSVIsTheTabulatedTable(t *testing.T) {
	const q = `component.net(?r, ?n) => ?r, ?n`
	var resp webapi.RunQueryResponse
	if err := protojson.Unmarshal([]byte(runCLI(t, queryCmd(), bindDesign, q, "--format", "json")), &resp); err != nil {
		t.Fatal(err)
	}
	tables := tabulate(t, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Query{Query: &resp}, OrderBy: []string{"r", "-n"}})
	sameAsCSV(t, "query", runCLI(t, queryCmd(), bindDesign, q, "--format", "csv", "--order-by", "r,-n"), tables["query"])
}

func TestOrderByIsRefusedWithoutACSV(t *testing.T) {
	cmd := checkCmd()
	cmd.SetArgs([]string{"--format", "json", "--order-by", "rule", tabulateDesign})
	cmd.SetOut(&strings.Builder{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--order-by") {
		t.Errorf("--order-by with json = %v, want it refused", err)
	}
}

func TestDiffCSVIsTheTabulatedTable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		a, b  string
		flags []string
	}{
		{"unchanged nets", "gateway.edn", "gateway-rev-b.edn", []string{"--include-equal"}},
		{"near renames", "gateway-rev-b.edn", "gateway-rev-c.edn", []string{"--rename-approx"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := tutorialGateway+tc.a, tutorialGateway+tc.b
			var resp webapi.DiffDesignsResponse
			js := runCLI(t, diffCmd(), append([]string{a, b, "--format", "json"}, tc.flags...)...)
			if err := protojson.Unmarshal([]byte(js), &resp); err != nil {
				t.Fatal(err)
			}
			order := []string{"-change_class", "subject"}
			tables := tabulate(t, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Diff{Diff: &resp}, OrderBy: order})
			csvText := runCLI(t, diffCmd(), append([]string{a, b, "--format", "csv", "--order-by", strings.Join(order, ",")}, tc.flags...)...)
			sameAsCSV(t, "diff "+tc.name, csvText, tables["diff"])
		})
	}
}

func TestReviewCSVIsTheTabulatedTable(t *testing.T) {
	var rv webapi.Review
	if err := protojson.Unmarshal([]byte(runCLI(t, reviewCmd(), tutorialGateway, "--format", "json")), &rv); err != nil {
		t.Fatal(err)
	}
	tables := tabulate(t, &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Review{Review: &rv}})
	sameAsCSV(t, "review", runCLI(t, reviewCmd(), tutorialGateway, "--format", "csv"), tables["review"])
	if s := tables["review_summary"]; s == nil || len(s.GetRows()) != 1 {
		t.Errorf("review_summary = %v, want one row", s)
	}
}
