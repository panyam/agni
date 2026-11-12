// Command odb-verification checks a layout that came back as ODB++ against the schematic it was
// drawn from. It reads the job's netlist and board, runs the checks over both tiers, and diffs the
// layout's connectivity against the schematic's netlist. It is the walkthrough form of
// `agni check <job>` and `agni diff <schematic> <job>`.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/examples/common"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog
	"github.com/panyam/demokit"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// diffLimit caps how many items each diff section prints, matching agni's CLI edge.
const diffLimit = 40

func main() {
	schematic := common.AskPath("schematic", "../common/designs/odb/gateway.edn")
	layout := common.AskPath("layout", "../common/designs/odb/gateway-rev-b-layout")

	demo := demokit.New("odb-verification").
		Dir("odb-verification").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("schematic").Input(schematic.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		schematic.Capture(ctx)
		fmt.Printf("Schematic netlist: %s\n", schematic.Path())
		return nil
	})

	demo.Bind("layout").Input(layout.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		layout.Capture(ctx)
		fmt.Printf("Layout: %s\n", layout.Path())
		return nil
	})

	demo.Bind("read").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(layout.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", layout.Path(), err)
		}
		b, err := common.ReadBoard(layout.Path())
		if err != nil {
			return demokit.Errf("read the board in %s: %v", layout.Path(), err)
		}
		fmt.Println(common.StatsLines(d))
		segs, vias := 0, 0
		for _, nc := range b.GetNets() {
			segs += len(nc.GetSegments())
			vias += len(nc.GetVias())
		}
		fmt.Printf("\nBoard: %d copper layers, %d placed parts, %d routed nets (%d track segments, %d vias), %d copper pours.\n",
			copperLayers(b.GetLayers()), len(b.GetPlacements()), len(b.GetNets()), segs, vias, len(b.GetZones()))
		return nil
	})

	demo.Bind("check").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.ReadDesign(layout.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", layout.Path(), err)
		}
		b, err := common.ReadBoard(layout.Path())
		if err != nil {
			return demokit.Errf("read the board in %s: %v", layout.Path(), err)
		}
		fmt.Println(common.FindingsLines(check.RunBackground(check.NewModel(d, check.WithBoard(b)), check.BuiltinRules())))
		return nil
	})

	demo.Bind("compare").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		sch, err := schematic.Load()
		if err != nil {
			return demokit.Errf("load %s: %v", schematic.Path(), err)
		}
		lay, err := common.ReadDesign(layout.Path())
		if err != nil {
			return demokit.Errf("read %s: %v", layout.Path(), err)
		}
		fmt.Print(diff.Designs(sch, lay).Render(diffLimit))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// copperLayers counts the layers a board routes copper on.
func copperLayers(layers []*geom.BoardLayer) int {
	n := 0
	for _, l := range layers {
		if l.GetKind() == "signal" {
			n++
		}
	}
	return n
}
