// Package xschem reads xschem schematic (.sch) and symbol (.sym) files into the agni IR.
//
// xschem shares the .sch extension with gEDA and legacy KiCad; the CLI disambiguates by
// sniffing the header (an xschem file opens with "v {xschem ..."). See IsXschem.
//
// Fidelity: lossy-bounded. Read extracts the placed components (grouped by their xschem
// instance name, which is the reference designator) and the named nets. xschem wires carry
// their net name inline (N ... {lab=Name}) and net-label symbols (lab_pin/ipin/opin/gnd/vdd)
// name a net at a point, so net *names* come straight from the file. Pin-level membership
// (which Net each component pin joins) needs the referenced .sym pin geometry: ReadWithSymbols
// resolves symbols through a caller-supplied opener and returns a fully connected netlist,
// while plain Read (no opener) returns the nets by name only. sourceFile is recorded in
// provenance only; the caller owns file I/O (CONSTRAINTS C1).
package xschem

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/refdes"
	"github.com/panyam/agni/internal/symread"
)

// sourceFormat is the IR origin tag for xschem-sourced designs.
const sourceFormat = "xschem"

// xschemNativeIDKind labels the provenance native id: an xschem instance name (the value of
// the symbol's name= attribute, e.g. "R5"). Unlike a KiCad uuid it is the human refdes, and
// it is unique within one schematic.
const xschemNativeIDKind = "xschem-name"

// labelSymbols are the pseudo-symbols that name a net at a point rather than being physical
// components: power taps and port/label markers. Their lab= attribute is a net name, anchored
// at the symbol origin (these symbols connect at (0,0) by convention, so no .sym is needed to
// place the anchor). Keyed by the symbol basename without the .sym suffix.
var labelSymbols = map[string]bool{
	"gnd": true, "vdd": true, "vss": true,
	"lab_pin": true, "lab_wire": true,
	"ipin": true, "opin": true, "iopin": true,
}

// externalLabelSymbols are the label symbols whose net leaves this read's scope, so their
// anchor is marked External (WS1-021): the supply taps gnd/vdd/vss (a global rail fed from
// a supply that may lie off the read — KiCad's power-symbol semantics), and the hierarchy
// ports ipin/opin/iopin (the net continues into a parent/child sheet). External keeps the
// power/absence rules from false-firing on rails and ports whose full membership the
// single-sheet read cannot see. Plain net labels (lab_pin/lab_wire) are sheet-local and
// stay unmarked.
var externalLabelSymbols = map[string]bool{
	"gnd": true, "vdd": true, "vss": true,
	"ipin": true, "opin": true, "iopin": true,
}

// annotationSymbols carry no electrical identity: title blocks, embedded SPICE code, probes,
// launchers. The NETLIST reader skips them entirely (neither components nor net anchors).
var annotationSymbols = map[string]bool{
	"title": true, "code": true, "code_shown": true,
	"netlist_commands": true, "netlist_not_shown": true,
	"spice_probe": true, "launcher": true, "use": true,
	"noconn": true, "arrow": true, "text": true,
}

// geomRenderAnnotation is the subset of annotationSymbols the GEOMETRY reader draws faithfully
// (WS7-037): the title block and the visible SPICE-code blocks (A1/A2/A3), which carry real
// on-sheet content. The rest of annotationSymbols (spice_probe hooks on every net, launcher,
// use, arrow, noconn, netlist_*) stay skipped in geometry too — rendering them all is visual
// noise. The netlist reader still skips every annotationSymbol regardless.
var geomRenderAnnotation = map[string]bool{
	"title": true, "code": true, "code_shown": true,
}

// SymbolOpener resolves a symbol reference from a component's C line (e.g. "res.sym" or
// "devices/res.sym") to the raw bytes of that .sym file. It returns an error when the symbol
// cannot be found; ReadWithSymbols treats an unresolved symbol as "component present, pins
// unknown" and carries on. The caller owns the search path and file I/O (CONSTRAINTS C1).
type SymbolOpener func(symref string) ([]byte, error)

// IsXschem reports whether the first bytes look like an xschem file: a leading "v {xschem".
// Comment (*) and blank lines before the header are tolerated.
func IsXschem(head []byte) bool {
	for _, ln := range strings.Split(string(head), "\n") {
		ln = strings.TrimLeft(ln, " \t")
		if ln == "" || strings.HasPrefix(ln, "*") {
			continue
		}
		return strings.HasPrefix(ln, "v {xschem") || strings.HasPrefix(ln, "v{xschem")
	}
	return false
}

// Read parses an xschem schematic into an ir.Design with nets named but not pin-connected.
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	return read(r, sourceFile, nil)
}

// ReadWithSymbols parses an xschem schematic and resolves component pins through open,
// producing a fully connected netlist. Symbols that open cannot resolve leave their component
// present but contribute no pins.
func ReadWithSymbols(r io.Reader, sourceFile string, open SymbolOpener) (*ir.Design, error) {
	return read(r, sourceFile, open)
}

func read(r io.Reader, sourceFile string, open SymbolOpener) (*ir.Design, error) {
	br := bufio.NewReader(r)
	head, _ := br.Peek(256)
	if !IsXschem(head) {
		return nil, fmt.Errorf("xschem: not an xschem file (no \"v {xschem\" header)")
	}
	objs, err := parse(br)
	if err != nil {
		return nil, err
	}
	return extract(objs, sourceFile, open), nil
}

// placement is a component instance to be pin-resolved: its symbol, grid transform, and refdes.
// extract turns the parsed object stream into a Design: the part-type library, the components,
// and the nets. Wires and label anchors are collected during the walk; component placements are
// resolved to pins afterward (when an opener is supplied) and handed to netgraph.
func extract(objs []object, src string, open SymbolOpener) *ir.Design {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: sourceFormat,
		Attributes:   map[string]string{},
		Prov:         &ir.Provenance{SourceFile: src},
	}
	lib := &ir.PartLibrary{Name: "xschem", Prov: &ir.Provenance{SourceFile: src}}
	partByName := map[string]*ir.PartType{}

	var wires []netgraph.Wire
	var anchors []netgraph.Anchor
	var placements []symread.Placement
	// A `lab=NAME[n:0]` label is a bus; xschem does not expand its members, so each distinct bus
	// label is recorded for the bus-not-modeled rule (WS1-034 Phase 1). Deduped by label text.
	var buses []*ir.BusNotModeled
	seenBus := map[string]bool{}
	addBus := func(lab string) {
		if lab != "" && !seenBus[lab] && busLabelRe.MatchString(lab) {
			seenBus[lab] = true
			buses = append(buses, &ir.BusNotModeled{Kind: "xschem_bus_label", Label: lab, Prov: &ir.Provenance{SourceFile: src}})
		}
	}

	for _, o := range objs {
		switch o.typ {
		case 'N':
			p := props(lastBrace(o))
			a, oka := point(o.word(0), o.word(1))
			b, okb := point(o.word(2), o.word(3))
			if oka && okb {
				wires = append(wires, netgraph.Wire{A: a, B: b, Label: p["lab"]})
			}
			addBus(p["lab"])
		case 'C':
			symref := o.braceAt(0)
			sym := symread.SymbolBase(symref)
			x, _ := atoi(o.word(1))
			y, _ := atoi(o.word(2))
			rot := atoiInt(o.word(3))
			flip := atoiInt(o.word(4))
			p := props(lastBraceC(o))
			if labelSymbols[sym] {
				// Net anchor at the symbol origin (label symbols connect at (0,0)).
				anchors = append(anchors, netgraph.Anchor{At: quant(x, y), Label: p["lab"], External: externalLabelSymbols[sym]})
				addBus(p["lab"])
				continue
			}
			if annotationSymbols[sym] {
				continue
			}
			ref := p["name"]
			if ref == "" {
				continue
			}
			part := partByName[sym]
			if part == nil {
				part = &ir.PartType{Name: sym, Kind: "xschem-symbol", Prov: &ir.Provenance{SourceFile: src}}
				partByName[sym] = part
				lib.Parts = append(lib.Parts, part)
			}
			d.Components = append(d.Components, dialect.NewComponent(ref, sym, p, src))
			placements = append(placements, symread.Placement{
				Symref: symref, Ref: ref, Part: part,
				Place: func(px, py float64) (float64, float64) { return transform(px, py, x, y, rot, flip) },
			})
		}
	}

	if len(lib.Parts) > 0 {
		d.Libraries = append(d.Libraries, lib)
	}
	sort.SliceStable(d.Components, func(i, j int) bool { return d.Components[i].RefDes < d.Components[j].RefDes })

	if open != nil {
		pins, unresolved := symread.ResolvePins(placements, loadPins(open), quant)
		nets, dangles, _ := netgraph.Build(wires, anchors, pins, nil)
		d.Nets = append(d.Nets, netgraph.IRNets(nets, src)...)
		// Dangling endpoints are trustworthy only when every placement resolved: a
		// symbol that fails to load from the external .sym library drops its pins, so a
		// wire end meant to land on one reads as a phantom dangle (WS1-013). One
		// unresolved placement suppresses the whole design's dangles — the conservative
		// gate that keeps false positives at zero. Grid points map back to the geometry
		// frame the viewer draws via danglePoint (xschem's netgraph grid is scaled;
		// geometry is native). No per-wire id in these formats, so location is the subject.
		// An unresolved symbol is recorded either way (WS1-052): the suppression above is what
		// makes it invisible, so the two must be emitted together or the read gets quieter with
		// nothing to say why.
		d.InputDiagnostics = &ir.InputDiagnostics{UnresolvedSymbols: irUnresolved(unresolved, src)}
		if len(unresolved) == 0 {
			d.InputDiagnostics.DanglingEndpoints = geomDangles(dangles, src)
		}
	} else {
		for _, n := range symread.NameOnlyNets(wires, anchors) {
			d.Nets = append(d.Nets, &ir.Net{Name: n, Prov: &ir.Provenance{SourceFile: src}})
		}
	}
	// Bus detection is name-based, independent of symbol resolution, so attach after either branch.
	if len(buses) > 0 {
		ensureDiag(d).UnmodeledBuses = buses
	}
	// xschem keeps a placeholder-designated component, so it owes the diagnostic. Unlike gEDA's
	// refdes=R? templates, nothing here attests that xschem SHIPS placeholders: the tool assigns an
	// instance name from the symbol template on placement, so a `?` in one is a name somebody typed.
	// It is recorded anyway because the reader's own assumption depends on it — the instance name is
	// this format's provenance native id and is documented as unique within a schematic (see the
	// extract doc comment), and a shared "R?" breaks that silently. Reporting is how that surfaces.
	if un := refdes.Unannotated(d.Components); len(un) > 0 {
		ensureDiag(d).UnannotatedComponents = un
	}
	return d
}

// ensureDiag returns d's InputDiagnostics, building it on first use. Both signals recorded after
// the symbol-resolution branch attach through it, because the no-opener path leaves the field nil
// and assigning a fresh struct per signal silently drops whatever was recorded before it.
func ensureDiag(d *ir.Design) *ir.InputDiagnostics {
	if d.InputDiagnostics == nil {
		d.InputDiagnostics = &ir.InputDiagnostics{}
	}
	return d.InputDiagnostics
}

// busLabelRe matches an xschem bus label's member-range suffix (`DATA[7:0]`): a bus is drawn with a
// `[hi:lo]` range on its net name. It is detection-only (WS1-034 Phase 1); a scalar indexed net like
// `A[3]` (single element, no colon) is not a bus and does not match.
var busLabelRe = regexp.MustCompile(`\[\d+:\d+\]`)

// dialect carries the xschem constants for the shared component emission
// (internal/symread): library name, provenance id kind, and the section attributes.
var dialect = symread.Dialect{
	Lib:          "xschem",
	NativeIDKind: xschemNativeIDKind,
	SectionAttrs: []string{"value", "device", "model", "footprint"},
}

// lastBrace returns the last brace token's inner text (an N object's {props}, or a component's
// {props} which is its final brace).
func lastBrace(o object) string {
	for i := len(o.tokens) - 1; i >= 0; i-- {
		if o.tokens[i].brace {
			return o.tokens[i].text
		}
	}
	return ""
}

// lastBraceC returns a component object's {props} block, which is the last brace token (the
// first brace is the symref). Returns "" when the instance has a symref but no props.
func lastBraceC(o object) string {
	seen := 0
	for _, t := range o.tokens {
		if t.brace {
			seen++
		}
	}
	if seen < 2 {
		return ""
	}
	return lastBrace(o)
}

// irUnresolved turns the resolver's unresolved references into IR diagnostics, stamping the
// construct kind and source file the resolver does not know. Returns nil for an empty set so a
// clean read carries no empty slice.
func irUnresolved(us []symread.Unresolved, src string) []*ir.UnresolvedSymbol {
	var out []*ir.UnresolvedSymbol
	for _, u := range us {
		out = append(out, &ir.UnresolvedSymbol{
			Symref: u.Symref,
			Kind:   "xschem_sym",
			RefDes: u.RefDes,
			Prov:   &ir.Provenance{SourceFile: src},
		})
	}
	return out
}
