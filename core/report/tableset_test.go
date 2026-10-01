package report

import (
	"bytes"
	"strings"
	"testing"
)

func sampleSet() TableSet {
	return TableSet{
		Title: "Audit", Source: "mount://m/board.edn", Preamble: `has_tp(?n) :- component.net(?t,?n);`,
		Sections: []TableSection{
			{Name: "Probed nets", Description: "nets with a test point", Table: Table{
				Query: `has_tp(?n) => ?n`, Columns: []string{"n"},
				Rows: []TableRow{{Cells: []string{"GND"}, Cites: []string{"board.edn:GND"}}},
			}},
			{Name: "Empty", Table: Table{Query: `x(?a)`, Columns: []string{"a"}}},
			{Name: "Broken", Error: `unknown relation "compnent"`},
		},
	}
}

func render(t *testing.T, f func(*bytes.Buffer, TableSet) error, s TableSet) string {
	t.Helper()
	var b bytes.Buffer
	if err := f(&b, s); err != nil {
		t.Fatalf("render: %v", err)
	}
	return b.String()
}

func TestTableSetEveryFormatCarriesEverySection(t *testing.T) {
	formats := map[string]func(*bytes.Buffer, TableSet) error{
		"text":     func(b *bytes.Buffer, s TableSet) error { return TableSetText(b, s) },
		"markdown": func(b *bytes.Buffer, s TableSet) error { return TableSetMarkdown(b, s) },
		"html":     func(b *bytes.Buffer, s TableSet) error { return TableSetHTML(b, s) },
	}
	for name, f := range formats {
		out := render(t, f, sampleSet())
		for _, want := range []string{"Audit", "Probed nets", "GND", "Empty", "Broken", "compnent"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q", name, want)
			}
		}
		if name != "text" && !strings.Contains(out, "No rows matched") {
			t.Errorf("%s: an empty section must say it matched nothing", name)
		}
	}
}

// A section renders through the single-table renderer, so it cannot drift from the same table asked
// alone.
func TestTableSetSectionMatchesTheSingleTable(t *testing.T) {
	s := sampleSet()
	var alone bytes.Buffer
	if err := TableMarkdown(&alone, s.Sections[0].body()); err != nil {
		t.Fatal(err)
	}
	out := render(t, func(b *bytes.Buffer, s TableSet) error { return TableSetMarkdown(b, s) }, s)
	if !strings.Contains(out, alone.String()) {
		t.Errorf("section body differs from the table rendered alone:\n%s\n--- alone:\n%s", out, alone.String())
	}
}

func TestTableSetHTMLEscapesDesignText(t *testing.T) {
	s := sampleSet()
	s.Sections[0].Table.Rows[0].Cells[0] = "<script>x</script>"
	s.Sections[2].Error = "<b>bad</b>"
	out := render(t, func(b *bytes.Buffer, s TableSet) error { return TableSetHTML(b, s) }, s)
	if strings.Contains(out, "<script>x</script>") || strings.Contains(out, "<b>bad</b>") {
		t.Error("design-supplied text reached the page unescaped")
	}
}

func TestTableSetUntitledHasAHeading(t *testing.T) {
	s := sampleSet()
	s.Title = ""
	out := render(t, func(b *bytes.Buffer, s TableSet) error { return TableSetMarkdown(b, s) }, s)
	if !strings.HasPrefix(out, "# Query set") {
		t.Errorf("untitled set starts %q, want a Query set heading", out[:20])
	}
}
