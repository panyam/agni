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
// findings. It draws on the first of: an explicit --companion file, a sibling <stem>.eds next to a
// netlist design (joined to the netlist BY NET NAME, WS1-047), the design's own faithful geometry,
// or the default auto-layout. Every finding's subject becomes a highlight, and it writes one SVG per
// sheet the findings land on, to <outDir>/<design-stem>/<sheet>.svg, as the report-side twin of the
// web click-to-locate. Designs with no findings are skipped, a per-design failure is reported and
// skipped, and a companion whose net names poorly overlap the design's is flagged as likely
// mis-paired. Returns a human summary.
func renderReviewImages(reports []review.Report, sources []string, outDir, companionFlag string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("--render %s: %w", outDir, err)
	}
	reg, err := buildRegistry(nil, "")
	if err != nil {
		return "", err
	}
	l := newLoader()

	var lines []string
	for i, r := range reports {
		specs := findingSpecs(reviewReportFindings(r))
		if len(specs) == 0 {
			continue // nothing flagged, nothing to draw
		}
		comp, err := companionPath(r.Design, companionFlag, len(reports))
		if err != nil {
			return "", err // an explicit --companion misuse is a user error, not a per-design skip
		}
		// r.Design is the report's READING name, for a person. The file to open is the design's URI
		// from sources, since only that can be handed to a loader (agni issue 177).
		src := r.Design
		if i < len(sources) {
			src = localOf(sources[i])
		}
		g, warn, err := reviewGeometry(l, reg, src, comp)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  %s: skipped (%v)", r.Design, err))
			continue
		}
		stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
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
// geometry or auto-layout). It looks at filenames only and never reads a file's contents.
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
// the design's own faithful geometry, else the default auto-layout. The warning is advisory, so a
// mismatched companion still renders and the caller surfaces the caveat.
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
// returning a warning when the overlap is below companionOverlapFloor, which suggests the two are
// different-revision or mismatched exports (cf. the WS9-007 overlay alignment check). The warning
// carries only the overlap fraction and never a net name. An empty result means the pairing looks
// consistent.
func companionAlignment(l *formats.Loader, designPath string, g *geom.SchematicGeometry) string {
	d, err := l.ReadDesign(designPath)
	if err != nil {
		return "" // the design side is unreadable, so neither block nor warn
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

// reviewReportFindings flattens every finding across a report's areas and items. Only fail and
// provisional items carry findings, so this is the design's flagged evidence.
func reviewReportFindings(r review.Report) []check.Finding {
	var out []check.Finding
	for _, a := range r.Areas {
		for _, it := range a.Items {
			out = append(out, it.Findings...)
		}
	}
	return out
}

// findingSpecs maps findings to highlight specs, deduplicated because one rule finding can bind to
// several review items. A net draws as a PATH marker along its wire, carrying its per-instance id so
// same-named nets stay distinct, and a component or pin as a bounding box. Severity picks the color,
// in the same vocabulary as the render-highlight example and the web click-to-locate.
func findingSpecs(findings []check.Finding) []*geom.HighlightSpec {
	seen := map[string]bool{}
	var specs []*geom.HighlightSpec
	for _, f := range findings {
		e := f.Subject
		key := e.Kind + "\x00" + e.Ref + "\x00" + e.Pin + "\x00" + e.NetID
		if seen[key] {
			continue
		}
		seen[key] = true
		spec := &geom.HighlightSpec{Color: severityColor(f.Severity)}
		switch e.Kind {
		case check.KindNet:
			spec.Nets = []string{e.Ref}
			if e.NetID != "" {
				spec.NetIds = []string{e.NetID}
			}
			spec.Shape = geom.HighlightShape_HIGHLIGHT_SHAPE_PATH
		case check.KindPin:
			spec.Pins = []*geom.PinRef{{RefDes: e.Ref, Pin: e.Pin}}
		default: // KindComponent
			spec.Components = []string{e.Ref}
			spec.Shape = geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT
		}
		specs = append(specs, spec)
	}
	return specs
}

// severityColor maps a finding severity to a highlight color, red for error and amber for warning.
// Anything else returns "", which lets render pick DefaultHighlightColor.
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

// sheetFileName makes a filesystem-safe base name for a sheet from its id, else its name, else
// "sheet", folding path separators and spaces so a hierarchical id like "/amp1/in" stays one file.
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
