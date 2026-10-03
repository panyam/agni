package service

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

func TestCompareNaturalReadsDigitsAsNumbers(t *testing.T) {
	got := []string{"R10", "r2", "C1", "R1", "U10.12", "U10.3", "R01", ""}
	sort.SliceStable(got, func(i, j int) bool { return compareNatural(got[i], got[j]) < 0 })
	want := "|C1|R01|R1|r2|R10|U10.3|U10.12"
	if strings.Join(got, "|") != want {
		t.Errorf("natural order = %q, want %q", strings.Join(got, "|"), want)
	}
}

func table(cols []*webapi.TableColumn, rows ...[]string) *webapi.Table {
	t := &webapi.Table{Name: "t", Columns: cols}
	for _, r := range rows {
		t.Rows = append(t.Rows, &webapi.TableRow{Cells: r})
	}
	return t
}

func firsts(t *webapi.Table) string {
	var out []string
	for _, r := range t.GetRows() {
		out = append(out, strings.Join(r.GetCells(), "/"))
	}
	return strings.Join(out, " ")
}

func TestOrderTablesByTypedColumns(t *testing.T) {
	cols := []*webapi.TableColumn{
		{Name: "rule"},
		{Name: "ref", Type: webapi.ColumnType_COLUMN_TYPE_NAME},
		{Name: "n", Type: webapi.ColumnType_COLUMN_TYPE_NUMBER},
	}
	tb := table(cols, []string{"b", "R10", "9"}, []string{"a", "R2", "10"}, []string{"b", "R2", ""}, []string{"a", "R10", "x"})
	if err := OrderTables([]*webapi.Table{tb}, []string{"rule", "-ref"}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := firsts(tb), "a/R10/x a/R2/10 b/R10/9 b/R2/"; got != want {
		t.Errorf("by rule, ref descending = %q, want %q", got, want)
	}
	// A number column orders numerically, unparsable after numbers and empty last.
	if err := OrderTables([]*webapi.Table{tb}, []string{"n"}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := firsts(tb), "b/R10/9 a/R2/10 a/R10/x b/R2/"; got != want {
		t.Errorf("by n = %q, want %q", got, want)
	}
	// An override reads the ref column as plain text, where R10 sorts before R2.
	if err := OrderTables([]*webapi.Table{tb}, []string{"ref", "rule"}, map[string]webapi.ColumnType{"ref": webapi.ColumnType_COLUMN_TYPE_TEXT}); err != nil {
		t.Fatal(err)
	}
	if got, want := firsts(tb), "a/R10/x b/R10/9 a/R2/10 b/R2/"; got != want {
		t.Errorf("ref as text = %q, want %q", got, want)
	}
}

func TestOrderTablesRefusesAColumnNoTableHas(t *testing.T) {
	tb := table([]*webapi.TableColumn{{Name: "rule"}}, []string{"a"})
	if err := OrderTables([]*webapi.Table{tb}, []string{"-nope"}, nil); !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown column = %v, want an invalid argument naming it", err)
	}
	if err := OrderTables([]*webapi.Table{tb}, []string{"-"}, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("a bare dash = %v, want an invalid argument", err)
	}
}

func TestTabulateACheckGivesFindingsThenVerdicts(t *testing.T) {
	resp := &webapi.CheckDesignResponse{
		Findings: []*checkspb.Finding{{Rule: "r", Severity: "error", Subject: &checkspb.Subject{Kind: "net", Ref: "N1"}}},
		Verdicts: []*checkspb.Verdict{
			{Id: "v2", Rule: "r", Outcome: checkspb.Outcome_OUTCOME_PASS, Subjects: []*checkspb.Subject{{Kind: "component", Ref: "R10"}},
				Witness: &checkspb.Witness{Statement: "pulled up", Terms: []*checkspb.WitnessTerm{{Label: "R", Value: "4k7"}}}},
			{Id: "v1", Rule: "r", Outcome: checkspb.Outcome_OUTCOME_FAIL, Subjects: []*checkspb.Subject{{Kind: "component", Ref: "R2"}}},
		},
	}
	out, err := TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{
		Answer: &webapi.TabulateRequest_Check{Check: resp}, OrderBy: []string{"subjects"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.GetTables()) != 3 || out.GetTables()[0].GetName() != "findings" || out.GetTables()[1].GetName() != "verdicts" || out.GetTables()[2].GetName() != "verdicts_by_rule" {
		t.Fatalf("tables = %v, want findings, verdicts, verdicts_by_rule", out.GetTables())
	}
	if got, want := firsts(out.GetTables()[2]), "r/1/1/0/0/0/2"; got != want {
		t.Errorf("verdicts_by_rule = %q, want %q", got, want)
	}
	if got, want := firsts(out.GetTables()[1]), "v1//r/fail/component:R2//// v2//r/pass/component:R10/pulled up//R=4k7/"; got != want {
		t.Errorf("verdicts = %q, want %q", got, want)
	}
	// A response with no verdicts gives findings alone rather than an empty verdicts table.
	resp.Verdicts = nil
	out, _ = TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_Check{Check: resp}})
	if len(out.GetTables()) != 1 {
		t.Errorf("a stripped response gave %d tables, want findings alone", len(out.GetTables()))
	}
}

func TestTabulateRefusesASetWithAFailedQuery(t *testing.T) {
	set := &webapi.RunQueriesResponse{Results: []*webapi.NamedQueryResult{
		{Name: "ok", Result: &webapi.RunQueryResponse{Columns: []string{"n"}}},
		{Name: "typo", Error: "unknown relation"},
	}}
	_, err := TableService{}.Tabulate(context.Background(), &webapi.TabulateRequest{Answer: &webapi.TabulateRequest_QuerySet{QuerySet: set}})
	if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), "typo") {
		t.Errorf("a failed query = %v, want an invalid argument naming it", err)
	}
}

func TestQueryTableTypesEntityColumnsAndAppendsProvenance(t *testing.T) {
	tb := QueryTable("q", &webapi.RunQueryResponse{
		Columns: []string{"n", "count(r)"}, ColumnKinds: []string{"net", ""},
		Rows: []*webapi.QueryRow{{Cells: []string{"GND", "3"}, Cites: []string{"a", "b"}}},
	})
	if got := tb.GetColumns(); len(got) != 3 || got[0].GetType() != webapi.ColumnType_COLUMN_TYPE_NAME || got[0].GetKind() != "net" || got[2].GetName() != "provenance" {
		t.Errorf("columns = %v, want n as a net name, count, then provenance", got)
	}
	if got := firsts(tb); got != "GND/3/a ; b" {
		t.Errorf("row = %q", got)
	}
}
