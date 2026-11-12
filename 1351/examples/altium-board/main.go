// Command altium-board reads an Altium Designer board straight from its .PcbDoc, with no ODB++
// export from Altium, and runs the checks over it, the copper rules included. The narration lives
// in the sidecar walkthrough.md.
//
// Nobody can author a synthetic Altium board without Altium, so this example reads a public one
// from the pinned samples corpus (CONSTRAINTS C10): the foxBMS master board, fetched by
// `make samples-oracle` into tools/samples/.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// sampleBoard is the foxBMS master board in the altium samples artifact, relative to this folder.
const sampleBoard = "../../tools/samples/altium/foxbms/hw-master/master.PcbDoc"

func main() {
	board := common.AskPath("board", sampleBoard)

	demo := demokit.New("altium-board").
		Dir("altium-board").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(board.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		board.Capture(ctx)
		fmt.Printf("Selected %s.\n", board.Path())
		return nil
	})

	demo.Bind("netlist").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(board.Path())
		if err != nil {
			return demokit.Errf("read %s: %v (run `make samples-oracle` from the repo root)", board.Path(), err)
		}
		fmt.Println(common.StatsLines(d))
		return nil
	})

	demo.Bind("board").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		g, err := common.ReadBoard(board.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", board.Path(), err)
		}
		if g == nil {
			return demokit.Errf("%s carries no board geometry", board.Path())
		}
		fmt.Println(common.BoardLines(g))
		return nil
	})

	demo.Bind("check").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(board.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", board.Path(), err)
		}
		g, err := common.ReadBoard(board.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", board.Path(), err)
		}
		fmt.Println(common.RuleTally(check.RunBackground(check.NewModel(d, check.WithBoard(g)), check.BuiltinRules())))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
