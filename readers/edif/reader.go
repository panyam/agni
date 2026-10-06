package edif

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/refdes"
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
	// The S-expr parser is version-agnostic but the extractor is keyed to the EDIF
	// 2.0.0 netlist schema, so a later version errors rather than mis-parsing.
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

// collectArrayBuses records each EDIF `(array name size)` bus-port declaration anywhere in the tree
// as an unmodeled-bus diagnostic (WS1-034). Its members are the `size` indices, so the bus-not-modeled
// rule can count a bus whose `NAME[i]` members are already nets as RESOLVED
// (docsite/content/architecture/ingestion-and-ir.md#input-diagnostics). The name resolves via parseName.
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
// unconditionally rather than asserting a member set the source did not give.
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

	dn := findFirst(root, "design")
	if dn != nil {
		if disp := parseName(dn.Arg(1)).Display; disp != "" {
			d.Name = disp
		}
	}
	if d.Name == "" {
		d.Name = atom(root.Arg(1)) // e.g. "DxD"
	}
	recordRootRefs(d, root, dn)

	var libs []*node
	collect(root, "library", &libs)
	for _, l := range libs {
		d.Libraries = append(d.Libraries, libraryOf(l, src))
	}

	// Scope extraction to the design's root cell so a hierarchical design's sub-cell
	// contents are not merged into the top netlist (WS1-004), falling back to the whole
	// document when the root cell cannot be resolved. The cells left out are recorded as
	// InputDiagnostics.unexpanded_hierarchy below (agni issue 707).
	scope := root
	if tc := topCell(root); tc != nil {
		scope = tc
	}
	// One physical component is often several EDIF instances, such as the sections of a
	// multi-gate IC, connector banks, or a relay's coil and contacts. Group instances by
	// ref_des into one Component with N sections (WS1-001). Designator-less instances are
	// keyed by their internal id so they are not merged together.
	var insts []*node
	collect(scope, "instance", &insts)
	namedByInstance := !anyDesignator(insts)
	refByID := make(map[string]string, len(insts))
	pinsByID := make(map[string]map[string][]string, len(insts))
	byKey := make(map[string]*ir.Component, len(insts))
	var order []string
	for _, in := range insts {
		refDes, sec, id, pinMap := instanceOf(in, src)
		if refDes == "" && namedByInstance {
			refDes = parseName(in.Arg(1)).best()
		}
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
		// Aggregate section properties to the component level, first section winning a key
		// conflict. The per-section attributes remain authoritative.
		for k, v := range sec.Attributes {
			if _, ok := comp.Attributes[k]; !ok {
				comp.Attributes[k] = v
			}
		}
	}
	for _, key := range order {
		d.Components = append(d.Components, byKey[key])
	}

	var nets []*node
	collect(scope, "net", &nets)
	for _, nn := range nets {
		d.Nets = append(d.Nets, netOf(nn, src, refByID, pinsByID))
	}
	// One struct for every diagnostic, built once, since a fresh InputDiagnostics per signal
	// drops the previous one. Always set, because this reader looks at every cell and so
	// SUPPLIES unexpanded_hierarchy on every read, where an empty list means a flat design.
	//
	// No ref_des_collisions and none in Supplied, so duplicate-ref-des reads not-applicable
	// (agni issue 309). EDIF gives a multi-gate part several instances sharing a designator and
	// carries no capture unit to tell that from a real duplicate, so detecting one would
	// false-positive on every multi-gate part
	// (docsite/content/architecture/ingestion-and-ir.md#input-diagnostics).
	d.InputDiagnostics = &ir.InputDiagnostics{
		UnmodeledBuses:        collectArrayBuses(root, src),
		UnannotatedComponents: refdes.Unannotated(d.Components),
		UnexpandedHierarchy:   unexpandedCells(root, scope, src),
		Supplied:              []string{"unexpanded_hierarchy"},
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
// It looks only at the cell, its views, and each view's interface, and never descends into a port,
// because a port's designator is a PIN NUMBER. A recursive search finds one whenever the cell
// declares no prefix of its own, and the pin number then lands in PartType.DesignatorPrefix, where
// Lexicon.Classify prefers it over the component's ref-des prefix. prefixClasses["1"] misses, every
// component of that cell reads UNKNOWN, and every class-quantified rule selects zero members and
// stays silent, indistinguishable from a clean design (agni issue 109).
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
	// OrCAD names a shared part-type cell by its part number and carries the Manufacturer_PN on
	// the CELL, not on each placed instance (WS1-046 Piece B). It goes into the part type's mpn
	// field, which classify.StampMPN falls back to for an instance carrying none of its own. Only
	// a LEAF cell (no contents) is scanned, so a hierarchical cell's nested instance properties
	// are never read as the cell's own. Classification reads component attributes, so this
	// touches none.
	if n.Child("contents") == nil {
		var cprops []*node
		collect(n, "property", &cprops)
		for _, alias := range classify.MPNAliases {
			for _, p := range cprops {
				if parseName(p.Arg(1)).best() != alias {
					continue
				}
				if v := propValue(p); v != "" {
					pt.Mpn = v
					break
				}
			}
			if pt.GetMpn() != "" {
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
			// (`refDes + "\x00" + pin.Designator`) while EDIF's portRef names the PORT, so without
			// this a part whose ports declare no designator indexes every pin under one empty key,
			// PinRole returns unknown for every pin, and EVERY pin-role rule (diode orientation,
			// gate/source/drain, LED polarity) is silently inert on EDIF.
			//
			// Only a fallback, because an explicit designator is the physical pin number a
			// connection on such a part already carries, and overwriting it would break that join.
			pin.Designator = pin.Name
		}
		pt.Pins = append(pt.Pins, pin)
	}
	return pt
}

// mapDirection normalizes an EDIF port direction onto ir.PinDirection. EDIF uses only
// INPUT/OUTPUT/INOUT.
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

// anyDesignator reports whether any instance states its reference designator in a (designator ...)
// list. A producer that writes none, such as Altium's EDIF For PCB export, names each instance by its
// reference designator instead, so Read takes the instance's name then (agni issue 941). A producer
// that writes designators leaves them off power and off-page symbols on purpose, so the name is
// never used for an instance in a file that has any.
func anyDesignator(insts []*node) bool {
	for _, in := range insts {
		if in.Child("designator") != nil {
			return true
		}
	}
	return false
}

// instanceOf converts an EDIF (instance ...) node into one ComponentSection. It returns
// the ref_des (to group sections into a Component), the instance's internal rename id
// (so netOf can resolve instanceRefs, which use the internal id, back to the ref_des),
// and the instance's portInstance table: logical port -> the PHYSICAL pin designator(s)
// it maps to on this placement. The table keeps connection pin identity physical
// (WS1-025). A connector cell's single logical "GND" port fans out to different physical
// pins per section, and without the mapping every section's ground collapses onto one
// (ref_des, "GND") key, which the pin-net-conflict tripwire caught on a real corpus. A
// port may map to SEVERAL pins on one placement, and the slice preserves source order.
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
			// The EDIF "&" escapes an identifier the bare grammar cannot hold (one starting with a
			// digit, which OrCAD/Allegro emit for numeric library-cell ids) and is NOT part of it.
			// partTypeOf names the part by its un-escaped display ((rename &87844225 "87844225") ->
			// "87844225") and PartIndex keys on that, so the cellRef must strip the escape too.
			// Otherwise the component never links to its part type and has NO pins, invisible to every
			// pin-level rule. portName does the same for pin references.
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

// netOf converts an EDIF (net ...) node into an ir.Net, turning each (portRef pin
// (instanceRef id)) into a Connection, resolving the internal id to a ref_des via
// refByID and the logical port to its PHYSICAL pin designator(s) via the instance's
// portInstance table (pinsByID). A port that maps to several pins on the placement
// fans out to one Connection per pin. A port with no mapping keeps the port name, which
// also keeps keys stable for the common `&N`-style port whose stripped name equals its
// physical designator.
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
		// top-level port ref with no instanceRef) is keyed by the "" no-ref marker, NOT by the
		// export-unstable internal id, which would make the connection read as changed on every
		// revision diff (WS1-004). The raw id stays in provenance only.
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
// predicate plus one row in TestParseNameForms. Named predicates rather than a switch
// inlined at each call site turn a missing form into a failing test instead of a silently
// empty name (WS1-026).

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

// nameParts returns parseName's (id, display) pair as a tuple. A caller wanting the one
// preferred name uses parseName().best().
func nameParts(n *node) (id, disp string) {
	p := parseName(n)
	return p.ID, p.Display
}

// portName is the PIN-identity projection of a portRef's name: the & escape is stripped and a
// (member NAME IDX) bus pin becomes NAME[IDX] so bus-pin identity survives (WS1-004). It is
// separate from parseName because pin identity needs those normalizations and net/entity names
// must not carry them. The two share the shape predicates.
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

// unexpandedCells lists every cell outside scope whose contents hold instances, which extract
// therefore never read (WS1-004 scopes it to the top cell, agni issue 707). A cell with no instances
// is a leaf part and loses nothing. When the top cell cannot be resolved, scope is the whole document
// and extract read everything, so nothing is listed.
func unexpandedCells(root, scope *node, src string) []*ir.UnexpandedHierarchy {
	if scope == root {
		return nil
	}
	var cells []*node
	collect(root, "cell", &cells)
	var out []*ir.UnexpandedHierarchy
	for _, c := range cells {
		if c == scope {
			continue
		}
		var ci []*node
		collect(c, "instance", &ci)
		if len(ci) == 0 {
			continue
		}
		nm := parseName(c.Arg(1))
		out = append(out, &ir.UnexpandedHierarchy{
			Name:          nm.best(),
			Kind:          "edif_cell",
			InstanceCount: int32(len(ci)),
			Prov:          &ir.Provenance{SourceFile: src, NativeId: nm.ID, NativeIdKind: edifNativeIDKind},
		})
	}
	return out
}

// stringDisplayText returns the scalar text held by a value node (a designator or a property
// (string ...)), unwrapping the schematic-view (stringDisplay "V" ...) wrapper the .eds export
// uses. The netlist (.edn) writes the value as a bare atom ((designator "R1"), (string "1%")),
// while the schematic (.eds) wraps it ((designator (stringDisplay "R1")), (string (stringDisplay
// "10k" ...))), and both views run through this one reader. A bare atom passes straight
// through, so on the .edn form it is a no-op. Mirrors refDesOf/propText in schematic.go.
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

// recordRootRefs stashes the three names extract resolves and then discards. Each is an
// escape-hatch attribute (CONSTRAINTS C9), in the same shape as edif_version.
//
// The design's root reference names two of them, the cell carrying the top-level contents and the
// library holding that cell. extract resolves the pair through topCell and works from the cell's
// contents, so neither name reaches the IR. The scope a read recovers is decided by
// (design ... (cellRef C (libraryRef L))), so an emitter that guesses the pair points the design at a
// different cell, and the next read scopes to that cell and recovers different components and nets.
// Recording them keeps a write-then-read on the same scope, not merely the same file.
//
// The third is the (edif NAME ...) root name, distinct from the design's and consulted in extract
// only as a fallback. Three fixtures carry a root name their design does not (CELLMPN/TOP, UNANN/D,
// WRAPPED/a rename), so it would otherwise be lost. It is recorded only when it differs.
func recordRootRefs(d *ir.Design, root, design *node) {
	if design != nil {
		if cr := design.Child("cellRef"); cr != nil {
			if c := atom(cr.Arg(1)); c != "" {
				d.Attributes["edif_top_cell"] = c
			}
			if lr := cr.Child("libraryRef"); lr != nil {
				if l := atom(lr.Arg(1)); l != "" {
					d.Attributes["edif_work_library"] = l
				}
			}
		}
	}
	if rn := atom(root.Arg(1)); rn != "" && rn != d.Name {
		d.Attributes["edif_root_name"] = rn
	}
}
