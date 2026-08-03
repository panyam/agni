// Command render-board is the board rung of the Agni examples ladder: read a KiCad
// board (.kicad_pcb) into the WS1-006 board geometry sidecar and render it — the physical
// outline, per-layer copper, pads, and vias — plus a highlight overlay joining a net to its
// routed copper (the same join the web viewer's findings use). Narration lives in the
// sidecar walkthrough.md; this file only binds the steps that run engine code.
//
// Run modes (see the Makefile): `make run` (plain text), `make demo` (TUI boxes),
// `make runquiet` (non-interactive defaults, CI-safe), `make doc` (render to markdown).
package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/panyam/demokit"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/examples/common"
	"github.com/panyam/agni/kicad"
	"github.com/panyam/agni/render"
)

//go:embed walkthrough.md
var walkthroughMD []byte

// loadBoard reads a .kicad_pcb into the board sidecar.
func loadBoard(path string) (*geom.BoardGeometry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return kicad.ReadBoardGeometry(f, path)
}

func main() {
	board := common.AskPath("board", "../common/designs/demo-board.kicad_pcb")

	demo := demokit.New("render-board").
		Dir("render-board").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(board.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		board.Capture(ctx)
		fmt.Printf("Selected %s.\n", board.Path())
		return nil
	})

	demo.Bind("read").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		b, err := loadBoard(board.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", board.Path(), err)
		}
		segs, vias := 0, 0
		for _, nc := range b.GetNets() {
			segs += len(nc.GetSegments())
			vias += len(nc.GetVias())
		}
		fmt.Printf("Board %q: %d layers, %d placements, %d routed nets (%d segments, %d vias), %d zones.\n",
			b.GetDesignRef(), len(b.GetLayers()), len(b.GetPlacements()), len(b.GetNets()), segs, vias, len(b.GetZones()))
		return nil
	})

	demo.Bind("svg").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		b, err := loadBoard(board.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", board.Path(), err)
		}
		svg := render.BoardSVG(b)
		if err := os.WriteFile("board.svg", []byte(svg), 0o644); err != nil {
			return demokit.Errf("write board.svg: %v", err)
		}
		fmt.Printf("Wrote board.svg (%d bytes) — outline, copper per layer, pads, vias.\n", len(svg))
		return nil
	})

	demo.Bind("highlight").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		b, err := loadBoard(board.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", board.Path(), err)
		}
		if len(b.GetNets()) == 0 {
			fmt.Println("No routed copper to highlight on this board.")
			return nil
		}
		net := b.GetNets()[0].GetNet()
		overlay := render.HighlightBoardSVG(b, []*geom.HighlightSpec{{Nets: []string{net}}})
		if err := os.WriteFile("board-highlight.svg", []byte(overlay), 0o644); err != nil {
			return demokit.Errf("write board-highlight.svg: %v", err)
		}
		fmt.Printf("Wrote board-highlight.svg: net %q re-stroked on its copper, connected pads ringed.\n", net)
		fmt.Println("Stack it over board.svg (same frame) — the join the web viewer's findings use.")
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}
