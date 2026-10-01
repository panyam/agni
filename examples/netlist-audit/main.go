// Command netlist-audit walks a netlist audit written as a query set (agni issue 729). Each table a
// review workbook holds (parts on nets, part numbers, test points per net, passives a tester can
// measure, part numbers never probed on both ends) is a named query in audit.yaml, and all of them
// are answered over one read of the design.
//
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown), and this file only binds
// the steps that run engine code. Every step prints the CLI line that reproduces it.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/examples/common"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/lib"       // register the derived relations the audit calls
	_ "github.com/panyam/agni/stdlib/relations" // register the fact relations the audit reads
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

//go:embed audit.yaml
var auditYAML []byte

func main() {
	design := common.AskPath("design", "../common/designs/netlist-audit.tel")

	demo := demokit.New("netlist-audit").
		Dir("netlist-audit").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("the-set").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		set, err := query.ParseQuerySet(auditYAML)
		if err != nil {
			return demokit.Errf("audit.yaml: %v", err)
		}
		fmt.Printf("%q holds %d queries:\n", set.Title, len(set.Queries))
		for _, q := range set.Queries {
			fmt.Printf("  - %s\n", q.Name)
		}
		return nil
	})

	demo.Bind("answer").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		cli("agni query <design> --set audit.yaml")
		sections, err := answerSet(d, auditYAML)
		if err != nil {
			return demokit.Errf("%v", err)
		}
		for _, s := range sections {
			fmt.Println(s.summary())
		}
		return nil
	})

	demo.Bind("workbook").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		cli("agni query <design> --set audit.yaml --format html -o /tmp/audit.html")
		cli("python clients/python/examples/audit_workbook.py -o audit.xlsx")
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// section is one query's answer, as the walkthrough prints it.
type section struct {
	name string
	cols []string
	rows [][]string
	err  error
}

// answerSet answers every query of a set over ONE fact base. The design is read and projected once,
// and each query is a lookup against the same indexes.
func answerSet(d *ir.Design, yaml []byte) ([]section, error) {
	set, err := query.ParseQuerySet(yaml)
	if err != nil {
		return nil, err
	}
	base := query.NewBase(check.NewModel(d))
	out := make([]section, 0, len(set.Queries))
	for i, nq := range set.Queries {
		s := section{name: nq.Name}
		q, err := set.Compile(i)
		if err == nil {
			var got []query.Row
			got, err = query.Naive{}.Eval(q, base)
			for _, c := range q.Columns() {
				s.cols = append(s.cols, string(c))
			}
			for _, r := range got {
				row := make([]string, len(s.cols))
				for j, c := range q.Columns() {
					row[j] = r.Bind[c].S
				}
				s.rows = append(s.rows, row)
			}
		}
		s.err = err
		out = append(out, s)
	}
	return out, nil
}

// rowsCap bounds the rows a section prints. The fixture's tables are small, and a real board's "parts
// on nets" runs to thousands of rows, which would bury every other table (agni issue 644). The count
// is never truncated.
const rowsCap = 6

func (s section) summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%s", s.name)
	if s.err != nil {
		fmt.Fprintf(&b, ": could not answer: %v", s.err)
		return b.String()
	}
	fmt.Fprintf(&b, " (%d rows)", len(s.rows))
	if len(s.rows) == 0 {
		return b.String()
	}
	fmt.Fprintf(&b, "\n  %s", strings.Join(s.cols, " | "))
	for i, r := range s.rows {
		if i == rowsCap {
			fmt.Fprintf(&b, "\n  ... %d more", len(s.rows)-rowsCap)
			break
		}
		fmt.Fprintf(&b, "\n  %s", strings.Join(r, " | "))
	}
	return b.String()
}

// cli prints the command a reader can paste into a second terminal to reproduce the step.
func cli(line string) { fmt.Printf("$ %s\n\n", line) }
