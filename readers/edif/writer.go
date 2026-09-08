package edif

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// WriteNetlist emits an EDIF 2.0.0 netlist from an ir.Design (the inverse of Read).
//
// Fidelity: lossy-bounded, matching the reader (CONSTRAINTS C6). It writes the netlist subset Read
// consumes -- libraries, cells with their interfaces and ports, and the top cell's contents as
// instances and nets -- so the output round-trips at the IR level, not at the byte level. Read is
// itself lossy-bounded, so several things are already gone before the writer sees the design and it
// cannot invent them back:
//
//   - HIERARCHY. extract scopes instances and nets to the design's top cell and drops every
//     sub-cell's contents (WS1-004, TestHierarchyDetected). A sub-cell survives as a part type with
//     its pins and no contents, so writing a hierarchical design emits a FLAT one. The
//     edif_hierarchical attribute, which extract sets from the count of cells carrying instances,
//     is therefore absent on the re-read. That difference is the round trip reporting the reader's
//     own loss, not a writer bug, and writer_test.go excludes the attribute for exactly that reason.
//   - ARRAY BUS DECLARATIONS. A (port (array DATA 8) ...) reaches the IR as a BusNotModeled
//     diagnostic that records the label and the member set but not the cell or port it was declared
//     on, so there is nowhere to put it back. Array ports are not emitted.
//   - THE portInstance TABLE. Read resolves each logical port to its physical pin designator(s)
//     while building connections (WS1-025), so the IR holds the resolved pin and not the mapping
//     that produced it. Connections are written as direct portRefs naming the physical pin, which
//     reads back to the same connections by the no-mapping fallback in netOf.
//   - View names, view types, instance display names, and property value types, none of which reach
//     the IR at all. Each is minted from a constant below, which changes the bytes and not the IR.
//
// The caller owns file I/O, so the core stays runtime-agnostic (CONSTRAINTS C1).
func WriteNetlist(w io.Writer, d *ir.Design) error {
	e := &emitter{}
	e.design(d)
	_, err := io.WriteString(w, e.String())
	return err
}

// The names EDIF requires and the IR has no field for. Read records the first two per design when
// the source stated them (recordRootRefs), so these are the fallback for a design that came from
// another format. The rest are pure syntax: every cell needs a view, and every viewRef names one.
const (
	defaultTopCell     = "TOP"
	defaultWorkLibrary = "WORK"
	defaultDesignName  = "DESIGN"
	viewName           = "V"
	viewType           = "NETLIST"
)

// atomOK reports whether s can be written as a bare name rather than wrapped in a rename.
//
// It sits between two grammars, deliberately, and the gap in each direction is load-bearing.
//
// LOOSER than the EDIF identifier grammar, which admits only a letter followed by letters, digits
// and underscores, with `&` escaping anything else. The shared s-expression tokenizer splits on
// whitespace and parens and treats a quote as a string delimiter, returning everything else as one
// atom, which is why the fixtures spell a numeric pin as a bare `1` rather than as the &1 escape.
// Holding to the strict grammar here would wrap every such name in a rename and change its
// Prov.NativeId from "1" to "&1", failing the round-trip oracle over a difference that is cosmetic
// in the source. Real OrCAD and Allegro exports carry the loose forms too, so a reader that rejected
// them would not get far.
//
// TIGHTER than the tokenizer, by the characters a real reader rejects. GNU Electric refuses a cell
// name containing whitespace, `:`, `;`, `{`, `}` or `|` and ABANDONS THE WHOLE IMPORT, so a KiCad
// part name like `gateway:CAP` took every design read from KiCad out with it. Those characters
// cannot reach here from an EDIF source anyway: a name carrying one would have had to arrive as a
// bare atom, and no fixture in the tree has one, so tightening costs nothing on the round trip and
// is what makes the output readable elsewhere.
func atomOK(s string) bool {
	return s != "" && !strings.ContainsAny(s, badAtomChars)
}

const badAtomChars = " \t\r\n()\":;{}|"

// refID is the identifier a name is DECLARED under, and every reference to that name has to repeat
// it. A cellRef, a libraryRef and an instanceRef all name an identifier rather than a display name,
// so a declaration that gets renamed and a reference that does not stop pointing at each other.
// Sections carry the reference strings raw (`s.PartRef` is the cellRef id an EDIF source used, and
// the part's own name for every other format), which is why this takes the raw string and is the
// same function on both sides.
func refID(name string) string {
	if atomOK(name) {
		return name
	}
	return mintID(name)
}

// emitter accumulates LINES rather than one growing buffer, because a close has to reach back and
// append its parens to the line already written -- EDIF trails a run of closing parens on the last
// leaf rather than giving them lines of their own. Reaching into a strings.Builder means rebuilding
// it, which is a full copy per close and quadratic over a real export; reaching into a slice is not.
type emitter struct {
	lines []string
	ind   int
	// inst is the design's instance-name table, built once by contents and read by both the
	// instance and the net path, because the two have to agree on the identifier.
	inst *instanceTable
}

func (e *emitter) line(format string, args ...any) {
	e.lines = append(e.lines, strings.Repeat("  ", e.ind)+fmt.Sprintf(format, args...))
}

// open writes a list head and indents; close closes as many lists as it is given, all onto the last
// line written.
func (e *emitter) open(format string, args ...any) {
	e.line(format, args...)
	e.ind++
}

func (e *emitter) close(n int) {
	e.ind -= n
	e.lines[len(e.lines)-1] += strings.Repeat(")", n)
}

func (e *emitter) String() string {
	return strings.Join(e.lines, "\n") + "\n"
}

func (e *emitter) design(d *ir.Design) {
	root := d.GetAttributes()["edif_root_name"]
	if root == "" {
		root = d.GetName()
	}
	if root == "" {
		root = defaultDesignName
	}
	top, work := topRefs(d)

	e.open("(edif %s", nameExpr(root, ""))
	e.line("(edifVersion %s)", strings.ReplaceAll(edifVersionOf(d), ".", " "))
	name := d.GetName()
	if name == "" {
		name = defaultDesignName
	}
	// Where the contents go depends on whether the declared top cell is one of the design's part
	// types, and BOTH answers reproduce a shape the reader already handles.
	//
	// When it is, the contents belong in that cell, which is the ordinary export shape. When it is
	// not, topCell fails to resolve on the next read and extract falls back to scoping over the whole
	// document, so the contents are written into the (design ...) node itself. That is not a
	// workaround: unannotated.edn is exactly that file, contents under the design and a cellRef
	// naming a cell no library declares. Minting a cell to hold them instead would add a part type
	// the source never had, and the round trip would report the writer's invention as a difference.
	inCell := hasCell(d, work, top)
	e.inst = newInstanceTable(d)

	// LIBRARIES FIRST, and the design node last, because the design's cellRef is a FORWARD reference
	// otherwise. Our own reader walks the parsed tree and does not care, which is how the order
	// survived: it resolves every reference after the whole file is in memory. A reader that resolves
	// as it goes cannot, and GNU Electric is one, so it reported the top cell as missing on every file
	// this writer had ever produced. Real EDIF exports put the design node at the end.
	for _, lib := range d.GetLibraries() {
		e.open("(library %s", nameExpr(lib.GetName(), ""))
		for _, pt := range lib.GetParts() {
			e.cell(pt, d, inCell && lib.GetName() == work && pt.GetName() == top)
		}
		e.close(1)
	}
	decl := fmt.Sprintf("(design %s (cellRef %s (libraryRef %s))", nameExpr(name, ""), refID(top), refID(work))
	if inCell {
		e.line("%s)", decl)
	} else {
		e.open("%s", decl)
		e.contents(d)
		e.close(1)
	}
	e.close(1)
}

// hasCell reports whether the design declares the named cell in the named library, which is the test
// for whether the top cell is somewhere the contents can be attached.
func hasCell(d *ir.Design, lib, cell string) bool {
	for _, l := range d.GetLibraries() {
		if l.GetName() != lib {
			continue
		}
		for _, pt := range l.GetParts() {
			if pt.GetName() == cell {
				return true
			}
		}
	}
	return false
}

// topRefs resolves which cell carries the contents and which library holds it. Read records both
// when the source stated them, because the pair is what decides the SCOPE a re-read recovers: point
// the design at a different cell and the next read scopes to that cell's contents and comes back
// with different components and nets. Falling back to constants is therefore only right for a design
// that never came from EDIF.
func topRefs(d *ir.Design) (top, work string) {
	top, work = d.GetAttributes()["edif_top_cell"], d.GetAttributes()["edif_work_library"]
	if top == "" {
		top = defaultTopCell
	}
	if work == "" {
		work = defaultWorkLibrary
	}
	return top, work
}

// edifVersionOf recovers the version triple Read stashed, defaulting to the only version the
// extractor is keyed to.
func edifVersionOf(d *ir.Design) string {
	if v := d.GetAttributes()["edif_version"]; v != "" {
		return v
	}
	return "2.0.0"
}

// cell writes one part type. The designator prefix goes at CELL level, which is one of the three
// places cellDesignator accepts and the one the fixtures use; putting it in the interface would work
// equally but reads worse beside the ports, whose own designators are pin numbers.
func (e *emitter) cell(pt *ir.PartType, d *ir.Design, contents bool) {
	e.open("(cell %s", nameExpr(pt.GetName(), pt.GetProv().GetNativeId()))
	if k := pt.GetKind(); k != "" {
		e.line("(cellType %s)", k)
	}
	if p := pt.GetDesignatorPrefix(); p != "" {
		e.line("(designator %s)", edifString(p))
	}
	// A part type's own MPN is written back as a cell property under the canonical spelling. Read
	// accepts any spelling in classify.MPNAliases and only scans a LEAF cell, so this is skipped for
	// the cell that carries the contents -- where a property would not be read back anyway.
	if m := pt.GetMpn(); m != "" && !contents {
		e.line("(property %s (string %s))", classify.MPNAliases[0], edifString(m))
	}
	e.open("(view %s (viewType %s)", viewName, viewType)
	// A pin with NEITHER a name nor a designator is not a pin the source declared. It is what
	// partTypeOf produces for an array port: parseName recognizes four name forms and (array DATA 8)
	// is none of them, so the port lands in the part type nameless while the bus itself is picked up
	// separately as a BusNotModeled diagnostic. Array declarations are not written (see
	// WriteNetlist), so the pin they produce is not either, and writer_test.go drops those pins from
	// both sides for the same reason. Writing one would be worse than dropping it: there is no name
	// to write, so it would come back named after whatever placeholder was invented for it.
	//
	// A pin carrying ONLY a designator is a different thing and is declared. gEDA and Telesis record
	// a pin by its number and give it no logical name, so testing for a name alone dropped every one
	// of them out of the interface, and every net then referenced a port the cell did not declare
	// (agni issue 580). Its designator is its identifier, which is what the re-read recovers anyway
	// when no portInstance maps it.
	var pins []*ir.Pin
	for _, p := range pt.GetPins() {
		if p.GetName() != "" || p.GetDesignator() != "" {
			pins = append(pins, p)
		}
	}
	// Ports the NETLIST references and the part type never declared. EDIF resolves a portRef against
	// the cell's interface, so a reference to an undeclared port is not a thin file, it is a broken
	// one, and a conforming reader drops the connection. Several readers deliver a part type with
	// fewer pins than the design connects: a board file carries no part types at all, Telesis records
	// a package with no pin list, and a gEDA slot maps its gate onto physical pins the shared symbol
	// never names.
	//
	// This DECLARES what the connections already assert rather than inventing anything, which is why
	// it is not the fabrication the writer refuses elsewhere. It is bounded the same way: a pin on no
	// net is invisible to a netlist, so a cell completed this way carries the pins the design uses
	// and not the pins the part has.
	extra := e.inst.undeclaredPorts(pt)
	if len(pins) == 0 && len(extra) == 0 {
		e.line("(interface)")
	} else {
		e.open("(interface")
		for _, p := range pins {
			e.port(p)
		}
		for _, id := range extra {
			e.line("(port %s (designator %s))", refID(id), edifString(id))
		}
		e.close(1)
	}
	if contents {
		e.contents(d)
	}
	e.close(2)
}

// port writes one pin. The direction is written from the typed field, falling back to the raw source
// spelling the reader kept when it did not map cleanly (the C9 escape hatch), so a direction agni
// does not model survives a round trip instead of being normalized to nothing.
func (e *emitter) port(p *ir.Pin) {
	var parts []string
	if dir := directionName(p); dir != "" {
		parts = append(parts, fmt.Sprintf("(direction %s)", dir))
	}
	if des := p.GetDesignator(); des != "" {
		parts = append(parts, fmt.Sprintf("(designator %s)", edifString(des)))
	}
	e.line("(port %s%s)", nameExpr(portIdent(p), p.GetProv().GetNativeId()), joinPrefixed(parts))
}

// portIdent is the name a port is DECLARED under: its own where the source gave it one, and its
// designator otherwise, because a port has to be named something for a net to reference it.
func portIdent(p *ir.Pin) string {
	if n := p.GetName(); n != "" {
		return n
	}
	return p.GetDesignator()
}

func directionName(p *ir.Pin) string {
	switch p.GetDirection() {
	case ir.PinDirection_PIN_DIRECTION_INPUT:
		return "INPUT"
	case ir.PinDirection_PIN_DIRECTION_OUTPUT:
		return "OUTPUT"
	case ir.PinDirection_PIN_DIRECTION_INOUT:
		return "INOUT"
	}
	return p.GetAttributes()["direction_raw"]
}

// contents writes the top cell's instances and nets. One instance per component SECTION, not per
// component: extract groups several instances sharing a designator into one component with N
// sections (a multi-gate IC, a connector bank), so unrolling the sections is what restores the
// source's instance count.
func (e *emitter) contents(d *ir.Design) {
	e.open("(contents")
	for _, c := range d.GetComponents() {
		for _, s := range c.GetSections() {
			e.instance(c, s)
		}
	}
	for _, n := range d.GetNets() {
		e.net(n)
	}
	e.close(1)
}

func (e *emitter) instance(c *ir.Component, s *ir.ComponentSection) {
	ref := refID(s.GetPartRef())
	if lib := s.GetLibraryRef(); lib != "" {
		ref = fmt.Sprintf("%s (libraryRef %s)", ref, refID(lib))
	}
	head := fmt.Sprintf("(instance %s (viewRef %s (cellRef %s))",
		e.inst.name[s], viewName, ref)
	// A designator-less instance is a real state the reader models (refdes.Unannotated reports it),
	// so an empty ref-des emits no designator rather than an empty one.
	if r := c.GetRefDes(); r != "" {
		head += fmt.Sprintf(" (designator %s)", edifString(r))
	}
	pins := e.inst.mappedPins(s)
	props := sortedKeys(s.GetAttributes())
	if len(pins) == 0 && len(props) == 0 {
		e.line("%s)", head)
		return
	}
	e.open("%s", head)
	// The portInstance table maps each of the cell's logical ports to the physical pin it lands on
	// for THIS placement, and it is what makes a portRef naming a port resolvable back to a pin.
	// Read consumes it (pinsByID) and the IR keeps only the resolved pin, so this half is rebuilt
	// from the part type, whose pins carry both the name and the designator.
	for _, pin := range pins {
		e.line("(portInstance %s (designator %s))", refID(pin.GetName()), edifString(pin.GetDesignator()))
	}
	for _, k := range props {
		// Every property is written as a string. Read collapses the string, integer and boolean
		// value forms into one map of strings (propValue), so the source's form is not recoverable
		// and the string form is the one that reads back to the same value.
		e.line("(property %s (string %s))", nameExpr(k, ""), edifString(s.GetAttributes()[k]))
	}
	e.close(1)
}

// net writes one net. Every connection that names a component in the design is ANCHORED to that
// component's instance, and the unanchored form is reachable only for a connection that names no
// instance at all.
//
// That distinction is the whole point, because the two forms are not more and less detailed spellings
// of one thing. `(portRef 1 (instanceRef iu3))` names pin 1 of instance iu3; `(portRef 1)` names a
// port called 1 on the CONTAINING CELL, an interface port. Writing the second where the first was
// meant does not lose the connection, it asserts a different one, and a conforming reader believes
// it. Anchoring off Prov.NativeId alone was exactly that bug: only the EDIF reader fills that field,
// so every design read from any other format emitted a netlist whose components were all isolated
// while its component, net and pin counts all looked right (agni issue 563).
//
// The remaining unanchored case is a connection whose ComponentRef names something the design does
// not carry as a component: a KiCad power symbol or PWR_FLAG, which the reader records as a
// connection on "#PWR01" while deliberately keeping the component list physical. There is no
// instance to point at and minting one would fabricate a component the source never had, so the bare
// portRef stands. It re-reads as a connection with an empty ComponentRef, which is what an EDIF
// no-ref connection has always been.
func (e *emitter) net(n *ir.Net) {
	var refs []string
	seen := map[string]bool{}
	for _, c := range n.GetConnections() {
		id, anchored := e.inst.anchor(c)
		port := e.inst.portOf(c)
		if !anchored {
			refs = append(refs, fmt.Sprintf("(portRef %s)", portRefExpr(port)))
			continue
		}
		// One portRef per (instance, PORT), not per connection. A port mapped to several pins is
		// one reference in the source and several Connections in the IR (netOf fans it out over
		// pinsByID), so writing one each would double it on the way back. Folding here is the exact
		// inverse, because the portInstance table the instance carries fans it out again.
		key := id + "\x00" + port
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, fmt.Sprintf("(portRef %s (instanceRef %s))", portRefExpr(port), id))
	}
	e.line("(net %s (joined %s))", nameExpr(n.GetName(), n.GetProv().GetNativeId()), strings.Join(refs, " "))
}

// instanceTable decides the identifier each (instance ...) is written under, for the whole design at
// once. It exists as a table rather than a function of one section because the identifier has to be
// UNIQUE across the contents -- a net's (instanceRef ...) is a lookup, so two instances sharing a
// name make one of them unreachable -- and uniqueness is not a property any single section knows.
//
// Neither seed is unique on its own. A section's Prov.NativeId is only filled by some readers, and
// where it is filled it still repeats: a KiCad symbol placed on two sheets of one hierarchy carries
// the same id under two ref-des, and an unannotated part carries "R?" as many times as it occurs.
// A ref-des repeats too, both for the multi-section part it is meant to (a multi-gate IC) and for a
// genuine duplicate the reader models rather than resolves. So a seed is taken as a preference and
// the collision is broken here.
type instanceTable struct {
	name        map[*ir.ComponentSection]string
	byNative    map[string]string
	byNativeSec map[string]*ir.ComponentSection
	byRef       map[string][]*ir.ComponentSection
	parts       map[string]*ir.PartType
	usedPorts   map[*ir.PartType]map[string]bool
}

func newInstanceTable(d *ir.Design) *instanceTable {
	t := &instanceTable{
		name:        map[*ir.ComponentSection]string{},
		byNative:    map[string]string{},
		byNativeSec: map[string]*ir.ComponentSection{},
		byRef:       map[string][]*ir.ComponentSection{},
		parts:       classify.PartIndex(d),
		usedPorts:   map[*ir.PartType]map[string]bool{},
	}
	taken := map[string]bool{}
	anon := 0
	for _, c := range d.GetComponents() {
		secs := c.GetSections()
		for _, s := range secs {
			native := s.GetProv().GetNativeId()
			seed := ""
			if native != "" {
				seed = refID(native)
			}
			switch {
			case seed != "":
			// A ref-des is the only other name a section has, and it is the one the rest of the file
			// already spells out in the instance's own (designator ...), so a reader diffing two
			// exports sees a name that moves with the design rather than with the export.
			case c.GetRefDes() != "" && len(secs) > 1:
				seed = fmt.Sprintf("%s_%d", mintID(c.GetRefDes()), s.GetIndex())
			case c.GetRefDes() != "":
				seed = mintID(c.GetRefDes())
			default:
				anon++
				seed = fmt.Sprintf("I%d", anon)
			}
			id := seed
			for n := 2; taken[id]; n++ {
				id = fmt.Sprintf("%s_%d", seed, n)
			}
			taken[id] = true
			t.name[s] = id
			if native != "" {
				if _, ok := t.byNative[native]; !ok {
					t.byNative[native] = id
					t.byNativeSec[native] = s
				}
			}
			t.byRef[c.GetRefDes()] = append(t.byRef[c.GetRefDes()], s)
		}
	}
	// A second pass, because resolving a connection to its section needs the index the first pass
	// builds.
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			s := t.sectionFor(c)
			if s == nil {
				continue
			}
			pt := t.partOf(s)
			if pt == nil {
				continue
			}
			if t.usedPorts[pt] == nil {
				t.usedPorts[pt] = map[string]bool{}
			}
			t.usedPorts[pt][t.portOf(c)] = true
		}
	}
	return t
}

// anchor resolves the instance a connection hangs off, reporting false when the design carries none.
//
// Provenance is consulted first and wins outright, because a reader that filled it recorded the
// source's own instance id and the connection is already keyed on it (EDIF, where a connection to an
// instance carrying no ref-des keeps the id here and an empty ComponentRef -- TestUnresolvedRefIsStable).
// An id naming no instance in the contents is written back verbatim for the same reason: it is what
// the source said, and inventing a different anchor for it would be worse than preserving it.
func (t *instanceTable) anchor(c *ir.Connection) (string, bool) {
	if id := c.GetProv().GetNativeId(); id != "" {
		if n, ok := t.byNative[id]; ok {
			return n, true
		}
		return refID(id), true
	}
	secs := t.byRef[c.GetComponentRef()]
	if c.GetComponentRef() == "" || len(secs) == 0 {
		return "", false
	}
	// Which SECTION of a multi-section component a connection belongs to is not recorded in the IR --
	// a Connection carries a ref-des and a pin, and nothing narrows it to one gate. Where the sections
	// have different part types (a relay's coil and its contacts) the pin itself decides; where they
	// share one (a multi-gate IC, whose gates declare the same pins) nothing can, and the first
	// section is taken. Both instances belong to the same component and carry the same designator, so
	// the choice moves which gate the file names and never which part the connection reaches.
	if len(secs) > 1 {
		if s := t.sectionDeclaring(secs, c.GetPinRef()); s != nil {
			return t.name[s], true
		}
	}
	return t.name[secs[0]], true
}

// undeclaredPorts lists, in a stable order, the port identifiers nets reference on instances of this
// part type that its own interface does not declare.
func (t *instanceTable) undeclaredPorts(pt *ir.PartType) []string {
	used := t.usedPorts[pt]
	if len(used) == 0 {
		return nil
	}
	declared := map[string]bool{}
	for _, p := range pt.GetPins() {
		if id := portIdent(p); id != "" {
			declared[id] = true
		}
	}
	var out []string
	for id := range used {
		if !declared[id] {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// portOf resolves a connection's PHYSICAL pin to the logical port its cell declares, which is what a
// portRef has to name. A portRef naming the pin designator instead is agni's own spelling and nobody
// else's: our reader recovers it through netOf's no-mapping fallback, and a conforming reader looks
// for a port by that name on the cell, finds none, and drops the connection (agni issue 580).
//
// The pin is returned unchanged when the part type declares no matching designator, which covers two
// real cases and keeps both working exactly as before. A design read from a board file has no part
// types at all. And an EDIF source whose interface carries no designators (dup_ports.edn) has no
// mapping to rebuild, which is what WriteNetlist's header means about the portInstance table: it is
// unrecoverable THERE, and recoverable wherever the part type carries the pair.
func (t *instanceTable) portOf(c *ir.Connection) string {
	pin := c.GetPinRef()
	s := t.sectionFor(c)
	if s == nil {
		return pin
	}
	for _, p := range t.partPins(s) {
		if p.GetDesignator() == pin && p.GetName() != "" {
			return p.GetName()
		}
	}
	return pin
}

// mappedPins lists the pins whose portInstance entry an instance has to carry: the ones whose
// logical port and physical designator differ, since those are the ones a portRef cannot resolve
// without the table. A pin already named after its designator needs no entry.
func (t *instanceTable) mappedPins(s *ir.ComponentSection) []*ir.Pin {
	var out []*ir.Pin
	for _, p := range t.partPins(s) {
		if d := p.GetDesignator(); d != "" && p.GetName() != "" && d != p.GetName() {
			out = append(out, p)
		}
	}
	return out
}

// partOf resolves a section's part type through the same two keys PartIndex builds.
func (t *instanceTable) partOf(s *ir.ComponentSection) *ir.PartType {
	if pt := t.parts[s.GetLibraryRef()+"/"+s.GetPartRef()]; pt != nil {
		return pt
	}
	return t.parts["/"+s.GetPartRef()]
}

func (t *instanceTable) partPins(s *ir.ComponentSection) []*ir.Pin {
	return t.partOf(s).GetPins()
}

// sectionFor is anchor's resolution over again, returning the SECTION rather than its name, because
// the port lookup needs the part type and anchor's answer has already lost it.
func (t *instanceTable) sectionFor(c *ir.Connection) *ir.ComponentSection {
	if id := c.GetProv().GetNativeId(); id != "" {
		return t.byNativeSec[id]
	}
	secs := t.byRef[c.GetComponentRef()]
	if c.GetComponentRef() == "" || len(secs) == 0 {
		return nil
	}
	if len(secs) > 1 {
		if s := t.sectionDeclaring(secs, c.GetPinRef()); s != nil {
			return s
		}
	}
	return secs[0]
}

// sectionDeclaring picks the single section whose part type declares the pin, and reports nil when
// none or several do -- an ambiguous answer is no answer, and the caller has a defined fallback.
func (t *instanceTable) sectionDeclaring(secs []*ir.ComponentSection, pin string) *ir.ComponentSection {
	var hit *ir.ComponentSection
	for _, s := range secs {
		pt := t.parts[s.GetLibraryRef()+"/"+s.GetPartRef()]
		if pt == nil {
			pt = t.parts["/"+s.GetPartRef()]
		}
		if !declaresPin(pt, pin) {
			continue
		}
		if hit != nil {
			return nil
		}
		hit = s
	}
	return hit
}

// declaresPin matches a Connection.PinRef, which is a physical designator, against a part type's
// pins. A pin with no designator is matched on its name, because that is what the reader used as the
// designator when the source gave none.
func declaresPin(pt *ir.PartType, pin string) bool {
	for _, p := range pt.GetPins() {
		if d := p.GetDesignator(); d != "" {
			if d == pin {
				return true
			}
			continue
		}
		if p.GetName() == pin {
			return true
		}
	}
	return false
}

// portRefExpr is the inverse of portName, which is a NORMALIZATION rather than a parse: it strips a
// leading & escape and rewrites a (member NAME IDX) bus pin to NAME[IDX]. That asymmetry decides the
// encoding here, and it is narrower than it looks.
//
// portName reads a rename by its IDENTIFIER and discards the display string, unlike parseName, so a
// pin reference cannot be carried by a rename the way an entity name can: (rename P "with space")
// reads back as "P", not as the name written. The bare atom is therefore the ONLY form that reads
// back unchanged, which is also why the fixtures spell a numeric pin as a bare `1` rather than as
// the &1 escape the grammar would otherwise require -- the tokenizer returns either as one atom, and
// portName strips the &. A reference the tokenizer would not return as a single atom is genuinely
// unrepresentable and is sanitized, which is the one place this writer changes a value rather than
// its spelling. No EDIF-sourced design reaches it: portName built every PinRef in the first place.
//
// The member rewrite is worth inverting because it changes the shape rather than a character. A pin
// literally named DATA[0] and a member reference to bus DATA are indistinguishable in the IR, so
// this picks the member form; both read back to the same PinRef, so the choice costs bytes and not
// meaning.
func portRefExpr(pin string) string {
	if m := memberPin.FindStringSubmatch(pin); m != nil {
		return fmt.Sprintf("(member %s %s)", m[1], m[2])
	}
	if !atomOK(pin) {
		return mintID(pin)
	}
	return pin
}

var memberPin = regexp.MustCompile(`^(.+)\[(\d+)\]$`)

// nameExpr renders one EDIF entity name. Read accepts four forms (bare atom, (rename ID "D"),
// (name ID ...), and the nested combination) and collapses them through edifName.best() into one
// string, keeping the identifier in Prov.NativeId. The writer's job is to pick the form that reads
// back to the same pair.
//
// An identifier that differs from its display name is exactly what (rename ID "D") encodes, so that
// pair round-trips as a rename. When they agree the bare atom is shorter and is what the fixtures
// use. A display name the bare grammar cannot hold still needs an identifier to hang off, and one is
// derived from the name rather than counted from a sequence, because a positional id would move
// under any reordering and several committed captures sort by the strings this feeds.
func nameExpr(name, nativeID string) string {
	switch {
	case nativeID != "" && nativeID != name:
		return fmt.Sprintf("(rename %s %s)", refID(nativeID), edifString(name))
	case atomOK(name):
		return name
	default:
		return fmt.Sprintf("(rename %s %s)", mintID(name), edifString(name))
	}
}

// mintID derives a bare identifier from a display name, mapping every character the grammar rejects
// to an underscore and escaping a result that cannot start an identifier.
func mintID(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		return "&" + s
	}
	return s
}

// edifString renders a string literal. The dialect has NO escape mechanism -- sexpr.EDIFStrings
// reads to the closing quote -- so a value containing a quote cannot be represented at all, and the
// quote is dropped. Go's %q would emit a backslash escape that the reader would hand back verbatim
// as part of the value, which is worse: it corrupts the value silently instead of narrowing it.
func edifString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, "") + `"`
}

func joinPrefixed(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// sortedKeys orders a map's keys so the output is deterministic. A map walked in range order would
// make the writer emit a different byte stream per run, which is what a committed capture cannot
// tolerate.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
