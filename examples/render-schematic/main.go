// Command render-schematic is the render rung of the Agni examples ladder: read an EDIF
// schematic into the geometry sidecar and drive both render backends over it — the SVG one
// (offline) and the tier-2 packer that feeds the WebGL2 viewer. Narration lives in the
// sidecar walkthrough.md; this file only binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/demokit"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/render"
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// The shared path input: default to the bundled schematic, accept any .eds path.
	design := common.AskPath("design", "../common/designs/demo-schematic.eds")

	demo := demokit.New("render-schematic").
		Dir("render-schematic").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("read").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		g, err := common.LoadSchematic(design.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		fmt.Println(common.GeomStatsLines(g))
		return nil
	})

	demo.Bind("svg").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		g, err := common.LoadSchematic(design.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		if len(g.Sheets) == 0 {
			return demokit.Errf("no sheets in %s", design.Path())
		}
		svg := render.SheetSVG(g, g.Sheets[0])
		if err := os.WriteFile("render.svg", []byte(svg), 0o644); err != nil {
			return demokit.Errf("write render.svg: %v", err)
		}
		fmt.Printf("Wrote render.svg (%d bytes) — open it to see sheet %q.\n", len(svg), g.Sheets[0].Name)
		return nil
	})

	demo.Bind("pack").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		g, err := common.LoadSchematic(design.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		ps := render.PackSheet(g, g.Sheets[0])
		blob, err := proto.Marshal(ps)
		if err != nil {
			return demokit.Errf("marshal packed sheet: %v", err)
		}
		// Write into web/ (where the viewer serves from) as a gitignored *.local.pb, so
		// the printed URL loads it via the viewer's ?src= parameter.
		base := filepath.Base(design.Path())
		name := strings.TrimSuffix(base, filepath.Ext(base)) + ".local.pb"
		out := filepath.Join("..", "..", "web", name)
		if err := os.WriteFile(out, blob, 0o644); err != nil {
			return demokit.Errf("write %s: %v", out, err)
		}
		fmt.Printf("PackedSheet: %d vertices, %d primitives, %d keys.\n",
			len(ps.Vertices)/8, len(ps.Primitives)/12, len(ps.Keys))
		fmt.Printf("Wrote web/%s. Start the viewer (cd ../../web && pnpm dev), then open:\n", name)
		fmt.Printf("  http://localhost:5178/?src=%s\n", name)
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
