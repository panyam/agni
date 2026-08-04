// Command render-highlight is the explainability rung of the Agni examples ladder: run the
// rule catalog over a design, then BAKE each finding's subject as a highlight into a single
// rendered SVG, so a report finding becomes a picture of the actual design. It is the offline,
// static twin of the web viewer's click-to-locate — one render.SheetSVGHighlighted call over the
// same HighlightSpec vocabulary the server projects as a live overlay. Narration lives in the
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

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/examples/common"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/formats"
	_ "github.com/panyam/agni/stdlib/rules/builtin" // register the built-in rule catalog RunDesign runs
)

//go:embed walkthrough.md
var walkthroughMD []byte

func main() {
	// Default to a bundled KiCad schematic with faithful geometry AND a clear finding (two symbols
	// share ref-des U1): a component finding that draws a box on each real placement.
	design := common.AskPath("design", "../common/designs/duplicate-refdes.kicad_sch")

	demo := demokit.New("render-highlight").
		Dir("render-highlight").
		FromMarkdownBytes(walkthroughMD)

	demo.Bind("pick").Input(design.Def()).Run(func(ctx demokit.StepContext) *demokit.StepResult {
		design.Capture(ctx)
		fmt.Printf("Selected %s.\n", design.Path())
		return nil
	})

	demo.Bind("check").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.Load(design.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		findings := check.RunDesign(d)
		fmt.Println(common.FindingsLines(findings))
		return nil
	})

	demo.Bind("render").Run(func(ctx demokit.StepContext) *demokit.StepResult {
		d, err := common.Load(design.Path())
		if err != nil {
			return demokit.Errf("load %s: %v", design.Path(), err)
		}
		// FaithfulGeometry dispatches by extension (KiCad, EDIF .eds, ...), the same format-aware
		// geometry the CLI renders — so the highlight lands on the design's own drawing, not an
		// EDIF-only path. common.Load already read the netlist above; this reads the render canvas.
		g, err := (&formats.Loader{}).FaithfulGeometry(design.Path())
		if err != nil {
			return demokit.Errf("load geometry %s: %v", design.Path(), err)
		}
		if len(g.Sheets) == 0 {
			return demokit.Errf("no sheets in %s", design.Path())
		}
		findings := check.RunDesign(d)
		specs := specsForFindings(findings)
		svg := render.SheetSVGHighlighted(g, g.Sheets[0], specs)
		if err := os.WriteFile("render.svg", []byte(svg), 0o644); err != nil {
			return demokit.Errf("write render.svg: %v", err)
		}
		fmt.Printf("Wrote render.svg (%d bytes): sheet %q with %d finding(s) baked in as highlights.\n",
			len(svg), g.Sheets[0].Name, len(specs))
		fmt.Println("Open it: each flagged net/component/pin is drawn framed, in place, on the real schematic.")
		return nil
	})

	common.SetupRenderer(demo)
	demo.Execute()
}

// specsForFindings maps each finding's subject to one HighlightSpec, the same net/component/pin
// vocabulary the web click-to-locate builds. A net draws as a PATH marker along its wire (and
// carries its per-instance net id so two same-named nets stay distinct); a component or pin draws
// as a translucent bounding box, one per matched placement. Severity picks the color so an error
// reads hotter than a warning. This is the CLI/report side of the finding->picture join: findings
// are computed on the netlist, located on the geometry by name/id (CONSTRAINTS C21).
func specsForFindings(findings []check.Finding) []*geom.HighlightSpec {
	var specs []*geom.HighlightSpec
	for _, f := range findings {
		spec := &geom.HighlightSpec{Color: colorFor(f.Severity)}
		switch f.Kind {
		case check.KindNet:
			spec.Nets = []string{f.Subject}
			if f.NetID != "" {
				spec.NetIds = []string{f.NetID}
			}
			spec.Shape = geom.HighlightShape_HIGHLIGHT_SHAPE_PATH
		case check.KindPin:
			spec.Pins = []*geom.PinRef{{RefDes: f.Subject, Pin: f.Pin}}
		default: // KindComponent
			spec.Components = []string{f.Subject}
			spec.Shape = geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT
		}
		specs = append(specs, spec)
	}
	return specs
}

// colorFor maps a finding severity to a highlight color: error hot red, warning amber, else the
// renderer default. Empty ("") lets render pick DefaultHighlightColor.
func colorFor(severity string) string {
	switch severity {
	case "error":
		return "#e11d48"
	case "warning":
		return "#f59e0b"
	default:
		return ""
	}
}
