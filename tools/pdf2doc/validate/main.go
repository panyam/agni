// Command validate loads a doc-IR textproto, runs doc.Validate, and prints a
// per-page region summary. The check harness for pdf2doc prototype output: a
// produced file that loads, validates (including table content-hash recomputation),
// and shows the expected tables is schema-conformant.
package main

import (
	"fmt"
	"os"

	"github.com/panyam/agni/doc"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: validate <doc-ir.textproto>")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	d, err := doc.Load(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := doc.Validate(d); err != nil {
		fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("valid: %s (%s, %d pages)\n", d.ContentHash[:23], d.Producer, d.PageCount)
	for _, pg := range d.Pages {
		fmt.Printf("  page %d: %d tables, %d figures, %d text blocks\n",
			pg.Number, len(pg.Tables), len(pg.Figures), len(pg.TextBlocks))
		for _, t := range pg.Tables {
			fmt.Printf("    %s %dx%d %q\n", t.Id, t.Rows, t.Cols, t.Title)
		}
		for _, fg := range pg.Figures {
			fmt.Printf("    %s fig %q\n", fg.Id, fg.Caption)
		}
	}
}
