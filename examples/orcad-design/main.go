// Command orcad-design reads an OrCAD Capture design straight from its .DSN files, every block it
// places included, and runs the checks over it. The narration lives in the sidecar walkthrough.md.
//
// Nobody can author a synthetic Capture design without Capture, so this example reads a public one
// from the pinned samples corpus (CONSTRAINTS C10): Clay R6, fetched by `make samples-oracle` into
// tools/samples/.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/report"
	"github.com/panyam/agni/examples/common"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// sampleDesign is Clay R6's top design in the cadence-exports samples artifact, relative to this
// folder. Its ten blocks sit in folders beside it.
const sampleDesign = "../../tools/samples/cadence/clay-r6/CLAYR6.DSN"

func main() {
	design := common.AskPath("design", sampleDesign)

	demo := demokit.New("orcad-design").
		Dir("orcad-design").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("netlist").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(design.Path())
		if err != nil {
			return demokit.Errf("read %s: %v (run `make samples-oracle` from the repo root)", design.Path(), err)
		}
		fmt.Println(common.StatsLines(d))
		if note := report.HierarchyNote(design.Path(), d.GetInputDiagnostics().GetUnexpandedHierarchy()); note != "" {
			fmt.Println(note)
		} else {
			fmt.Println("Every block the design places was read.")
		}
		return nil
	})

	demo.Bind("check").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(design.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", design.Path(), err)
		}
		fmt.Println(common.RuleTally(check.RunBackground(check.NewModel(d), check.BuiltinRules())))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
