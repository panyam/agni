// Command diff is the diff rung of the Agni examples ladder: compare two revisions of a
// design and read the semantic change taxonomy. It is the walkthrough form of `agni diff`.
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
	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// diffLimit caps how many items each diff section prints, matching agni's CLI edge.
const diffLimit = 40

func main() {
	// Two shared path inputs: the old and new revisions, each defaulting to a bundled fixture.
	oldRev := common.AskPath("old", "../common/designs/rev-a.edn")
	newRev := common.AskPath("new", "../common/designs/rev-b.edn")

	demo := demokit.New("diff").
		Dir("diff").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("old").Input(oldRev.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		oldRev.Capture(ctx)
		fmt.Printf("Old revision: %s\n", oldRev.Path())
		return nil
	})

	demo.Bind("new").Input(newRev.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		newRev.Capture(ctx)
		fmt.Printf("New revision: %s\n", newRev.Path())
		return nil
	})

	demo.Bind("run").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		a, err := oldRev.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", oldRev.Path(), err)
		}
		b, err := newRev.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", newRev.Path(), err)
		}
		fmt.Print(diff.Designs(a, b).Render(diffLimit))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
