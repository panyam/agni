// Command validate is the reader-health rung of the Agni examples ladder: run the
// validate package's structural invariants over a design and read the problem lists. It is
// the walkthrough form of `agni validate`. The narration lives in the sidecar
// walkthrough.md (demokit FromMarkdown); this file only binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/readers/edif"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/validate"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// problemsLine renders a problem list the way the CLI's table does: ok, or the reasons.
func problemsLine(problems []string) string {
	if len(problems) == 0 {
		return "ok: all invariants hold"
	}
	out := "FAIL:"
	for _, p := range problems {
		out += "\n  - " + p
	}
	return out
}

func main() {
	design := common.AskPath("design", "../common/designs/i2c-sensor.edn")

	demo := demokit.New("validate").
		Dir("validate").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("netlist").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Printf("netlist (%d components, %d nets): %s\n", len(d.Components), len(d.Nets), problemsLine(validate.Design(d)))
		return nil
	})

	demo.Bind("geometry").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		const eds = "../common/designs/demo-schematic.eds"
		f, err := os.Open(eds)
		if err != nil {
			return demokit.Errf("open %s: %v", eds, err)
		}
		defer f.Close()
		g, err := edif.ReadSchematic(f, eds)
		if err != nil {
			return demokit.Errf("read schematic: %v", err)
		}
		placements := 0
		for _, s := range g.Sheets {
			placements += len(s.Placements)
		}
		fmt.Printf("geometry (%d sheets, %d placements, %d resolved): %s\n",
			len(g.Sheets), placements, validate.Resolved(g), problemsLine(validate.Geometry(g)))
		return nil
	})

	demo.Bind("failure").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		fmt.Println("an empty design:", problemsLine(validate.Design(&ir.Design{})))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
