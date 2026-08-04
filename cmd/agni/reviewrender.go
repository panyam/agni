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

// companionOverlapFloor is the minimum fraction of a companion's named wires that must be real
// design nets before its geometry is trusted as the SAME design's drawing. Below it, the companion
// is likely a different-revision or mismatched export and a mis-highlight is worse than none.
const companionOverlapFloor = 0.5

// renderReviewImages writes an annotated schematic image for each design in the review that has
// findings. The geometry it draws on is, in order: an explicit --companion file; else a sibling
// <stem>.eds next to a netlist design (WS1-047: the netlist is analysis truth, a companion schematic
// is the drawing, joined BY NET NAME); else the design's own faithful geometry; else the default
// auto-layout. It bakes every finding's subject as a highlight and writes one SVG per sheet the
// findings land on, to <outDir>/<design-stem>/<sheet>.svg — the report-side twin of the web
// click-to-locate. Designs with no findings are skipped; a per-design failure is reported and
// skipped, never fatal. When a companion's net names poorly overlap the design's, it is flagged
// (likely mis-paired) rather than silently mis-highlighted. Returns a human summary.
func renderReviewImages(reports []review.Report, outDir, companionFlag string) (string, error) {
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
		comp, err := companionPath(r.Design, companionFlag, len(reports))
		if err != nil {
			return "", err // an explicit --companion misuse is a user error, not a per-design skip
		}
		g, warn, err := reviewGeometry(l, reg, r.Design, comp)
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
		on := ""
		if comp != "" {
			on = fmt.Sprintf(" [companion %s]", filepath.Base(comp))
		}
		lines = append(lines, fmt.Sprintf("  %s: %d finding(s) on %d sheet(s) -> %s/%s", r.Design, len(specs), wrote, destDir, on))
		if warn != "" {
			lines = append(lines, "    ! "+warn)
		}
	}
	if len(lines) == 0 {
		return "no findings to render (every design passed).\n", nil
	}
	return "rendered annotated review images:\n" + strings.Join(lines, "\n") + "\n", nil
}

// companionPath resolves the geometry companion for one design: an explicit --companion file (valid
// only with a single design, and it must exist), else an auto-detected sibling <stem>.eds next to a
// NETLIST design (a design that already draws itself needs none), else "" (use the design's own
// geometry / auto-layout). Deterministic and filename-only — it never reads a file's contents.
func companionPath(designPath, flag string, nDesigns int) (string, error) {
	if flag != "" {
		if nDesigns > 1 {
			return "", fmt.Errorf("--companion applies to a single design; with several designs, omit it and a sibling .eds is auto-detected per design")
		}
		if _, err := os.Stat(flag); err != nil {
			return "", fmt.Errorf("--companion %s: %w", flag, err)
		}
		return flag, nil
	}
	if formats.HasFaithful(designPath) {
		return "", nil // the design already carries its own drawing
	}
	sib := strings.TrimSuffix(designPath, filepath.Ext(designPath)) + ".eds"
	if sib == designPath {
		return "", nil
	}
	if _, err := os.Stat(sib); err == nil {
		return sib, nil
	}
	return "", nil
}

// reviewGeometry loads the geometry to annotate for one design: a companion's faithful geometry when
// one is resolved (with an alignment warning when its net names poorly overlap the design's), else
// the design's own faithful geometry, else the default auto-layout. The warning is advisory — a
// mismatched companion still renders, but the caller surfaces the caveat.
func reviewGeometry(l *formats.Loader, reg *graph.Registry, designPath, companion string) (*geom.SchematicGeometry, string, error) {
	if companion != "" {
		g, err := l.FaithfulGeometry(companion)
		if err != nil {
			return nil, "", fmt.Errorf("companion %s: %w", companion, err)
		}
		return g, companionAlignment(l, designPath, g), nil
	}
	layout := graph.DefaultStrategy
	if formats.HasFaithful(designPath) {
		layout = formats.LayoutFaithful
	}
	g, err := l.ResolveGeometry(designPath, layout, reg, symbolsGlyph)
	return g, "", err
}

// companionAlignment measures how many of the companion's named wires are real nets of the design,
// returning a warning when the overlap is below companionOverlapFloor — the signal that the two are
// different-revision or mismatched exports (cf. the WS9-007 overlay alignment check). It reads the
// design's netlist through the loader (the ENGINE parses it); it never surfaces net names, only the
// overlap fraction. An empty result means the pairing looks consistent.
func companionAlignment(l *formats.Loader, designPath string, g *geom.SchematicGeometry) string {
	d, err := l.ReadDesign(designPath)
	if err != nil {
		return "" // cannot check the design side: do not block, do not false-warn
	}
	designNets := map[string]bool{}
	for _, n := range d.GetNets() {
		if n.GetName() != "" {
			designNets[n.GetName()] = true
		}
	}
	compNets := map[string]bool{}
	for _, sh := range g.GetSheets() {
		for _, w := range sh.GetWires() {
			if w.GetNet() != "" {
				compNets[w.GetNet()] = true
			}
		}
	}
	if len(compNets) == 0 {
		return "companion carries no named wires; net-subject findings cannot locate on it"
	}
	matched := 0
	for name := range compNets {
		if designNets[name] {
			matched++
		}
	}
	if frac := float64(matched) / float64(len(compNets)); frac < companionOverlapFloor {
		return fmt.Sprintf("companion net-name overlap %.0f%% (%d/%d) is low — likely a different-revision or mismatched export",
			frac*100, matched, len(compNets))
	}
	return ""
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
