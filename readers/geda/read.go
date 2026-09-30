// Package geda reads gEDA gschem schematic (.sch) and symbol (.sym) files into the agni IR.
//
// gEDA shares the .sch extension with xschem and legacy KiCad; the CLI disambiguates by
// sniffing the header (a gEDA file opens with "v <version> <fileformat>", e.g. "v 20200319 2").
// See IsGeda.
//
// Fidelity: lossy-bounded. Read extracts the placed components (grouped by their refdes
// attribute) and the nets. gEDA net segments are geometric and unlabelled, so pin-level
// membership needs the referenced .sym pin geometry. ReadWithSymbols resolves symbols through a
// caller-supplied opener and returns a connected netlist (nets named from netname= attributes
// that sit on a wire, else synthetic), while plain Read (no opener) returns only the netname=
// nets by name. sourceFile is provenance only (C1).
package geda

import (
	"bufio"
	"io"
	"sort"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
	"github.com/panyam/agni/internal/refdes"
	"github.com/panyam/agni/internal/symread"
)

const sourceFormat = "geda"

// gedaNativeIDKind labels the provenance native id, a gEDA refdes (the value of the
// component's refdes= attribute, e.g. "R5"), unique within one schematic.
const gedaNativeIDKind = "geda-refdes"

// powerSymbols name a power/ground net at a point rather than being physical components.
var powerSymbols = map[string]bool{
	"gnd-1": true, "gnd": true,
	"vcc-1": true, "vcc": true,
	"vdd-1": true, "vdd": true,
	"vss-1": true, "vss": true,
}

// annotationSymbols carry no electrical identity (title blocks and SPICE directive/model/
// include markers). The NETLIST reader skips them entirely.
var annotationSymbols = map[string]bool{
	"title-a": true, "title-b": true, "title-c": true, "title-d": true,
	"spice-directive-1": true, "spice-model-1": true, "spice-include-1": true,
}

// geomRenderAnnotation is the subset the GEOMETRY reader draws (WS7-037). Every gEDA annotation
// symbol carries visible on-sheet content and its field text resolves through normal attribute
// promotion, so the whole set renders.
var geomRenderAnnotation = annotationSymbols

// SymbolOpener resolves a symbol reference from a component's C line (e.g. "resistor-1.sym") to
// the raw bytes of that .sym file. The caller owns the search path and file I/O (C1).
type SymbolOpener func(symref string) ([]byte, error)

// IsGeda reports whether the first bytes look like a gEDA gschem file, meaning a leading "v "
// header whose next field is an all-digit release date (e.g. "v 20200319 2"). An xschem header
// ("v {xschem ...") fails the digit test.
func IsGeda(head []byte) bool {
	for _, ln := range strings.Split(string(head), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		f := strings.Fields(ln)
		if len(f) < 2 || f[0] != "v" {
			return false
		}
		return isAllDigits(f[1])
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Read parses a gEDA schematic into an ir.Design with nets named but not pin-connected.
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	return read(r, sourceFile, nil)
}

// ReadWithSymbols parses a gEDA schematic and resolves component pins through open, producing a
// connected netlist.
func ReadWithSymbols(r io.Reader, sourceFile string, open SymbolOpener) (*ir.Design, error) {
	return read(r, sourceFile, open)
}

func read(r io.Reader, sourceFile string, open SymbolOpener) (*ir.Design, error) {
	lines, err := readLines(r)
	if err != nil {
		return nil, err
	}
	if !IsGeda([]byte(strings.Join(firstN(lines, 3), "\n"))) {
		return nil, errNotGeda
	}
	return extract(lines, sourceFile, open), nil
}

var errNotGeda = &parseError{"geda: not a gEDA gschem file (no \"v <version>\" header)"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func firstN(s []string, n int) []string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// extract walks the gEDA object stream of components (each with attribute block and placement),
// net segments, power taps, and netname= labels. Wires and anchors feed netgraph, and component
// placements are resolved to pins when an opener is supplied.
func extract(lines []string, src string, open SymbolOpener) *ir.Design {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: sourceFormat,
		Attributes:   map[string]string{},
		Prov:         &ir.Provenance{SourceFile: src},
	}
	lib := &ir.PartLibrary{Name: "geda", Prov: &ir.Provenance{SourceFile: src}}
	partByName := map[string]*ir.PartType{}
	compByRef := map[string]*ir.Component{} // fold a multi-gate package's slot instances into one

	var wires []netgraph.Wire
	var buses []*ir.BusNotModeled // gEDA `U` bus segments, detected but not yet expanded (WS1-034)
	var anchors []netgraph.Anchor
	var placements []symread.Placement
	var placementSlots []string // slot= per placement, aligned with placements; resolved to SlotPins after the walk
	var powers []powerTap       // power/ground taps, pin-resolved after the walk
	var labels []textLabel      // standalone netname= texts, placed by proximity after the walk
	var netTaps []netTap        // component net= hidden pin taps, pin-resolved after the walk

	i := 0
	for i < len(lines) {
		f := strings.Fields(lines[i])
		if len(f) == 0 {
			i++
			continue
		}
		switch f[0] {
		case "C":
			cx, _ := atof(field(f, 1))
			cy, _ := atof(field(f, 2))
			angle := atoiInt(field(f, 4))
			mirror := atoiInt(field(f, 5))
			sym := symread.SymbolBase(field(f, 6))
			attrs, nets, next := readAttrBlock(lines, i+1)
			i = next
			if isPowerSymbol(sym) {
				// A power/ground tap connects at its symbol's pin (offset from the origin), so
				// it needs the .sym geometry; resolved after the walk. Its net name comes from
				// the instance net= attribute if present, else the symbol's net= or convention.
				powers = append(powers, powerTap{symref: field(f, 6), x: cx, y: cy, angle: angle, mirror: mirror, instanceNet: firstTapNet(nets)})
				continue
			}
			if annotationSymbols[sym] {
				continue
			}
			ref := attrs["refdes"]
			if ref == "" {
				continue
			}
			// Hidden pin taps. net=NAME:pin[,pin] connects those pins to NAME with no drawn wire
			// (how gEDA carries an IC's power/ground pins), resolved to grid points after the walk.
			for _, spec := range nets {
				name, pinList := parseNetTap(spec)
				for _, pin := range pinList {
					netTaps = append(netTaps, netTap{ref: ref, pin: pin, name: name})
				}
			}
			part := partByName[sym]
			if part == nil {
				part = &ir.PartType{Name: sym, Kind: "geda-symbol", Prov: &ir.Provenance{SourceFile: src}}
				partByName[sym] = part
				lib.Parts = append(lib.Parts, part)
			}
			// A multi-gate package places one C line per gate, all sharing a refdes and carrying
			// slot=. Fold those into one Component with a section per gate, so the ref-des is not a
			// false duplicate. Instances without slot= stay separate, so a real collision still shows.
			slot := attrs["slot"]
			if comp := compByRef[ref]; comp != nil && slot != "" {
				dialect.AddSection(comp, sym, attrs, src)
			} else {
				comp = dialect.NewComponent(ref, sym, attrs, src)
				compByRef[ref] = comp
				d.Components = append(d.Components, comp)
			}
			placements = append(placements, symread.Placement{
				Symref: field(f, 6), Ref: ref, Part: part,
				Place: func(px, py float64) (float64, float64) { return transform(px, py, cx, cy, angle, mirror) },
			})
			placementSlots = append(placementSlots, slot)
		case "N", "U":
			a, oka := point(field(f, 1), field(f, 2))
			b, okb := point(field(f, 3), field(f, 4))
			attrs, _, next := readAttrBlock(lines, i+1)
			i = next
			if field(f, 0) == "U" {
				// A gEDA `U` bus is GRAPHICAL-ONLY for connectivity. lepton-netlist traces signals
				// through the member `netname=` labels on the ripped-off wires, never the bus itself
				// (verified against lepton-netlist, WS1-034 Phase 2). Aliasing the bus to a wire invents
				// a phantom `DATA[7:0]` net and a false single-pin-net finding. The bus is still drawn
				// (geometry.go) and recorded with its range expansion as members, so bus-not-modeled
				// stays silent once every member is a net.
				name := attrs["netname"]
				buses = append(buses, &ir.BusNotModeled{
					Kind:    "geda_bus",
					Label:   name,
					Members: netgraph.ExpandBusName(name),
					Prov:    &ir.Provenance{SourceFile: src},
				})
			} else if oka && okb {
				wires = append(wires, netgraph.Wire{A: a, B: b, Label: attrs["netname"]})
			}
		case "T":
			text, next := readText(lines, i)
			i = next
			if k, v, ok := splitAttr(text); ok && k == "netname" {
				if p, okp := point(field(f, 1), field(f, 2)); okp {
					labels = append(labels, textLabel{at: p, name: v})
				}
			}
		case "H":
			_, next := readText(lines, i)
			i = next
		default:
			i++
		}
	}

	if len(lib.Parts) > 0 {
		d.Libraries = append(d.Libraries, lib)
	}
	sort.SliceStable(d.Components, func(i, j int) bool { return d.Components[i].RefDes < d.Components[j].RefDes })

	// A standalone netname= text names the net whose wire endpoint it sits nearest to.
	anchors = append(anchors, snapLabels(labels, wires)...)

	if open != nil {
		anchors = append(anchors, resolveAnchors(powers, open)...)
		resolveSlots(placements, placementSlots, open)
		pins, resolved, unresolved := symread.ResolvePins(placements, loadPins(open), quant)
		// Hidden net= pin taps land as anchors at their resolved pin points, the same mechanism
		// power taps use.
		anchors = append(anchors, tapAnchors(netTaps, pins)...)
		nets, dangles, _ := netgraph.Build(wires, anchors, pins, nil)
		d.Nets = append(d.Nets, netgraph.IRNets(nets, src)...)
		// Dangling endpoints are trustworthy only when every placement resolved (WS1-013). An
		// unresolved symbol drops its pins and turns wire ends into phantom dangles, so one
		// unresolved placement suppresses the whole design's dangles. The netgraph grid is the
		// geometry frame (quant only rounds), and with no per-wire id the location is the subject.
		// Unresolved symbols are recorded regardless (WS1-052), since the suppression would
		// otherwise hide their cause.
		d.InputDiagnostics = &ir.InputDiagnostics{
			UnresolvedSymbols: irUnresolved(unresolved, src),
			// Declared only on this branch (agni issue 418). The no-opener path opens no symbol,
			// so declaring it there would claim a check that never ran. See
			// docsite/content/architecture/ingestion-and-ir.md#input-diagnostics.
			ResolvedSymbols: irResolved(resolved),
			Supplied:        []string{"resolved_symbols"},
		}
		if len(unresolved) == 0 {
			d.InputDiagnostics.DanglingEndpoints = netgraph.IRDangles(dangles, src, "")
		}
	} else {
		// Without a symbol library the power taps and net= taps cannot be placed, but their net
		// names are still known, so surface them by name.
		for _, pt := range powers {
			if nm := pt.instanceNet; nm != "" {
				anchors = append(anchors, netgraph.Anchor{Label: nm, External: true}) // supply rail (WS1-021)
			}
		}
		for _, t := range netTaps {
			anchors = append(anchors, netgraph.Anchor{Label: t.name})
		}
		for _, name := range symread.NameOnlyNets(wires, anchors) {
			d.Nets = append(d.Nets, &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: src}})
		}
	}
	// Bus detection is syntactic and independent of symbol resolution, so it attaches after
	// either branch.
	if len(buses) > 0 {
		ensureDiag(d).UnmodeledBuses = buses
	}
	// gEDA KEEPS placeholder-designated parts (the libraries ship refdes=R? / U? as the template
	// value), so it owes the unannotated diagnostic. See
	// docsite/content/architecture/ingestion-and-ir.md#placeholder-designators.
	if un := refdes.Unannotated(d.Components); len(un) > 0 {
		ensureDiag(d).UnannotatedComponents = un
	}
	// Declared unconditionally, because `supplied` says this reader LOOKED and a clean read must
	// say so too (agni issue 309).
	diag := ensureDiag(d)
	diag.RefDesCollisions = symread.RefDesCollisions(d.Components)
	diag.Supplied = append(diag.Supplied, "ref_des_collisions")
	return d
}

// ensureDiag returns d's InputDiagnostics, building it on first use. Every signal recorded after
// the symbol-resolution branch goes through it, because that branch leaves the field nil on one
// path and assigning a fresh struct per signal drops whatever was recorded before it.
func ensureDiag(d *ir.Design) *ir.InputDiagnostics {
	if d.InputDiagnostics == nil {
		d.InputDiagnostics = &ir.InputDiagnostics{}
	}
	return d.InputDiagnostics
}

// powerTap is a power/ground symbol instance, pin-resolved after the walk.
type powerTap struct {
	symref        string
	x, y          float64
	angle, mirror int
	instanceNet   string // net name from the instance's net= attribute, if any
}

// textLabel is a standalone netname= text at its drawn location, resolved to a net later.
type textLabel struct {
	at   netgraph.Point
	name string
}

// netTap is one component pin named onto a net by a net=NAME:pin attribute (a wireless
// connection), resolved to the pin's grid point after the walk.
type netTap struct {
	ref, pin, name string
}

// isPowerSymbol reports whether a symbol names a power/ground net at a point rather than being a
// physical component. Besides gnd/vcc/vdd/vss, gEDA carries voltage rails as "<rail>-plus-N" /
// "<rail>-minus-N" symbols (3.3V-plus-1, 12V-minus-1) that name the rail via a symbol-level net=.
// They must not fall through to the component path, which drops anything without a ref-des.
func isPowerSymbol(sym string) bool {
	return powerSymbols[sym] || strings.Contains(sym, "-plus-") || strings.Contains(sym, "-minus-")
}

// parseNetTap splits a net= attribute value "NAME:pin[,pin...]" into the net name and its pin
// list. A value with no ":pin" names nothing to tap (a bare rail name is a power symbol's job), so
// it yields no pins.
func parseNetTap(spec string) (name string, pins []string) {
	i := strings.IndexByte(spec, ':')
	if i < 0 {
		return "", nil
	}
	name = spec[:i]
	for _, p := range strings.Split(spec[i+1:], ",") {
		if p = strings.TrimSpace(p); p != "" {
			pins = append(pins, p)
		}
	}
	return name, pins
}

// firstTapNet returns the net name of the first net= value (a power symbol instance names one
// rail), or "" when there is none.
func firstTapNet(nets []string) string {
	if len(nets) == 0 {
		return ""
	}
	return netFromNetAttr(nets[0])
}

// tapAnchors places each net= tap at its component pin's resolved grid point, so the pin merges
// onto the named net. A tap whose (ref, pin) did not resolve (a missing symbol or a wrong pin
// number) cannot be located and is dropped.
func tapAnchors(taps []netTap, pins []netgraph.Pin) []netgraph.Anchor {
	loc := map[[2]string]netgraph.Point{}
	for _, p := range pins {
		loc[[2]string{p.Comp, p.Pin}] = p.At
	}
	var out []netgraph.Anchor
	for _, t := range taps {
		if at, ok := loc[[2]string{t.ref, t.pin}]; ok {
			out = append(out, netgraph.Anchor{At: at, Label: t.name})
		}
	}
	return out
}

// dialect carries the gEDA constants for the shared component emission (internal/symread).
var dialect = symread.Dialect{
	Lib:          "geda",
	NativeIDKind: gedaNativeIDKind,
	SectionAttrs: []string{"value", "device", "model-name", "footprint"},
}

// netFromNetAttr extracts the net name from a gEDA net= attribute ("GND:1" -> "GND").
func netFromNetAttr(v string) string {
	if i := strings.IndexByte(v, ':'); i >= 0 {
		return v[:i]
	}
	return v
}

// irResolved stamps the resolver's loaded references with the construct kind. Returns nil for an
// empty set, which on this branch means a design that places no external symbol rather than a
// read that skipped them.
func irResolved(rs []symread.Resolved) []*ir.ResolvedSymbol {
	var out []*ir.ResolvedSymbol
	for _, r := range rs {
		out = append(out, &ir.ResolvedSymbol{Symref: r.Symref, Kind: "geda_sym", PinCount: int32(r.PinCount)})
	}
	return out
}

// irUnresolved turns the resolver's unresolved references into IR diagnostics, stamping the
// construct kind and source file the resolver does not know. Returns nil for an empty set.
func irUnresolved(us []symread.Unresolved, src string) []*ir.UnresolvedSymbol {
	var out []*ir.UnresolvedSymbol
	for _, u := range us {
		out = append(out, &ir.UnresolvedSymbol{
			Symref: u.Symref,
			Kind:   "geda_sym",
			RefDes: u.RefDes,
			Prov:   &ir.Provenance{SourceFile: src},
		})
	}
	return out
}
