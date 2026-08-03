// Command read-and-stats is the first rung of the Agni examples ladder: read a source
// file into the neutral IR and look at what came back. The narration lives in the sidecar
// walkthrough.md (loaded via demokit's FromMarkdown), so this file only binds the steps that
// run engine code and wires the renderer.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// The shared path input: default to the bundled fixture, accept any path the user enters.
	design := common.AskPath("design", "../common/designs/two-resistors.edn")

	demo := demokit.New("read-and-stats").
		Dir("read-and-stats").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("read").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Println(common.StatsLines(d))
		return nil
	})

	demo.Bind("nets").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Println(common.NetLines(d, 12))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
