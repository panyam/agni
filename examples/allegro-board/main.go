// Command allegro-board reads a Cadence Allegro board straight from its .brd, with no export from
// Allegro, and runs the checks over it, the copper rules included. The narration lives in the sidecar
// walkthrough.md.
//
// Nobody can author a synthetic Allegro board without Allegro, so this example reads a public one
// from the pinned samples corpus (CONSTRAINTS C10): the BeagleBone AI, fetched by
// `make samples-oracle` into tools/samples/.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// sampleBoard is the BeagleBone AI in the allegro samples artifact, relative to this folder.
const sampleBoard = "../../tools/samples/allegro/beaglebone-ai/BeagleBone-AI.brd"

func main() {
	board := common.AskPath("board", sampleBoard)

	demo := demokit.New("allegro-board").
		Dir("allegro-board").
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

	demo.Bind("pairs").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(board.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", board.Path(), err)
		}
		fmt.Println(groupLines(d))
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

// groupLines lists the differential pairs and match groups a design declares, one per line.
func groupLines(d *ir.Design) string {
	var pairs, groups []string
	for _, g := range d.GetNetGroups() {
		line := fmt.Sprintf("  %-12s %s", g.GetName(), strings.Join(g.GetNets(), " "))
		if g.GetKind() == ir.NetGroupKind_NET_GROUP_KIND_DIFF_PAIR {
			pairs = append(pairs, line)
		} else {
			groups = append(groups, line)
		}
	}
	return fmt.Sprintf("%d differential pairs:\n%s\n%d match groups:\n%s",
		len(pairs), strings.Join(pairs, "\n"), len(groups), strings.Join(groups, "\n"))
}
