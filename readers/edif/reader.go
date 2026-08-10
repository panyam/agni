package edif

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// Read parses an EDIF 2.0.0 netlist into an ir.Design.
//
// Fidelity: lossy-bounded (netlist subset). We extract components, part
// references, and net connectivity, not the full EDIF document. See CONSTRAINTS
// C6. sourceFile is recorded in provenance only; the caller owns file I/O so the
// core stays runtime-agnostic (CONSTRAINTS C1).
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	root, err := parse(r)
	if err != nil {
		return nil, err
	}
	// Version readiness: the S-expr parser is version-agnostic, but the extractor
	// below is keyed to the EDIF 2.0.0 netlist schema. Detect and gate rather than
	// silently mis-parsing a later schema; a 3.0.0/4.0.0 extractor would be additive.
	ver := edifVersion(root)
	if len(ver) > 0 && ver[0] != "2" {
		return nil, fmt.Errorf("edif: unsupported version %s (reader supports 2.0.0)", strings.Join(ver, "."))
	}
	d := extract(root, sourceFile)
	d.IrVersion = "0"
	d.SourceFormat = "edif-2.0.0"
	if len(ver) > 0 {
		d.SourceFormat = "edif-" + strings.Join(ver, ".")
		d.Attributes["edif_version"] = strings.Join(ver, ".")
	}
	return d, nil
}

// collectArrayBuses detects EDIF `array` bus-port declarations anywhere in the tree and records each
// as an unmodeled-bus diagnostic (WS1-034). An `(array name size)` port is a bus; its member set is
// the `size` indices, so the bus-not-modeled rule can tell a RESOLVED bus (every member already a net,
// because the design joined per-member nets named `NAME[i]`) from one whose members are unmodeled —
// the resolution-aware behavior PR 286 gave KiCad, now for EDIF (WS1-034 Phase 2). The array's name
// form (bare / rename / name) resolves via parseName.
func collectArrayBuses(root *node, src string) []*ir.BusNotModeled {
	var arrays []*node
	collect(root, "array", &arrays)
	var out []*ir.BusNotModeled
	for _, a := range arrays {
		nm := parseName(a.Arg(1))
		out = append(out, &ir.BusNotModeled{
			Kind:    "edif_array",
			Label:   nm.Display,
			Members: arrayMembers(nm.best(), atom(a.Arg(2))),
			Prov:    &ir.Provenance{SourceFile: src, NativeId: nm.ID, NativeIdKind: edifNativeIDKind},
		})
	}
	return out
}

// arrayMembers expands an EDIF `(array NAME size)` bus into its member net names, `base[0]..base[size-1]`,
// the `NAME[idx]` convention that portName projects a `(member NAME idx)` pin to (`reader_test.go`
// pins it). Matching that convention lets the bus-not-modeled rule confirm each member as a net. Returns
// nil for a missing base or a non-positive / non-numeric size, which leaves the bus flagged
// unconditionally (the pre-Phase-2 behavior) rather than asserting a member set the source did not give.
func arrayMembers(base, size string) []string {
	n, err := strconv.Atoi(size)
	if base == "" || err != nil || n <= 0 {
		return nil
	}
	members := make([]string, n)
	for i := range n {
		members[i] = fmt.Sprintf("%s[%d]", base, i)
	}
	return members
}

// edifNativeIDKind tags EDIF's internal rename &id in Provenance. It is regenerated per
// export, so it is a native id, never a stable cross-revision key (WS1-004).
const edifNativeIDKind = "edif-rename-id"

// extract walks the parsed EDIF tree and builds the netlist IR: part libraries,
// components (grouped from instances by ref_des), and nets. Instances and nets are
// collected from anywhere in the tree since they only occur inside the single
// (contents ...) block.
func extract(root *node, src string) *ir.Design {
	d := &ir.Design{Attributes: map[string]string{}, Prov: &ir.Provenance{SourceFile: src}}

	if dn := findFirst(root, "design"); dn != nil {
		if disp := parseName(dn.Arg(1)).Display; disp != "" {
			d.Name = disp
		}
	}
	if d.Name == "" {
		d.Name = atom(root.Arg(1)) // e.g. "DxD"
	}

	var libs []*node
	collect(root, "library", &libs)
	for _, l := range libs {
		d.Libraries = append(d.Libraries, libraryOf(l, src))
	}

	// One physical component (ref_des) is often several EDIF instances -- sections of a
	// multi-gate IC, connector banks, a relay's coil and contacts. Group instances by
	// ref_des into a Component with N sections rather than emitting one component per
	// instance (WS1-001 learning). Designator-less instances are keyed by their internal
	// id so they are not merged together.
	// Scope extraction to the design's root cell so a hierarchical design's sub-cell
	// contents are not merged into the top netlist (WS1-004). Falls back to the whole
	// document when the root cell cannot be resolved.
	scope := root
	if tc := topCell(root); tc != nil {
		scope = tc
	}
	if cellsWithInstances(root) > 1 {
		d.Attributes["edif_hierarchical"] = "true"
	}

	var insts []*node
	collect(scope, "instance", &insts)
	refByID := make(map[string]string, len(insts))
	pinsByID := make(map[string]map[string][]string, len(insts))
	byKey := make(map[string]*ir.Component, len(insts))
	var order []string
	for _, in := range insts {
		refDes, sec, id, pinMap := instanceOf(in, src)
		if id != "" && refDes != "" {
			refByID[id] = refDes
		}
		if id != "" && pinMap != nil {
			pinsByID[id] = pinMap
		}
		key := refDes
		if key == "" {
			key = "\x00" + id
		}
		comp := byKey[key]
		if comp == nil {
			comp = &ir.Component{RefDes: refDes, Attributes: map[string]string{}, Prov: &ir.Provenance{SourceFile: src}}
			byKey[key] = comp
			order = append(order, key)
		}
		sec.Index = int32(len(comp.Sections))
		comp.Sections = append(comp.Sections, sec)
		// Aggregate section properties to the component level (first section wins on a
		// key conflict) so consumers have a component view without re-walking sections.
		// The per-section attributes remain authoritative.
		for k, v := range sec.Attributes {
			if _, ok := comp.Attributes[k]; !ok {
				comp.Attributes[k] = v
			}
		}
		normalizeMPN(comp)
	}
	for _, key := range order {
		d.Components = append(d.Components, byKey[key])
	}
	// Cell-MPN fallback (WS1-046 Piece B): a component with no inline MPN inherits its part-type
	// (cell) MPN. The section's PartRef is the &-stripped cellRef id; the cell is indexed by both its
	// display name and that stripped native id (matching classify.PartIndex's WS1-045 alias), so the
	// join resolves whether the cellRef names the part by display or by native id. An inline MPN
	// (already normalized above) is never overwritten.
	applyCellMPNFallback(d)
	// No RefDesCollision (input diagnostic, docs/19) is emitted here on purpose: EDIF represents a
	// multi-gate part as several instances sharing a designator (folded into sections above), and
	// carries no capture-unit to tell that legitimate grouping from a genuine duplicate. Detecting
	// one would false-positive on every multi-gate part, so EDIF contributes none -- the same way a
	// board/netlist source contributes no dangling endpoints. Roadmap: a pending corpus case tracks
	// the gap; a future format with explicit unit/slot semantics can populate it.

	var nets []*node
	collect(scope, "net", &nets)
	for _, nn := range nets {
		d.Nets = append(d.Nets, netOf(nn, src, refByID, pinsByID))
	}
	if buses := collectArrayBuses(root, src); len(buses) > 0 {
		d.InputDiagnostics = &ir.InputDiagnostics{UnmodeledBuses: buses}
	}
	return d
}

// libraryOf converts an EDIF (library ...) node into an ir.PartLibrary with its parts.
func libraryOf(n *node, src string) *ir.PartLibrary {
	name := parseName(n.Arg(1)).Display
	lib := &ir.PartLibrary{Name: name, Prov: &ir.Provenance{SourceFile: src}}
	for _, c := range n.Children("cell") {
		lib.Parts = append(lib.Parts, partTypeOf(c, src))
	}
	return lib
}

// cellDesignator returns a cell's OWN reference-designator prefix ("U", "C?", "REF**"), or "" when
// it declares none.
//
// It looks only at levels that belong to the cell: the cell, its views, and each view's interface.
// It never descends into a port, and that restriction is the whole point. A port's designator is a
// PIN NUMBER, and a recursive search finds one whenever the cell declares no prefix of its own. The
// pin number then lands in PartType.DesignatorPrefix, where Lexicon.Classify prefers it over the
// component's ref-des prefix, so prefixClasses["1"] misses and every component of that cell reads
// UNKNOWN. Every class-quantified rule then selects zero members and stays silent, which is
// indistinguishable from a clean design (agni issue 109).
//
// Real EDIF puts the prefix inside the interface, alongside the ports rather than above them:
//
//	(cell X (cellType GENERIC)
//	  (view NORMAL (viewType SCHEMATIC)
//	    (interface (designator "U")            <- the cell's prefix
//	      (port IO1 ... (designator "1") ...)  <- a pin number
//
// which is why this cannot simply refuse to enter the interface. The cell and view levels are also
// accepted because a hand-written netlist reasonably declares the prefix there.
func cellDesignator(n *node) string {
	if d := n.Child("designator"); d != nil {
		return stringDisplayText(d)
	}
	for _, v := range n.Children("view") {
		if d := v.Child("designator"); d != nil {
			return stringDisplayText(d)
		}
		if iface := v.Child("interface"); iface != nil {
			if d := iface.Child("designator"); d != nil {
				return stringDisplayText(d)
			}
		}
	}
	return ""
}

// partTypeOf converts an EDIF (cell ...) node into an ir.PartType, pulling its cellType
// (as kind), reference-designator prefix, and the ports (pins) from its view interface.
func partTypeOf(n *node, src string) *ir.PartType {
	nm := parseName(n.Arg(1))
	pt := &ir.PartType{Name: nm.best(), Prov: &ir.Provenance{SourceFile: src, NativeId: nm.ID, NativeIdKind: edifNativeIDKind}}
	if ct := n.Child("cellType"); ct != nil {
		pt.Kind = atom(ct.Arg(1))
	}
	pt.DesignatorPrefix = cellDesignator(n)
	// Cell-level MPN (WS1-046 Piece B): OrCAD names a shared part-type cell by its part number and
	// carries the Manufacturer_PN on the CELL, not on every placed instance. Capture it (normalized
	// to "MPN") so an instance with no inline MPN falls back to its cell's in extract(). Only a LEAF
	// part cell (no contents) is scanned, so a hierarchical cell's nested instance properties are
	// never mistaken for the cell's own. classify reads component attributes, not part-type ones, so
	// this touches no classification.
	if n.Child("contents") == nil {
		var cprops []*node
		collect(n, "property", &cprops)
		for _, alias := range mpnPropertyAliases {
			for _, p := range cprops {
				if parseName(p.Arg(1)).best() != alias {
					continue
				}
				if v := propValue(p); v != "" {
					if pt.Attributes == nil {
						pt.Attributes = map[string]string{}
					}
					pt.Attributes["MPN"] = v
					break
				}
			}
			if pt.GetAttributes()["MPN"] != "" {
				break
			}
		}
	}
	var ports []*node
	collect(n, "port", &ports)
	for _, p := range ports {
		pn := parseName(p.Arg(1))
		pin := &ir.Pin{Name: pn.best(), Attributes: map[string]string{}, Prov: &ir.Provenance{SourceFile: src, NativeId: pn.ID, NativeIdKind: edifNativeIDKind}}
		if dir := p.Child("direction"); dir != nil {
			raw := atom(dir.Arg(1))
			pin.Direction = mapDirection(raw)
			// Keep the raw spelling only when it did not map cleanly (escape hatch, C9).
			if pin.Direction == ir.PinDirection_PIN_DIRECTION_UNSPECIFIED && raw != "" {
				pin.Attributes["direction_raw"] = raw
			}
		}
		if dg := p.Child("designator"); dg != nil {
			pin.Designator = stringDisplayText(dg)
		}
		if pin.Designator == "" {
			// Fall back to the port NAME (issue 71). The Model indexes pins by Designator
			// (`refDes + "\x00" + pin.Designator`) while a connection carries whatever the joined
			// portRef named, and EDIF's portRef names the PORT. A part whose ports declare no
			// designator therefore indexed every pin under one empty key and nothing ever resolved,
			// so PinRole returned unknown for every pin on the design and EVERY pin-role rule
			// (diode orientation, gate/source/drain, LED polarity) was silently inert on EDIF.
			//
			// Only the fallback: an explicit designator is the physical pin number and a connection
			// on such a part already carries that number, so overwriting it would break the join
			// that currently works. Same posture as normalizeMPN below, which fills the canonical
			// field from the best available source and never overrides an explicit one.
			pin.Designator = pin.Name
		}
		pt.Pins = append(pt.Pins, pin)
	}
	return pt
}

// mapDirection normalizes an EDIF port direction onto ir.PinDirection. EDIF only uses
// INPUT/OUTPUT/INOUT; other formats' extra kinds map here as more readers land.
func mapDirection(raw string) ir.PinDirection {
	switch strings.ToUpper(raw) {
	case "INPUT":
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case "OUTPUT":
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case "INOUT":
		return ir.PinDirection_PIN_DIRECTION_INOUT
	default:
		return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
	}
}

// instanceOf converts an EDIF (instance ...) node into one ComponentSection. It returns
// the ref_des (to group sections into a Component), the instance's internal rename id
// (so netOf can resolve instanceRefs, which use the internal id, back to the ref_des),
// and the instance's portInstance table: logical port -> the PHYSICAL pin designator(s)
// it maps to on this placement. The table is what keeps connection pin identity physical
// (WS1-025): a connector cell's single logical "GND" port fans out to different physical
// pins per section, and without the mapping every section's ground collapses onto one
// (ref_des, "GND") key — the pin-net-conflict tripwire's finding on the real corpus. A
// port may map to SEVERAL pins on one placement (the fan-out case); the slice preserves
// source order.
func instanceOf(n *node, src string) (refDes string, sec *ir.ComponentSection, id string, pinMap map[string][]string) {
	id = parseName(n.Arg(1)).ID
	sec = &ir.ComponentSection{
		Attributes: map[string]string{},
		Prov:       &ir.Provenance{SourceFile: src, NativeId: id, NativeIdKind: edifNativeIDKind},
	}
	if dg := n.Child("designator"); dg != nil {
		refDes = stringDisplayText(dg)
	}
	for _, pi := range n.Children("portInstance") {
		if dg := pi.Child("designator"); dg != nil {
			key := portName(pi.Arg(1))
			if pinMap == nil {
				pinMap = map[string][]string{}
			}
			pinMap[key] = append(pinMap[key], stringDisplayText(dg))
		}
	}
	if v := n.Child("viewRef"); v != nil {
		if cr := v.Child("cellRef"); cr != nil {
			// The EDIF "&" is a syntactic escape for an identifier the bare grammar cannot hold
			// (one starting with a digit, which OrCAD/Allegro emit for numeric library-cell ids),
			// NOT part of the identifier. partTypeOf names the part by its un-escaped display
			// ((rename &87844225 "87844225") -> "87844225"), and PartIndex keys on that, so the
			// cellRef reference must strip the escape to match — otherwise the component never
			// links to its part-type and has NO pins, invisible to every pin-level rule. Same
			// normalization portName applies to pin references.
			sec.PartRef = strings.TrimPrefix(atom(cr.Arg(1)), "&")
			if lr := cr.Child("libraryRef"); lr != nil {
				sec.LibraryRef = strings.TrimPrefix(atom(lr.Arg(1)), "&")
			}
		}
	}
	for _, p := range n.Children("property") {
		key := parseName(p.Arg(1)).best()
		if key != "" {
			sec.Attributes[key] = propValue(p)
		}
	}
	return refDes, sec, id, pinMap
}

// mpnPropertyAliases are the EDIF property names an OrCAD/Allegro export prints a part's
// manufacturer part number under, in precedence order. The reader normalizes the first present
// to the canonical "MPN" attribute the model's datasheet join reads (ComponentMPN), so a
// datasheet-backed rule works on an OrCAD netlist the same way it does on KiCad (whose reader
// already carries "MPN"). Both the display spelling ("Manufacturer PN", from a renamed property)
// and the bare id ("Manufacturer_PN") occur, so both are listed.
var mpnPropertyAliases = []string{"Manufacturer_PN", "Manufacturer PN"}

// normalizeMPN populates comp.Attributes["MPN"] from the first present alias when the component
// carries no explicit "MPN" already (an explicit MPN always wins). The source property is left in
// place; only the canonical key is added, so no existing attribute is lost or overwritten.
func normalizeMPN(comp *ir.Component) {
	if comp.Attributes["MPN"] != "" {
		return
	}
	for _, alias := range mpnPropertyAliases {
		if v := comp.Attributes[alias]; v != "" {
			comp.Attributes["MPN"] = v
			return
		}
	}
}

// applyCellMPNFallback fills each component's MPN from its part-type (cell) when the placed
// instance carried none inline (WS1-046 Piece B). It indexes every cell that captured an MPN by
// both its display name and its &-stripped native id, then joins on a component's first section
// PartRef (the &-stripped cellRef). Idempotent and non-destructive: a component that already has an
// MPN is left untouched, and a component whose cell has none stays empty (no false fill).
func applyCellMPNFallback(d *ir.Design) {
	cellMPN := map[string]string{}
	for _, lib := range d.Libraries {
		for _, pt := range lib.Parts {
			m := pt.GetAttributes()["MPN"]
			if m == "" {
				continue
			}
			if pt.Name != "" {
				cellMPN[pt.Name] = m
			}
			if id := strings.TrimPrefix(pt.GetProv().GetNativeId(), "&"); id != "" {
				if _, ok := cellMPN[id]; !ok {
					cellMPN[id] = m
				}
			}
		}
	}
	if len(cellMPN) == 0 {
		return
	}
	for _, comp := range d.Components {
		if comp.Attributes["MPN"] != "" || len(comp.Sections) == 0 {
			continue
		}
		if m := cellMPN[comp.Sections[0].PartRef]; m != "" {
			comp.Attributes["MPN"] = m
		}
	}
}

// netOf converts an EDIF (net ...) node into an ir.Net, turning each (portRef pin
// (instanceRef id)) into a Connection, resolving the internal id to a ref_des via
// refByID and the logical port to its PHYSICAL pin designator(s) via the instance's
// portInstance table (pinsByID). A port that maps to several pins on the placement
// fans out to one Connection per pin; a port with no mapping keeps the port name —
// which is also what keeps keys stable for the common `&N`-style port whose stripped
// name equals its physical designator.
func netOf(n *node, src string, refByID map[string]string, pinsByID map[string]map[string][]string) *ir.Net {
	nm := parseName(n.Arg(1))
	net := &ir.Net{Name: nm.best(), Prov: &ir.Provenance{SourceFile: src, NativeId: nm.ID, NativeIdKind: edifNativeIDKind}}
	var prs []*node
	collect(n, "portRef", &prs)
	for _, pr := range prs {
		inst := ""
		if ins := pr.Child("instanceRef"); ins != nil {
			inst = atom(ins.Arg(1))
		}
		// An instanceRef that resolves to no ref_des (power/ground/off-page symbol, or a
		// top-level port ref with no instanceRef) is keyed by the "" no-ref marker, NOT the
		// export-unstable internal id -- keying on the id would make the connection read as
		// changed on every revision diff (WS1-004). The raw id stays in provenance only.
		port := portName(pr.Arg(1))
		pins := pinsByID[inst][port]
		if len(pins) == 0 {
			pins = []string{port}
		}
		for _, pin := range pins {
			net.Connections = append(net.Connections, &ir.Connection{
				ComponentRef: refByID[inst],
				PinRef:       pin,
				Prov:         &ir.Provenance{SourceFile: src, NativeId: inst, NativeIdKind: edifNativeIDKind},
			})
		}
	}
	return net
}

// propValue extracts the scalar value from an EDIF (property ...) node, handling the
// string, integer, and boolean value forms. The string form covers both the netlist
// (string "V") and the schematic (string (stringDisplay "V" ...)) wrappers via
// stringDisplayText, so a property carries its value on the .eds view as well as the .edn.
func propValue(p *node) string {
	if s := p.Child("string"); s != nil {
		return stringDisplayText(s)
	}
	if i := p.Child("integer"); i != nil {
		return atom(i.Arg(1))
	}
	if b := p.Child("boolean"); b != nil {
		return b.Head()
	}
	return ""
}

// edifName is a parsed EDIF entity name: an identifier plus an optional human display
// string. best() resolves the pair the way every caller wants: the display when the form
// carried one, else the identifier.
type edifName struct {
	ID      string
	Display string
}

func (e edifName) best() string {
	if e.Display != "" {
		return e.Display
	}
	return e.ID
}

// An EDIF name is a small recursive sum type. Each predicate below recognizes ONE form and
// binds its parts, so parseName reads as an ordered alternation and adding a form is one
// predicate plus one row in TestNameForms. Keeping the grammar in named predicates rather
// than a switch hand-inlined at each call site is what turns a missing form into a failing
// test instead of a silently empty name (the WS1-026 failure mode).

// asAtom matches a bare identifier atom: FOO.
func asAtom(n *node) (id string, ok bool) {
	if n != nil && !n.IsList {
		return n.Atom, true
	}
	return "", false
}

// asRename matches (rename INNER "Display"), binding the inner name (itself a bare id atom or
// a nested (name ...)) and the trailing quoted string.
func asRename(n *node) (inner *node, disp string, ok bool) {
	if n != nil && n.IsList && n.Head() == "rename" && len(n.Kids) >= 3 {
		return n.Arg(1), atom(n.Arg(2)), true
	}
	return nil, "", false
}

// asName matches (name ID (display ...)): the display children carry placement, not a human
// string, so the form yields an identifier with no display name.
func asName(n *node) (id string, ok bool) {
	if n != nil && n.IsList && n.Head() == "name" {
		return atom(n.Arg(1)), true
	}
	return "", false
}

// asMember matches (member NAME IDX), a bus-element reference.
func asMember(n *node) (base, idx string, ok bool) {
	if n != nil && n.IsList && n.Head() == "member" {
		return atom(n.Arg(1)), atom(n.Arg(2)), true
	}
	return "", "", false
}

// parseName resolves an EDIF entity name (design, net, cell, library, part, pin, instance)
// across every form: bare atom, (rename ID "D"), (name ID ...), and the nested
// (rename (name ID ...) "D"). It leaves the identifier verbatim (no & strip, no member
// expansion): those are pin-identity normalizations, applied only in portName.
func parseName(n *node) edifName {
	if id, ok := asAtom(n); ok {
		return edifName{ID: id, Display: id}
	}
	if inner, disp, ok := asRename(n); ok {
		return edifName{ID: parseName(inner).ID, Display: disp}
	}
	if id, ok := asName(n); ok {
		return edifName{ID: id}
	}
	return edifName{}
}

// nameParts is the (id, display) tuple adapter over parseName, kept for the schematic.go
// call sites that predate edifName. New code uses parseName().best() directly.
func nameParts(n *node) (id, disp string) {
	p := parseName(n)
	return p.ID, p.Display
}

// portName is the PIN-identity projection of a portRef's name: the & escape is stripped and a
// (member NAME IDX) bus pin becomes NAME[IDX] so bus-pin identity survives (WS1-004). It is
// deliberately separate from parseName because pin identity needs those normalizations while
// net/entity names must not carry them; the two share the shape predicates and differ only in
// normalization.
func portName(n *node) string {
	if id, ok := asAtom(n); ok {
		return strings.TrimPrefix(id, "&")
	}
	if inner, _, ok := asRename(n); ok {
		id, _ := asAtom(inner)
		return strings.TrimPrefix(id, "&")
	}
	if base, idx, ok := asMember(n); ok {
		return base + "[" + idx + "]"
	}
	return ""
}

// topCell returns the cell node referenced by (design ... (cellRef NAME ...)), or nil when
// it cannot be resolved (extraction then falls back to the whole document).
func topCell(root *node) *node {
	dn := findFirst(root, "design")
	if dn == nil {
		return nil
	}
	cr := dn.Child("cellRef")
	if cr == nil {
		return nil
	}
	want := atom(cr.Arg(1))
	if want == "" {
		return nil
	}
	var cells []*node
	collect(root, "cell", &cells)
	for _, c := range cells {
		if nm := parseName(c.Arg(1)); nm.ID == want || nm.Display == want {
			return c
		}
	}
	return nil
}

// cellsWithInstances counts cells whose subtree contains at least one instance. More than
// one means the design is hierarchical (several levels carry contents).
func cellsWithInstances(root *node) int {
	var cells []*node
	collect(root, "cell", &cells)
	n := 0
	for _, c := range cells {
		var ci []*node
		collect(c, "instance", &ci)
		if len(ci) > 0 {
			n++
		}
	}
	return n
}

// stringDisplayText returns the scalar text held by a value node (a designator or a property
// (string ...)), unwrapping the schematic-view (stringDisplay "V" ...) wrapper the .eds export
// uses. The netlist (.edn) writes the value as a bare atom ((designator "R1"), (string "1%")),
// while the schematic (.eds) wraps it ((designator (stringDisplay "R1")), (string (stringDisplay
// "10k" ...))); extract() must read both, since both views run through this one reader. Mirrors
// refDesOf/propText on the geometry side (schematic.go); a bare atom passes straight through, so
// applying it to the .edn form is a no-op.
func stringDisplayText(n *node) string {
	if n == nil {
		return ""
	}
	if sd := n.Child("stringDisplay"); sd != nil {
		return atom(sd.Arg(1))
	}
	return atom(n.Arg(1))
}

// atom returns the text of a leaf node, or "" if the node is a list or nil.
func atom(n *node) string {
	if n != nil && !n.IsList {
		return n.Atom
	}
	return ""
}

// findFirst returns the first node anywhere in the tree whose Head is head, or nil.
func findFirst(n *node, head string) *node {
	var out []*node
	collect(n, head, &out)
	if len(out) > 0 {
		return out[0]
	}
	return nil
}

// edifVersion returns the parts of (edifVersion X Y Z), e.g. ["2","0","0"], or nil
// if the header is absent.
func edifVersion(root *node) []string {
	v := findFirst(root, "edifVersion")
	if v == nil {
		return nil
	}
	var parts []string
	for _, k := range v.Kids[1:] {
		if !k.IsList {
			parts = append(parts, k.Atom)
		}
	}
	return parts
}
