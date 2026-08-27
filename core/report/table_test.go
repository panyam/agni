package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

func sampleTable() Table {
	return Table{
		Title:   "Test point coverage",
		Query:   `component.class(?r,"test_point") => ?r`,
		Source:  "mount://local/board.tel",
		Columns: []string{"net", "ref"},
		Rows: []TableRow{
			{Cells: []string{"+3V3", "TP1"}, Cites: []string{"board.tel:+3V3"}},
			{Cells: []string{"GND", "TP2"}, Cites: []string{"board.tel:GND", "board.tel:TP2"}},
		},
	}
}

// TestTableTextIsUnchanged pins the terminal format, which existed before the other four and is what
// anyone already piping `agni query` is parsing. Adding formats was not allowed to move it.
func TestTableTextIsUnchanged(t *testing.T) {
	var b bytes.Buffer
	if err := TableText(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	for _, want := range []string{"net", "ref", "provenance", "+3V3", "TP1", "board.tel:GND ; board.tel:TP2", "2 result(s)"} {
		if !strings.Contains(got, want) {
			t.Errorf("text output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Test point coverage") {
		t.Error("the terminal table gained a title; the text format is the one that must not change")
	}
}

// TestTableCSVKeepsHeaderWhenEmpty is the difference between a table that matched nothing and a run
// that produced no file. A downstream sheet binds to the header row, so dropping it on an empty
// result breaks the consumer exactly when the answer is "none", which is a common and valid answer.
func TestTableCSVKeepsHeaderWhenEmpty(t *testing.T) {
	tbl := sampleTable()
	tbl.Rows = nil
	var b bytes.Buffer
	if err := TableCSV(&b, tbl); err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("empty result wrote %d record(s), want just the header", len(recs))
	}
	if got, want := strings.Join(recs[0], ","), "net,ref,provenance"; got != want {
		t.Errorf("header = %q, want %q", got, want)
	}
}

// TestTableCSVCarriesNoPreamble guards the one asymmetry between csv and the document formats: csv's
// first row has to be the header, so the title and query that markdown and html carry must not
// appear here.
func TestTableCSVCarriesNoPreamble(t *testing.T) {
	var b bytes.Buffer
	if err := TableCSV(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(b.String(), "\n")
	if !strings.HasPrefix(first, "net,ref,provenance") {
		t.Errorf("csv does not open with the header row: %q", first)
	}
	if strings.Contains(b.String(), "Test point coverage") {
		t.Error("csv carries the title; a preamble breaks a reader binding to row 1")
	}
}

// TestTableCSVEscapesFormulaCells is the end-to-end half of SanitizeCell for this format. A net named
// +3V3 is real (the shipped demo board has one) and a spreadsheet executes it on open.
func TestTableCSVEscapesFormulaCells(t *testing.T) {
	var b bytes.Buffer
	if err := TableCSV(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(&b).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if recs[1][0] != "'+3V3" {
		t.Errorf("cell = %q, want the formula-escaped %q", recs[1][0], "'+3V3")
	}
}

// TestTableMarkdownSeparatorMatchesHeader catches a malformed table that still LOOKS fine in a diff:
// a separator row with the wrong number of cells makes the whole table render as paragraph text on
// GitHub, silently. This shipped broken once during authoring, with a trailing empty column.
func TestTableMarkdownSeparatorMatchesHeader(t *testing.T) {
	var b bytes.Buffer
	if err := TableMarkdown(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	lines := tableLines(b.String())
	if len(lines) < 2 {
		t.Fatalf("expected a header and a separator, got %d table line(s)", len(lines))
	}
	hdr, sep := cellCount(lines[0]), cellCount(lines[1])
	if hdr != sep {
		t.Errorf("header has %d cells, separator has %d: the table will not render", hdr, sep)
	}
	if hdr != 3 {
		t.Errorf("header has %d cells, want 3 (two columns plus provenance)", hdr)
	}
}

// TestTableMarkdownEscapesCellBreakers covers the two characters that silently corrupt a markdown
// table. A pipe ends the cell early; a newline ends the ROW, shifting every later cell one column
// left. Both occur in real data: provenance joins several citations and a net name is whatever the
// design called it.
func TestTableMarkdownEscapesCellBreakers(t *testing.T) {
	tbl := sampleTable()
	tbl.Rows = []TableRow{{Cells: []string{"A|B", "line1\nline2"}, Cites: []string{"c"}}}
	var b bytes.Buffer
	if err := TableMarkdown(&b, tbl); err != nil {
		t.Fatal(err)
	}
	lines := tableLines(b.String())
	row := lines[len(lines)-1]
	if n := cellCount(row); n != 3 {
		t.Errorf("row rendered %d cells, want 3: %q", n, row)
	}
	if !strings.Contains(row, `A\|B`) {
		t.Errorf("pipe not escaped: %q", row)
	}
}

// TestTableMarkdownEmptySaysSo. An empty markdown table reads as an omission or a broken generator,
// so a view that matched nothing states it in words. Same discipline as a check that did not run
// never rendering as a pass.
func TestTableMarkdownEmptySaysSo(t *testing.T) {
	tbl := sampleTable()
	tbl.Rows = nil
	var b bytes.Buffer
	if err := TableMarkdown(&b, tbl); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "No rows matched") {
		t.Errorf("empty view does not say it matched nothing:\n%s", b.String())
	}
	if strings.Contains(b.String(), "| --- |") {
		t.Error("empty view rendered a headerless table instead of a sentence")
	}
}

// TestTableMarkdownCarriesItsQuestion is the property that makes a view a view: the question is in
// the artifact, so a reader can re-run it, check it, or argue with it.
func TestTableMarkdownCarriesItsQuestion(t *testing.T) {
	var b bytes.Buffer
	if err := TableMarkdown(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Test point coverage", `component.class(?r,"test_point")`, "mount://local/board.tel"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("view is missing %q:\n%s", want, b.String())
		}
	}
}

// TestTableJSONKeepsCitesApart. The text and csv forms flatten provenance into one trailing cell
// because they have nowhere else to put it. json does, and a consumer that wants the citations
// wants them separable.
func TestTableJSONKeepsCitesApart(t *testing.T) {
	var b bytes.Buffer
	if err := TableJSON(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Columns []string `json:"columns"`
		Count   int      `json:"count"`
		Rows    []struct {
			Cells []string `json:"cells"`
			Cites []string `json:"cites"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 2 || len(got.Rows) != 2 {
		t.Fatalf("count=%d rows=%d, want 2 and 2", got.Count, len(got.Rows))
	}
	if len(got.Rows[1].Cites) != 2 {
		t.Errorf("second row has %d cite(s), want 2 kept apart", len(got.Rows[1].Cites))
	}
	if len(got.Columns) != 2 {
		t.Errorf("columns = %v, want the projection without provenance folded in", got.Columns)
	}
}

// TestTableHTMLEscapesDesignData. Every cell came out of a file this engine did not author, so a net
// name carrying a bracket must not become markup.
func TestTableHTMLEscapesDesignData(t *testing.T) {
	tbl := sampleTable()
	tbl.Rows = []TableRow{{Cells: []string{"<script>x</script>", "TP1"}, Cites: []string{"c"}}}
	var b bytes.Buffer
	if err := TableHTML(&b, tbl); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "<script>x</script>") {
		t.Error("a cell reached the page as live markup")
	}
	if !strings.Contains(b.String(), "&lt;script&gt;") {
		t.Errorf("cell was not escaped into the document:\n%s", b.String())
	}
}

// TestTableHTMLIsSelfContained. The page has to survive being emailed and opened from a file:// path,
// which is the same promise the check report and the checklist make.
func TestTableHTMLIsSelfContained(t *testing.T) {
	var b bytes.Buffer
	if err := TableHTML(&b, sampleTable()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "border-collapse") {
		t.Error("the shared stylesheet was not inlined")
	}
	for _, bad := range []string{"<link", "<script src", "http://", "https://"} {
		if strings.Contains(b.String(), bad) {
			t.Errorf("page reaches outside itself via %q", bad)
		}
	}
}

// tableLines returns just the pipe-table rows of a markdown document.
func tableLines(doc string) []string {
	var out []string
	for _, l := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "|") {
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// cellCount counts the cells in one markdown table row, which is the delimiters minus the two that
// bound it. An escaped pipe is not a delimiter.
func cellCount(row string) int {
	return strings.Count(strings.ReplaceAll(row, `\|`, ""), "|") - 1
}
