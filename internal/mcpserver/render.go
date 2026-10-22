package mcpserver

import (
	"fmt"
	"strings"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
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

// writeLink writes the link that opens the answer in the viewer, or why there is none.
func writeLink(b *strings.Builder, link, why string) {
	switch {
	case link != "":
		fmt.Fprintf(b, "open in the viewer: %s\n", link)
	case why != "":
		fmt.Fprintf(b, "no viewer link: %s\n", why)
	}
}

// rowCount is n rows in words: "1 row", "3 rows".
func rowCount(n int) string {
	if n == 1 {
		return "1 row"
	}
	return fmt.Sprintf("%d rows", n)
}
