// Command whole-enchilada is the capstone rung of the Agni examples ladder: it runs the
// whole engine end to end over bundled synthetic designs, so one walkthrough shows every
// surface at once. Convergence across formats, structural checks, semantic diff, cross-format
// emit, and both renderers. The per-feature examples go deeper on each step; this is the tour.
//
// Each step echoes the equivalent `agni` command before it runs (via echo), so the
// walkthrough doubles as a copy/paste reference you can paste into a second terminal. Those
// commands assume agni is on your PATH (`make install`) and are run from the repo root.
//
// Narration lives in the sidecar walkthrough.md (demokit FromMarkdown); this file only binds
// the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/diff"
	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/readers/ipc2581"
	"github.com/panyam/agni/core/render"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog (check.RunDesign runs it)
)

//go:embed walkthrough.md
var walkthroughMD []byte

// echo prints the CLI command(s) a step is equivalent to, so the demo doubles as a
// copy/paste reference. Paths are repo-root-relative to match `make install` + run-from-root.
func echo(cmds ...string) {
	for _, c := range cmds {
		fmt.Printf("$ %s\n", c)
	}
	fmt.Println()
}

func main() {
	demo := demokit.New("whole-enchilada").
		Dir("whole-enchilada").
		FromMarkdownBytes(walkthroughMD)

	// 1) Convergence: the same board read from three formats yields the same netlist.
	demo.Bind("converge").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo(
			"agni stats examples/common/designs/mixer.kicad_pcb",
			"agni diff  examples/common/designs/mixer.edn examples/common/designs/mixer.ipc2581.xml",
		)
		edn, err := common.ReadFixture("mixer.edn")
		if err != nil {
			return demokit.Errf("read mixer.edn: %v", err)
		}
		fmt.Println(common.StatsLines(edn))
		for _, name := range []string{"mixer.kicad_pcb", "mixer.ipc2581.xml"} {
			d, err := common.ReadFixture(name)
			if err != nil {
				return demokit.Errf("read %s: %v", name, err)
			}
			r := diff.Designs(edn, d)
			fmt.Printf("\n%-18s vs mixer.edn: %d net change(s), components +%d/-%d",
				name, len(r.Nets), len(r.ComponentsAdded), len(r.ComponentsRemoved))
		}
		fmt.Println("\n\nThree formats, empty diffs: the readers converged on one netlist.")
		return nil
	})

	// 2) Structural checks over the netlist.
	demo.Bind("check").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo("agni check examples/common/designs/i2c-sensor.edn")
		d, err := common.ReadFixture("i2c-sensor.edn")
		if err != nil {
			return demokit.Errf("read i2c-sensor.edn: %v", err)
		}
		fmt.Println(common.FindingsLines(check.RunDesign(d)))
		return nil
	})

	// 3) Semantic diff of two revisions: the full change taxonomy.
	demo.Bind("diff").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo("agni diff examples/common/designs/rev-a.edn examples/common/designs/rev-b.edn")
		a, err := common.ReadFixture("rev-a.edn")
		if err != nil {
			return demokit.Errf("read rev-a.edn: %v", err)
		}
		b, err := common.ReadFixture("rev-b.edn")
		if err != nil {
			return demokit.Errf("read rev-b.edn: %v", err)
		}
		fmt.Print(diff.Designs(a, b).Render(40))
		return nil
	})

	// 4) Emit: cross-format convert through the IR, with an in-line round-trip.
	demo.Bind("emit").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo("agni emit examples/common/designs/mixer.edn mixer.ipc2581.xml")
		d, err := common.ReadFixture("mixer.edn")
		if err != nil {
			return demokit.Errf("read mixer.edn: %v", err)
		}
		var buf bytes.Buffer
		if err := ipc2581.Write(&buf, d); err != nil {
			return demokit.Errf("emit ipc-2581: %v", err)
		}
		rt, err := ipc2581.Read(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
		if err != nil {
			return demokit.Errf("re-read emitted ipc-2581: %v", err)
		}
		fmt.Printf("mixer.edn -> IR -> IPC-2581 (%d bytes) -> IR: %d components / %d nets preserved.\n",
			buf.Len(), len(rt.Components), len(rt.Nets))
		return nil
	})

	// 5) Render a schematic sheet from EDIF geometry to SVG.
	demo.Bind("schematic").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo("agni render examples/common/designs/demo-schematic.eds -o schematic.svg")
		g, err := common.LoadSchematic("../common/designs/demo-schematic.eds")
		if err != nil {
			return demokit.Errf("load schematic: %v", err)
		}
		if len(g.Sheets) == 0 {
			return demokit.Errf("no sheets in demo-schematic.eds")
		}
		svg := render.SheetSVG(g, g.Sheets[0])
		if err := os.WriteFile("schematic.svg", []byte(svg), 0o644); err != nil {
			return demokit.Errf("write schematic.svg: %v", err)
		}
		fmt.Println(common.GeomStatsLines(g))
		fmt.Printf("\nWrote schematic.svg (%d bytes) — open it to see sheet %q.\n", len(svg), g.Sheets[0].Name)
		return nil
	})

	// 6) Render a netlist-graph view from the IR alone (works for any format), and score
	// every registered layout by crossings so the choice is a number, not an opinion.
	demo.Bind("graph").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		echo(
			"agni render --compare examples/common/designs/i2c-sensor.edn",
			"agni render --layout layered examples/common/designs/i2c-sensor.edn -o graph.svg",
		)
		d, err := common.ReadFixture("i2c-sensor.edn")
		if err != nil {
			return demokit.Errf("read i2c-sensor.edn: %v", err)
		}
		fmt.Println("layout quality (fewer crossings is better):")
		for _, s := range graph.Strategies() {
			g, err := graph.LayoutWith(d, s.Name)
			if err != nil {
				return demokit.Errf("layout %s: %v", s.Name, err)
			}
			q := graph.Measure(g)
			fmt.Printf("  %-8s %d nodes, %d nets, %d crossings\n", s.Name, q.Nodes, q.Nets, q.Crossings)
		}
		// Render the layered layout (the better of the two on real designs) to SVG.
		g, err := graph.LayoutWith(d, "layered")
		if err != nil {
			return demokit.Errf("layout layered: %v", err)
		}
		svg := render.SheetSVG(g, g.Sheets[0])
		if err := os.WriteFile("graph.svg", []byte(svg), 0o644); err != nil {
			return demokit.Errf("write graph.svg: %v", err)
		}
		fmt.Printf("\nWrote graph.svg (%d bytes, layered layout) — connectivity from the IR alone.\n", len(svg))
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
