// Command multi-format is the multi-format rung of the Agni examples ladder: read the
// same board from EDIF, KiCad, and IPC-2581 and show the neutral IR converge. The narration
// lives in the sidecar walkthrough.md (demokit FromMarkdown); this file binds the steps that
// run engine code. Unlike the other examples it reads a fixed bundled trio (the point is the
// matched set), so it does not prompt for a path.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"

	"github.com/panyam/demokit"
	"github.com/panyam/agni/diff"
	"github.com/panyam/agni/examples/common"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// formats is the matched trio: the same board authored once per source format.
var formats = []struct{ label, fixture string }{
	{"EDIF netlist", "mixer.edn"},
	{"KiCad PCB", "mixer.kicad_pcb"},
	{"IPC-2581", "mixer.ipc2581.xml"},
}

func main() {
	demo := demokit.New("multi-format").
		Dir("multi-format").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("read").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		for _, f := range formats {
			d, err := common.ReadFixture(f.fixture)
			if err != nil {
				return demokit.Errf("read %s: %v", f.fixture, err)
			}
			fmt.Printf("--- %s (%s) ---\n%s\n\n", f.label, f.fixture, common.StatsLines(d))
		}
		return nil
	})

	demo.Bind("converge").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		base, err := common.ReadFixture(formats[0].fixture)
		if err != nil {
			return demokit.Errf("read %s: %v", formats[0].fixture, err)
		}
		for _, f := range formats[1:] {
			d, err := common.ReadFixture(f.fixture)
			if err != nil {
				return demokit.Errf("read %s: %v", f.fixture, err)
			}
			r := diff.Designs(base, d)
			fmt.Printf("EDIF vs %-9s -> %d net change(s), %d component add/remove, %d attribute-only component change(s)\n",
				f.label, len(r.Nets), len(r.ComponentsAdded)+len(r.ComponentsRemoved), len(r.ComponentsChanged))
		}
		fmt.Println("\nZero net changes and no components added or removed: the connectivity is")
		fmt.Println("identical across all three formats. The attribute-only changes are metadata")
		fmt.Println("each format spells differently (a Value, a part reference), not connectivity.")
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
