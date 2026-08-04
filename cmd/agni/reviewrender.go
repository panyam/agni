package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	"github.com/panyam/agni/core/review"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/formats"
)

// renderReviewImages writes an annotated schematic image for each design in the review that has
// findings: it loads the design's render geometry (faithful when the format carries it, else the
// default auto-layout, the same choice the viewer makes), bakes every finding's subject as a
// highlight, and writes one SVG per sheet the findings actually land on, to
// <outDir>/<design-stem>/<sheet>.svg. It is the report-side twin of the web click-to-locate: a
// static picture of each failing design that a markdown report can embed. Designs with no findings
// (all pass / n-a) are skipped. A per-design load failure is reported and skipped, never fatal, so
// one unreadable design does not sink the rollup. Returns a human summary of what was written.
func renderReviewImages(reports []review.Report, outDir string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("--render %s: %w", outDir, err)
	}
	reg, err := buildRegistry(nil, "")
	if err != nil {
		return "", err
	}
	l := newLoader()

	var lines []string
	for _, r := range reports {
		specs := findingSpecs(reviewReportFindings(r))
		if len(specs) == 0 {
			continue // nothing flagged: no picture to draw
		}
		layout := graph.DefaultStrategy
		if formats.HasFaithful(r.Design) {
			layout = formats.LayoutFaithful
		}
		g, err := l.ResolveGeometry(r.Design, layout, reg, symbolsGlyph)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  %s: skipped (%v)", r.Design, err))
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(r.Design), filepath.Ext(r.Design))
		destDir := filepath.Join(outDir, stem)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", fmt.Errorf("--render: %w", err)
		}
		wrote := 0
		for _, sheet := range g.Sheets {
			if !render.HasHighlights(g, sheet, specs) {
				continue // this sheet carries none of the design's findings
			}
			svg := render.SheetSVGHighlighted(g, sheet, specs)
			out := filepath.Join(destDir, sheetFileName(sheet)+".svg")
			if err := os.WriteFile(out, []byte(svg), 0o644); err != nil {
				return "", fmt.Errorf("--render write %s: %w", out, err)
			}
			wrote++
		}
		lines = append(lines, fmt.Sprintf("  %s: %d finding(s) on %d sheet(s) -> %s/", r.Design, len(specs), wrote, destDir))
	}
	if len(lines) == 0 {
		return "no findings to render (every design passed).\n", nil
	}
	return "rendered annotated review images:\n" + strings.Join(lines, "\n") + "\n", nil
}

// reviewReportFindings flattens every finding across a report's areas and items. Findings appear
// on fail / provisional items, so this is exactly the design's flagged evidence.
func reviewReportFindings(r review.Report) []check.Finding {
	var out []check.Finding
	for _, a := range r.Areas {
		for _, it := range a.Items {
			out = append(out, it.Findings...)
		}
	}
	return out
}

// findingSpecs maps findings to highlight specs, deduplicated: a single rule finding can bind to
// several review items (so appear several times), and one net/component highlighted once is enough.
// A net draws as a PATH marker along its wire (carrying its per-instance id so same-named nets stay
// distinct), a component or pin as a bounding box; severity picks the color. Same vocabulary as the
// render-highlight example and the web click-to-locate.
func findingSpecs(findings []check.Finding) []*geom.HighlightSpec {
	seen := map[string]bool{}
	var specs []*geom.HighlightSpec
	for _, f := range findings {
		key := f.Kind + "\x00" + f.Subject + "\x00" + f.Pin + "\x00" + f.NetID
		if seen[key] {
			continue
		}
		seen[key] = true
		spec := &geom.HighlightSpec{Color: severityColor(f.Severity)}
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

// severityColor maps a finding severity to a highlight color: error hot red, warning amber, else
// the renderer default (empty lets render pick DefaultHighlightColor).
func severityColor(severity string) string {
	switch severity {
	case "error":
		return "#e11d48"
	case "warning":
		return "#f59e0b"
	default:
		return ""
	}
}

// sheetFileName makes a filesystem-safe base name for a sheet: its id (or name, or "sheet"),
// with path separators and spaces folded so a hierarchical id like "/amp1/in" stays one file.
func sheetFileName(sheet *geom.SheetGeometry) string {
	name := sheet.GetId()
	if name == "" {
		name = sheet.GetName()
	}
	if name == "" {
		name = "sheet"
	}
	repl := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return strings.Trim(repl.Replace(name), "_")
}
