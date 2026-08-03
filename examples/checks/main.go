// Command checks is the checks rung of the Agni examples ladder: run the structural rule
// checks over a design and read the findings. It is the walkthrough form of `agni check`.
// The narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only
// binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// The shared path input: default to the bundled fixture, accept any path the user enters.
	design := common.AskPath("design", "../common/designs/i2c-sensor.edn")

	demo := demokit.New("checks").
		Dir("checks").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := design.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Println(common.FindingsLines(check.RunDesign(d)))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
