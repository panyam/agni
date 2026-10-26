package mcpserver

import (
	"fmt"
	"strings"

	rpt "github.com/panyam/agni/core/report"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// maxTextRows is the most rows of one table the text shows; the structured content carries them all.
const maxTextRows = 200

// writeTable writes t as a markdown table under its name, at most maxTextRows of it.
func writeTable(b *strings.Builder, t *webapi.Table) {
	if name := t.GetName(); name != "" {
		fmt.Fprintf(b, "%s:\n", name)
	}
	cols := make([]string, len(t.GetColumns()))
	for i, c := range t.GetColumns() {
		cols[i] = cell(c.GetName())
	}
	b.WriteString("| " + strings.Join(cols, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat(" --- |", len(cols)) + "\n")
	for i, r := range t.GetRows() {
		if i == maxTextRows {
			fmt.Fprintf(b, "(%d more rows; the structured content has every one)\n", len(t.GetRows())-maxTextRows)
			break
		}
		cells := make([]string, len(r.GetCells()))
		for j, c := range r.GetCells() {
			cells[j] = cell(c)
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}
	b.WriteString("\n")
}

// cell is s as one markdown table cell: on one line, with its pipes escaped.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	return strings.Join(strings.Fields(s), " ")
}

// maxWitnessLines is the most lines of one row's witness tree the text shows, the cap jaala's own text
// format uses; the structured content carries every tree whole.
const maxWitnessLines = 8

// writeWitnesses writes how each row of the page holds (agni issue 1166), numbered from offset as the
// whole answer numbers them, in the tree `agni query --witnesses` prints. It writes nothing for an
// answer with no witnesses, such as an aggregate.
func writeWitnesses(b *strings.Builder, resp *webapi.RunQueryResponse, offset int) {
	header := false
	for i, r := range resp.GetRows() {
		if i == maxTextRows {
			break
		}
		lines := rpt.WitnessLines(service.RowWitnesses(resp, r))
		if len(lines) == 0 {
			continue
		}
		if !header {
			b.WriteString("how each row holds:\n")
			header = true
		}
		fmt.Fprintf(b, "row %d: %s\n", offset+i+1, strings.Join(r.GetCells(), ", "))
		for j, l := range lines {
			if j == maxWitnessLines {
				fmt.Fprintf(b, "  … %s\n", moreFacts(len(lines)-maxWitnessLines))
				break
			}
			b.WriteString("  " + l + "\n")
		}
	}
	if header {
		b.WriteString("\n")
	}
}

// writeLink writes the link that opens the answer in the viewer, or why there is none.
func writeLink(b *strings.Builder, link, why string) {
	switch {
	case link != "":
		fmt.Fprintf(b, "open in the viewer: %s\n", link)
	case why != "":
		fmt.Fprintf(b, "no viewer link: %s\n", why)
	}
}

// moreFacts is n facts left out, in words: "1 more fact", "3 more facts".
func moreFacts(n int) string {
	if n == 1 {
		return "1 more fact"
	}
	return fmt.Sprintf("%d more facts", n)
}

// rowCount is n rows in words: "1 row", "3 rows".
func rowCount(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}
