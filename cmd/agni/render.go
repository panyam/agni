package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/core/graph"
	"github.com/panyam/agni/core/render"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// The layout/--symbols vocabulary is shared with the service tier via formats.
const (
	faithfulLayout  = formats.LayoutFaithful
	symbolsGlyph    = formats.SymbolsGlyph
	symbolsFaithful = formats.SymbolsFaithful
)

// renderLoader is the reader for a render, carrying the design's PROJECT config.
//
// `agni render` built its loader with no options at all, so a project's declared symbol library
// never reached the drawing however that project was configured, and --symbol-path was the only
// route to one. An unresolved symbol keeps its placement's reference designator and loses its pins,
// so the sheet draws every ref des and no body, and every component on it is unpickable.
//
// This is the same bypass agni issue 228 closed for the six netlist commands, surviving here because
// readDesign is a netlist function and nothing was the equivalent choke point for geometry
// (agni issue 347).
func renderLoader(file string) (*formats.Loader, error) {
	opts, err := designReadOptions(context.Background(), file)
	if err != nil {
		return nil, err
	}
	return readerFor(newLoader(), opts...), nil
}

// renderSource resolves the artifact a render should DRAW, and the note to say so.
//
// A design already declares where its sheets live. `service.DesignSources.GeometryURI` exists for
// exactly this ("where schematic sheets are rendered and findings located from"), and `render` was
// the one command that never asked for it: it passed the user's path straight to the loader. So
// `agni stats designs/gateway` read the folder and reported "sheets from gateway.kicad_sch", while
// `agni render designs/gateway` failed with `no reader for "" files`, having tried to open a
// directory as a design file. The descriptor had said where to look the whole time.
//
// Falls back to the path as named whenever nothing resolves, which is the ordinary loose-file case,
// and whenever the resolver errors, because a render is not the place to fail on somebody else's
// descriptor when the named file may be perfectly drawable.
func renderSource(path string) (string, string) {
	ws, err := workspace()
	if err != nil {
		return path, ""
	}
	src, err := newDesignResolver(ws).Resolve(context.Background(), path)
	if err != nil || src.GeometryURI == "" {
		return path, ""
	}
	geom := localOf(src.GeometryURI)
	if geom == "" {
		return path, ""
	}
	// Compare as absolute paths. localOf answers absolutely whether or not anything was redirected,
	// so a string compare against the named path calls every loose file a redirect and hands the
	// loader an absolute path the user never typed, which then shows up in its error messages.
	if abs, err := filepath.Abs(path); err == nil && abs == geom {
		return path, ""
	}
	return geom, src.Note
}

// renderCmd renders a design to a schematic view. Two orthogonal axes: --layout is the source
// of node positions (faithful ingested geometry, or an auto-layout computed from the netlist
// IR) and --format is the output encoding (svg or the WebGL pack). Both feed the same
// backend-neutral render layer. The two accept different file types: faithful needs ingested
// geometry (an EDIF .eds, a KiCad .kicad_sch, or whatever a design declares as its schematic
// companion), while auto-layout needs an IR, which a bare .eds does not carry. The errors say which
// to use.
func renderCmd() *cobra.Command {
	var layout, format, sheetSel, out, classFile, symbols, reportFormat string
	var compare, stats, pinDots, report bool
	var classFlags, highlightFlags []string
	cmd := &cobra.Command{
		Use:   "render <file>",
		Short: "Render a design: faithful schematic geometry, or an auto-laid-out netlist graph",
		Long: "render draws a design's schematic view.\n\n" +
			"--layout selects where node positions come from: 'faithful' (default) renders the\n" +
			"design's own ingested geometry, and an auto-layout (" + strings.Join(layoutNames(), ", ") + ")\n" +
			"computes positions from the netlist IR, which works for every format agni reads.\n" +
			"--format selects the output: svg (default) or pack (a PackedSheet for the WebGL viewer).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The path as NAMED stays the analysis source, because readDesign resolves it itself and
			// a design's entry is what analysis reads. `file` is what gets DRAWN, which is a
			// different question and the one the descriptor's geometry companion answers.
			named := args[0]
			file, note := renderSource(named)
			if note != "" {
				fmt.Fprint(cmd.ErrOrStderr(), note) // already prefixed and terminated by the resolver
			}
			// --compare benchmarks every auto-layout on this design and prints their quality
			// scores side by side, so choosing a layout is a comparison, not a guess. No render.
			if compare {
				d, err := readDesign(named)
				if err != nil {
					return err
				}
				// When the design also has faithful geometry, score layouts against it.
				var truth map[string]*geom.Point
				rl, err := renderLoader(file)
				if err != nil {
					return err
				}
				if fg, err := rl.FaithfulGeometry(file); err == nil {
					truth = graph.PositionsByRef(fg)
				}
				return compareLayouts(os.Stdout, d, truth)
			}
			if symbols != symbolsGlyph && symbols != symbolsFaithful {
				return fmt.Errorf("--symbols %q is not one of: %s, %s", symbols, symbolsGlyph, symbolsFaithful)
			}
			// --highlight bakes finding-locating overlays (a net's wire, a component's outline, a
			// pin) into the rendered SVG, so a static picture carries its annotations. Parse early
			// so a malformed spec fails before any load, and gate it to the SVG schematic path.
			specs, err := parseHighlightSpecs(highlightFlags)
			if err != nil {
				return err
			}
			if len(specs) > 0 && format != "svg" {
				return fmt.Errorf("--highlight is supported for --format svg only (got %q)", format)
			}
			reg, err := buildRegistry(classFlags, classFile)
			if err != nil {
				return err
			}
			// --report explains how the auto-layout maps each component to a drawn node (device
			// class, glyph/box, or a provided/unresolved symbol), instead of rendering.
			if report {
				return writeReport(os.Stdout, file, symbols, reg, reportFormat)
			}
			// The faithful drawing of a board file IS the board (WS7-034): when the format
			// carries the board sidecar and no auto-layout was requested, render it. An
			// explicit auto-layout still draws the netlist graph.
			if layout == faithfulLayout && format == "svg" && !stats {
				if b, err := newLoader().BoardGeometry(file); err == nil && b != nil {
					if len(specs) > 0 {
						return fmt.Errorf("--highlight is not yet supported for a board render; it covers schematic sheets (WS7-043)")
					}
					return writeBoard(out, b)
				}
			}
			rl, err := renderLoader(file)
			if err != nil {
				return err
			}
			g, err := rl.ResolveGeometry(file, layout, reg, symbols)
			if err != nil {
				return err
			}
			if len(g.Sheets) == 0 {
				return fmt.Errorf("no sheets in %s", file)
			}
			// Say what this render could not draw, BEFORE writing it. A render that lost its symbols
			// still produces a complete-looking sheet: every ref des, every wire and the title block
			// draw, and only the component bodies are missing, so nothing on the page says the drawing
			// is short (agni issue 354).
			noteUndrawn(cmd.ErrOrStderr(), g)
			if stats {
				return writeGeometryStats(os.Stdout, g)
			}
			sheet, err := render.PickSheet(g, sheetSel)
			if err != nil {
				return err
			}
			// For an auto-layout, report the quality yardstick (crossings especially) so the
			// choice is visible even when the render goes to a file or a pipe.
			if layout != faithfulLayout {
				q := graph.Measure(g)
				fmt.Fprintf(os.Stderr, "layout %q: %d nodes, %d nets, %d segments, %d crossings, edge length %.0f\n",
					layout, q.Nodes, q.Nets, q.Segments, q.Crossings, q.TotalEdgeLen)
			}
			var opts []render.Option
			if pinDots {
				opts = append(opts, render.WithPinDots())
			}
			return writeRender(out, g, sheet, format, specs, opts...)
		},
	}
	cmd.Flags().StringVar(&layout, "layout", faithfulLayout,
		"position source: faithful | "+strings.Join(layoutNames(), " | "))
	cmd.Flags().StringVar(&format, "format", "svg", "output format: svg | pack (png: not yet)")
	cmd.Flags().StringVar(&sheetSel, "sheet", "0", "sheet to render: index, id, or name")
	cmd.Flags().BoolVar(&compare, "compare", false, "print a layout-quality table across all auto-layouts, then exit")
	cmd.Flags().BoolVar(&stats, "stats", false, "print geometry counts instead of rendering")
	cmd.Flags().BoolVar(&pinDots, "pin-dots", false, "draw a verification dot at every pin (off: faithful render, like Eeschema)")
	cmd.Flags().StringVarP(&out, "out", "o", "-", "output file (- for stdout)")
	cmd.Flags().StringArrayVar(&classFlags, "class", nil, "auto-layout device-class override, repeatable: <symbol-glob>=<class> (e.g. my_res=resistor)")
	cmd.Flags().StringVar(&classFile, "class-file", "", "file of <symbol-glob>=<class> lines layered onto the default device classification")
	cmd.Flags().StringVar(&symbols, "symbols", symbolsGlyph, "auto-layout node artwork: glyph (classified synthetic symbols) | faithful (the design's own symbols, re-laid-out)")
	cmd.Flags().BoolVar(&report, "report", false, "print how each component maps to a drawn node (device class, glyph/box/provided/unresolved) instead of rendering")
	cmd.Flags().StringVar(&reportFormat, "report-format", "text", "--report output: text | json")
	cmd.Flags().StringArrayVar(&highlightFlags, "highlight", nil,
		"bake a finding-locating overlay into the SVG, repeatable. One subject per flag, comma-separated:\n"+
			"net=<name> | ref=<refdes> | pin=<refdes>:<pin>, plus optional shape=outline|rect|circle|path, "+
			"color=#rrggbb, alpha=0..1 (e.g. --highlight net=SCL,shape=path,color=#e11)")
	return cmd
}

// parseHighlightSpecs turns each --highlight value into one geom.HighlightSpec. A value is a
// comma-separated list of key=value pairs naming exactly one subject (net / ref / pin) plus
// optional style keys (shape, color, alpha). The subject shape defaults sensibly per kind — a net
// draws as a PATH marker along its wire, a component/pin as an OUTLINE — matching the web
// click-to-locate defaults, so the CLI static picture reads like the interactive one.
func parseHighlightSpecs(flags []string) ([]*geom.HighlightSpec, error) {
	var specs []*geom.HighlightSpec
	for _, raw := range flags {
		spec := &geom.HighlightSpec{}
		var subjectKind string
		shapeSet := false
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				return nil, fmt.Errorf("--highlight %q: %q is not key=value", raw, part)
			}
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			switch k {
			case "net":
				if v == "" {
					return nil, fmt.Errorf("--highlight %q: net= needs a name", raw)
				}
				spec.Nets = append(spec.Nets, v)
				subjectKind = "net"
			case "ref", "component":
				if v == "" {
					return nil, fmt.Errorf("--highlight %q: %s= needs a ref-des", raw, k)
				}
				spec.Components = append(spec.Components, v)
				subjectKind = "component"
			case "pin":
				ref, pin, ok := strings.Cut(v, ":")
				if !ok || ref == "" || pin == "" {
					return nil, fmt.Errorf("--highlight %q: pin must be <refdes>:<pin>, got %q", raw, v)
				}
				spec.Pins = append(spec.Pins, &geom.PinRef{RefDes: ref, Pin: pin})
				subjectKind = "pin"
			case "shape":
				sh, err := parseHighlightShape(v)
				if err != nil {
					return nil, fmt.Errorf("--highlight %q: %w", raw, err)
				}
				spec.Shape = sh
				shapeSet = true
			case "color":
				spec.Color = v
			case "alpha":
				a, err := strconv.ParseFloat(v, 32)
				if err != nil || a < 0 || a > 1 {
					return nil, fmt.Errorf("--highlight %q: alpha must be a number in [0,1], got %q", raw, v)
				}
				spec.Alpha = float32(a)
			default:
				return nil, fmt.Errorf("--highlight %q: unknown key %q (want net|ref|pin|shape|color|alpha)", raw, k)
			}
		}
		if subjectKind == "" {
			return nil, fmt.Errorf("--highlight %q names no subject (need one of net=, ref=, pin=)", raw)
		}
		if !shapeSet && subjectKind == "net" {
			spec.Shape = geom.HighlightShape_HIGHLIGHT_SHAPE_PATH
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// parseHighlightShape maps the CLI shape word to the proto enum. Unset draws as OUTLINE.
func parseHighlightShape(v string) (geom.HighlightShape, error) {
	switch v {
	case "outline", "":
		return geom.HighlightShape_HIGHLIGHT_SHAPE_UNSPECIFIED, nil
	case "rect":
		return geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_RECT, nil
	case "circle":
		return geom.HighlightShape_HIGHLIGHT_SHAPE_BOUNDING_CIRCLE, nil
	case "path":
		return geom.HighlightShape_HIGHLIGHT_SHAPE_PATH, nil
	default:
		return 0, fmt.Errorf("unknown shape %q (want outline|rect|circle|path)", v)
	}
}

// noteUndrawn reports the placements a render could not draw, to stderr so a redirected SVG or a
// `--format json` document on stdout stays clean.
//
// It names the missing LIBRARIES rather than listing every placement, because one missing file
// commonly costs every part drawn from it and forty identical lines bury that single cause. The ref
// des count is the blast radius, which is the number a reader needs to judge whether the drawing is
// worth reading at all.
//
// Silent when nothing is undrawn. A note on every complete render is a note nobody reads.
func noteUndrawn(w io.Writer, g *geom.SchematicGeometry) {
	u := g.GetUndrawn()
	if len(u) == 0 {
		return
	}
	byLib := map[string]int{}
	var order []string
	for _, p := range u {
		want := p.GetCellRef()
		if lib := p.GetLibraryRef(); lib != "" {
			want = lib + ":" + want
		}
		if _, seen := byLib[want]; !seen {
			order = append(order, want)
		}
		byLib[want]++
	}
	sort.Strings(order)
	total := 0
	for _, sh := range g.GetSheets() {
		total += len(sh.GetPlacements())
	}
	fmt.Fprintf(w, "warning: %d of %d placements have no symbol and were not drawn; their reference designators still appear\n", len(u), total)
	for _, want := range order {
		fmt.Fprintf(w, "  %s (%d placement(s))\n", want, byLib[want])
	}
	fmt.Fprintf(w, "  resolve with --symbol-path <dir>, a sym-lib-table, or the design descriptor's `symbols` key\n")
}

// writeReport builds the conversion report for the file's netlist under the chosen symbol source
// and writes it as text (a grouped summary + the unmapped/unresolved call-outs) or JSON.
func writeReport(w io.Writer, file, symbols string, reg *graph.Registry, format string) error {
	rl, err := renderLoader(file)
	if err != nil {
		return err
	}
	rep, err := rl.ConversionReport(file, symbols, reg)
	if err != nil {
		return err
	}
	name := filepath.Base(file)
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	case "text", "":
		writeReportText(w, name, symbols, rep)
		return nil
	default:
		return fmt.Errorf("unknown --report-format %q (want: text, json)", format)
	}
}

// writeReportText renders the report as a grouped human summary: a line per class/kind with its
// count and ref-des, then explicit call-outs for the unmapped (box) and unresolved components,
// the latter pointing at --symbol-path.
func writeReportText(w io.Writer, design, symbols string, rep *graph.ConversionReport) {
	fmt.Fprintf(w, "%s — %s symbols · %d components\n", design, symbols, len(rep.Components))

	// Group by label (device class for glyphs, else the kind), preserving first-seen order.
	labels := []string{}
	refs := map[string][]string{}
	for _, c := range rep.Components {
		l := c.Label()
		if _, seen := refs[l]; !seen {
			labels = append(labels, l)
		}
		refs[l] = append(refs[l], c.RefDes)
	}
	sort.Strings(labels)
	for _, l := range labels {
		fmt.Fprintf(w, "  %-12s %3d  %s\n", l, len(refs[l]), strings.Join(refs[l], " "))
	}

	if box := rep.RefsByKind(graph.KindBox); len(box) > 0 {
		fmt.Fprintf(w, "%d unmapped (generic box, no device glyph): %s\n", len(box), strings.Join(box, " "))
	}
	if un := rep.RefsByKind(graph.KindUnresolved); len(un) > 0 {
		fmt.Fprintf(w, "%d unresolved symbols — pass --symbol-path to draw the design's own symbols: %s\n", len(un), strings.Join(un, " "))
	}
}

// buildRegistry layers user classification rules (from --class flags and a --class-file) on top
// of the default registry, so a user can steer how auto-layout draws their parts without a code
// change (WS7-030). Each rule is "<symbol-glob>=<class>"; the class must be one the registry can
// draw. File and flag parsing happens here at the CLI edge so the graph core stays pure (C1).
func buildRegistry(classFlags []string, classFile string) (*graph.Registry, error) {
	reg := graph.DefaultRegistry()
	var lines []string
	if classFile != "" {
		b, err := os.ReadFile(classFile)
		if err != nil {
			return nil, err
		}
		lines = append(lines, strings.Split(string(b), "\n")...)
	}
	// Flags after file lines so a flag rule, prepended last, wins over the file (both beat defaults).
	lines = append(lines, classFlags...)

	known := map[string]bool{}
	for _, c := range reg.GlyphClasses() {
		known[c] = true
	}
	var rules []graph.ClassRule
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sym, class, ok := strings.Cut(line, "=")
		sym, class = strings.TrimSpace(sym), strings.TrimSpace(class)
		if !ok || sym == "" || class == "" {
			return nil, fmt.Errorf("bad class rule %q, want <symbol-glob>=<class>", line)
		}
		if !known[class] {
			return nil, fmt.Errorf("unknown class %q in rule %q (have: %s)", class, line, strings.Join(reg.GlyphClasses(), ", "))
		}
		rules = append(rules, graph.ClassRule{Class: class, Symbol: sym})
	}
	return reg.With(rules...), nil
}

// writeRender renders one sheet in the requested format to out (- or empty is stdout),
// printing a status note to stderr on a file write.
// writeBoard writes the board SVG (WS7-034) to out (or stdout), the board face of
// writeRender.
func writeBoard(out string, b *geom.BoardGeometry) error {
	w := os.Stdout
	if out != "" && out != "-" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	doc := render.BoardSVG(b)
	if _, err := io.WriteString(w, doc); err != nil {
		return err
	}
	if out != "" && out != "-" {
		fmt.Fprintf(os.Stderr, "wrote %s (board, %d placements, %d nets)\n", out, len(b.GetPlacements()), len(b.GetNets()))
	}
	return nil
}

func writeRender(out string, g *geom.SchematicGeometry, sheet *geom.SheetGeometry, format string, specs []*geom.HighlightSpec, opts ...render.Option) error {
	w := os.Stdout
	if out != "" && out != "-" {
		f, err := os.Create(out)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	note, err := renderGeometry(w, g, sheet, format, specs, opts...)
	if err != nil {
		return err
	}
	if out != "" && out != "-" {
		fmt.Fprintf(os.Stderr, "wrote %s (%s)\n", out, note)
	}
	return nil
}

// renderGeometry writes one sheet to w in the given format and returns a one-line status note.
// svg is the verification backend; pack is a tier-2 PackedSheet proto for the WebGL viewer
// (CONSTRAINTS C8). png is reserved but not implemented, and an unknown format errors.
func renderGeometry(w io.Writer, g *geom.SchematicGeometry, sheet *geom.SheetGeometry, format string, specs []*geom.HighlightSpec, opts ...render.Option) (string, error) {
	switch format {
	case "svg":
		doc := render.SheetSVG(g, sheet, opts...)
		note := fmt.Sprintf("sheet %q, %d placements, %d wires", sheet.Name, len(sheet.Placements), len(sheet.Wires))
		if len(specs) > 0 {
			doc = render.SheetSVGHighlighted(g, sheet, specs, opts...)
			note += fmt.Sprintf(", %d highlight(s)", len(specs))
		}
		if _, err := io.WriteString(w, doc); err != nil {
			return "", err
		}
		return note, nil
	case "pack":
		blob, err := proto.Marshal(render.PackSheet(g, sheet, opts...))
		if err != nil {
			return "", err
		}
		if _, err := w.Write(blob); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d bytes, sheet %q", len(blob), sheet.Name), nil
	case "png":
		return "", fmt.Errorf("png output is not implemented yet (available: svg, pack)")
	default:
		return "", fmt.Errorf("unknown format %q (available: svg, pack)", format)
	}
}

// writeGeometryStats prints geometry counts (symbols, sheets, placements, wires, labels) for
// whatever geometry --layout produced, faithful or auto-laid-out.
func writeGeometryStats(w io.Writer, g *geom.SchematicGeometry) error {
	placements, wires, polylines, labels := 0, 0, 0, 0
	for _, s := range g.Sheets {
		placements += len(s.Placements)
		wires += len(s.Wires)
		labels += len(s.Labels)
		for _, wg := range s.Wires {
			polylines += len(wg.Polylines)
		}
	}
	fmt.Fprintf(w, "design_ref:   %s\n", g.DesignRef)
	fmt.Fprintf(w, "unit_nm:      %d\n", g.UnitNm)
	fmt.Fprintf(w, "symbols:      %d (library)\n", len(g.Symbols))
	fmt.Fprintf(w, "sheets:       %d\n", len(g.Sheets))
	fmt.Fprintf(w, "placements:   %d\n", placements)
	fmt.Fprintf(w, "wires:        %d (%d polylines)\n", wires, polylines)
	fmt.Fprintf(w, "labels:       %d\n", labels)
	return nil
}

// compareLayouts runs every registered layout on one design and writes a quality table, so
// layouts can be judged by number (crossings especially) rather than by eye.
func compareLayouts(w io.Writer, d *ir.Design, truth map[string]*geom.Point) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	header := "layout\tnodes\tnets\tsegments\tcrossings\tbends\tedge-length\tstress"
	if truth != nil {
		// The design has faithful geometry, so also score each auto-layout against it (how
		// closely the layout reproduces the real placement; lower is closer). See WS7-011.
		header += "\ttruth-residual"
	}
	fmt.Fprintln(tw, header)
	for _, s := range graph.Strategies() {
		g, err := graph.LayoutWith(d, s.Name)
		if err != nil {
			return err
		}
		q := graph.MeasureWith(d, g)
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%d\t%.0f\t%.3f", s.Name, q.Nodes, q.Nets, q.Segments, q.Crossings, q.Bends, q.TotalEdgeLen, q.Stress)
		if truth != nil {
			fmt.Fprintf(tw, "\t%.3f", graph.GroundTruthResidual(graph.PositionsByRef(g), truth))
		}
		fmt.Fprintln(tw)
	}
	return tw.Flush()
}

// layoutNames returns the auto-layout strategy names (excludes faithful, which is not an
// IR-based strategy), for flag help and error text.
func layoutNames() []string {
	all := graph.Strategies()
	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
	}
	return names
}
