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

// WriteNetlist emits an EDIF 2.0.0 netlist from an ir.Design (the inverse of Read), lossy-bounded
// like the reader (CONSTRAINTS C6), so the output round-trips at the IR level and not the byte level.
// It writes for readers other than ours, a stricter target than the round trip. The interop rules
// and the fidelity list are in docsite/content/guide/cli-reference.md#emit-in-out.
//
// What Read has already dropped cannot be written back:
//
//   - HIERARCHY. extract scopes to the top cell (WS1-004), so a hierarchical design writes out FLAT
//     and InputDiagnostics.unexpanded_hierarchy is empty on the re-read. writer_test.go excludes that
//     list for this reason.
//   - ARRAY BUS DECLARATIONS. A (port (array DATA 8) ...) reaches the IR as a BusNotModeled
//     diagnostic with no cell or port, so array ports are not emitted.
//   - View names, view types, instance display names and property value types never reach the IR,
//     so each is minted from a constant below.
//
// The portInstance table is rebuilt wherever the part type carries both a pin's name and its
// designator (see portOf). The caller owns file I/O (CONSTRAINTS C1).
func WriteNetlist(w io.Writer, d *ir.Design) error {
	e := &emitter{}
	e.design(d)
	_, err := io.WriteString(w, e.String())
	return err
}

// The names EDIF requires and the IR has no field for. Read records the first two per design when
// the source stated them (recordRootRefs), so these are the fallback for a design that came from
// another format. The rest are syntax, since every cell needs a view and every viewRef names one.
const (
	defaultTopCell     = "TOP"
	defaultWorkLibrary = "WORK"
	defaultDesignName  = "DESIGN"
	viewName           = "V"
	viewType           = "NETLIST"
)

// atomOK reports whether s can be written as a bare name rather than wrapped in a rename. It sits
// between two grammars.
//
// LOOSER than the EDIF identifier grammar (see identOK). The shared tokenizer returns anything but
// whitespace, parens and quotes as one atom, so the fixtures spell a numeric pin as a bare `1`, and
// real OrCAD and Allegro exports carry the loose forms too. The strict grammar here would rename
// every such name and change its Prov.NativeId from "1" to "&1", failing the round-trip oracle.
//
// TIGHTER than the tokenizer, by the characters a real reader rejects. GNU Electric ABANDONS THE
// WHOLE IMPORT on a cell name containing whitespace, `:`, `;`, `{`, `}` or `|`, which failed every
// KiCad design on its `lib:part` names (agni issue 580). No fixture carries one as a bare atom, so
// this costs the round trip nothing.
func atomOK(s string) bool {
	return s != "" && !strings.ContainsAny(s, badAtomChars)
}

const badAtomChars = " \t\r\n()\":;{}|"

// localName is a cell's name WITHIN its library, so `gateway:CONN4` in library `gateway` is written
// as `CONN4`. GNU Electric takes a rename's display string as the cell name and abandons the import
// on the colon, so a clean identifier beside a qualified display is not enough.
//
// The strip is a WRITER decision. The reader uses the prefix to select the .kicad_sym an external
// symbol resolves from, so stripping it upstream breaks symbol resolution (agni issue 580). Only an
// EXACT match of the enclosing library is stripped, and a prefix naming another library is kept.
func localName(lib, name string) string {
	if lib == "" {
		return name
	}
	return strings.TrimPrefix(name, lib+":")
}

// refID is the identifier a name is DECLARED under, which every reference to it has to repeat. A
// cellRef, libraryRef or instanceRef names an identifier and not a display name, so renaming a
// declaration without its references breaks the link. Sections carry reference strings raw
// (`s.PartRef` is an EDIF source's cellRef id and the part's own name for other formats), so the
// same function serves both sides.
func refID(name string) string {
	if atomOK(name) {
		return name
	}
	return mintID(name)
}

// emitter accumulates LINES rather than one buffer, because EDIF trails a run of closing parens on
// the last leaf and close appends them to the line already written. Doing that to a strings.Builder
// is a full copy per close, quadratic over a real export.
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
	// The contents go in the declared top cell when it is one of the design's part types, the
	// ordinary export shape. When a top cell the source NAMED resolves to nothing, they go under the
	// (design ...) node itself, which is the shape unannotated.edn has and extract reads by scoping
	// over the whole document. Minting a cell there would add a part type the source never had.
	inCell := hasCell(d, work, top)
	e.inst = newInstanceTable(d, work)

	// LIBRARIES FIRST and the design node last, or the design's cellRef is a FORWARD reference that a
	// reader resolving as it goes (GNU Electric) reports as a missing top cell (agni issue 580). Our
	// own reader resolves after the whole file is parsed and hides the problem.
	// Where the top-cell name is our own default, because the design never came from EDIF, the cell
	// is minted too, since contents directly under a design node is not a construct EDIF has.
	mintTop := !inCell && d.GetAttributes()["edif_top_cell"] == ""

	declared := map[string]bool{}
	for _, lib := range d.GetLibraries() {
		declared[lib.GetName()] = true
		e.open("(library %s", nameExpr(lib.GetName(), ""))
		for _, pt := range lib.GetParts() {
			e.cell(lib.GetName(), pt, d, inCell && lib.GetName() == work && pt.GetName() == top)
		}
		e.mintedCells(lib.GetName())
		if mintTop && lib.GetName() == work {
			e.topCell(d, top)
			mintTop, inCell = false, true
		}
		e.close(1)
	}
	// A library the design references and does not declare. A board read produces nothing but these,
	// because copper carries footprints and pads and never says what the part is.
	for _, lib := range e.inst.mintedLibraries() {
		if declared[lib] {
			continue
		}
		declared[lib] = true
		e.open("(library %s", nameExpr(lib, ""))
		e.mintedCells(lib)
		if mintTop && lib == work {
			e.topCell(d, top)
			mintTop, inCell = false, true
		}
		e.close(1)
	}
	if mintTop {
		e.open("(library %s", nameExpr(work, ""))
		e.topCell(d, top)
		e.close(1)
		inCell = true
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

// topCell writes the cell the contents live in, for a design whose source never named one.
func (e *emitter) topCell(d *ir.Design, name string) {
	e.open("(cell %s", nameExpr(name, ""))
	e.open("(view %s (viewType %s)", viewName, viewType)
	e.line("(interface)")
	e.contents(d)
	e.close(2)
}

// mintedCells writes the cells of one library that nothing declares, with interfaces taken from the
// connections, the completion cell() does for a part type declaring too few pins (agni issue 580).
func (e *emitter) mintedCells(lib string) {
	for _, k := range e.inst.mintedCells(lib) {
		e.open("(cell %s", nameExpr(k.name, ""))
		e.open("(view %s (viewType %s)", viewName, viewType)
		ports := e.inst.mintedPortsOf(k)
		if len(ports) == 0 {
			e.line("(interface)")
		} else {
			e.open("(interface")
			for _, p := range ports {
				e.line("(port %s (designator %s))", refID(p), edifString(p))
			}
			e.close(1)
		}
		e.close(2)
	}
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

// topRefs resolves which cell carries the contents and which library holds it, from what Read
// recorded (see recordRootRefs for why the pair decides a re-read's scope). The constants are the
// fallback for a design that never came from EDIF.
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

// cell writes one part type. The designator prefix goes at CELL level, one of the three places
// cellDesignator accepts and the one the fixtures use.
func (e *emitter) cell(libName string, pt *ir.PartType, d *ir.Design, contents bool) {
	e.open("(cell %s", nameExpr(localName(libName, pt.GetName()), pt.GetProv().GetNativeId()))
	if k := pt.GetKind(); k != "" {
		e.line("(cellType %s)", k)
	}
	if p := pt.GetDesignatorPrefix(); p != "" {
		e.line("(designator %s)", edifString(p))
	}
	// A part type's own MPN is written back as a cell property under the canonical spelling. Read
	// accepts any spelling in classify.MPNAliases and only scans a LEAF cell, so this is skipped for
	// the cell carrying the contents, where it would not be read back.
	if m := pt.GetMpn(); m != "" && !contents {
		e.line("(property %s (string %s))", classify.MPNAliases[0], edifString(m))
	}
	e.open("(view %s (viewType %s)", viewName, viewType)
	// A pin with NEITHER a name nor a designator is what partTypeOf produces for an array port, whose
	// (array DATA 8) name parseName does not recognize. Array declarations are not written (see
	// WriteNetlist), so neither is that pin, and writer_test.go drops such pins from both sides.
	// Written, it would come back named after whatever placeholder was invented for it.
	//
	// A pin carrying ONLY a designator is declared. gEDA and Telesis record pins by number with no
	// logical name, so testing for a name alone left every net referencing an undeclared port (agni
	// issue 580). The designator is its identifier, which the re-read recovers anyway when no
	// portInstance maps it.
	var pins []*ir.Pin
	for _, p := range pt.GetPins() {
		if p.GetName() != "" || p.GetDesignator() != "" {
			pins = append(pins, p)
		}
	}
	// Ports the NETLIST references and the part type never declared. EDIF resolves a portRef against
	// the cell's interface, so a conforming reader drops a connection to an undeclared port. A board
	// file carries no part types, Telesis records a package with no pin list, and a gEDA slot maps its
	// gate onto pins the shared symbol never names. This declares only what the connections already
	// assert, so the cell carries the pins the design uses and not the pins the part has.
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

// contents writes the top cell's instances and nets, one instance per component SECTION rather than
// per component. extract groups instances sharing a designator into one component with N sections
// (a multi-gate IC, a connector bank), so unrolling them restores the source's instance count.
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
	ref := refID(localName(s.GetLibraryRef(), s.GetPartRef()))
	if lib := s.GetLibraryRef(); lib != "" {
		ref = fmt.Sprintf("%s (libraryRef %s)", ref, refID(lib))
	}
	head := fmt.Sprintf("(instance %s (viewRef %s (cellRef %s))",
		nameExpr(e.inst.displayOr(s), e.inst.name[s]), viewName, ref)
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

// net writes one net. A connection naming a component in the design is ANCHORED to that component's
// instance. `(portRef 1 (instanceRef iu3))` names pin 1 of instance iu3, and `(portRef 1)` names an
// interface port called 1 on the CONTAINING CELL, so writing the second where the first was meant
// asserts a different connection. Anchoring off Prov.NativeId alone, which only the EDIF reader
// fills, left every other format's components isolated while all the counts looked right (agni
// issue 563).
//
// The bare portRef stays for a connection naming no component the design carries, such as a KiCad
// power symbol or PWR_FLAG recorded on "#PWR01" while the component list stays physical. Minting an
// instance would fabricate a component, and the bare form re-reads as an EDIF no-ref connection.
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
// once, because the identifier has to be UNIQUE across the contents and no single section knows that.
// A net's (instanceRef ...) is a lookup, so two instances sharing a name make one unreachable.
//
// Neither seed is unique on its own. Only some readers fill a section's Prov.NativeId, and a KiCad
// symbol placed on two sheets of one hierarchy carries the same id under two ref-des. A ref-des
// repeats for a multi-section part and for a duplicate the reader models rather than resolves. So a
// seed is a preference and collisions are broken here.
type instanceTable struct {
	name        map[*ir.ComponentSection]string
	display     map[*ir.ComponentSection]string
	byNative    map[string]string
	byNativeSec map[string]*ir.ComponentSection
	byRef       map[string][]*ir.ComponentSection
	parts       map[string]*ir.PartType
	usedPorts   map[*ir.PartType]map[string]bool
	// mintPorts is usedPorts for the cells NOTHING declares, keyed by the libraryRef and partRef a
	// section names, because there is no part type to key on. A board read produces only these.
	mintPorts map[cellRef]map[string]bool
	mintOrder []cellRef
	work      string
}

// cellRef is a cell the file references: the library it is named in and its name within that library.
type cellRef struct{ lib, name string }

func newInstanceTable(d *ir.Design, work string) *instanceTable {
	t := &instanceTable{
		name:        map[*ir.ComponentSection]string{},
		display:     map[*ir.ComponentSection]string{},
		byNative:    map[string]string{},
		byNativeSec: map[string]*ir.ComponentSection{},
		byRef:       map[string][]*ir.ComponentSection{},
		parts:       classify.PartIndex(d),
		usedPorts:   map[*ir.PartType]map[string]bool{},
		mintPorts:   map[cellRef]map[string]bool{},
		work:        work,
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
			// A native id that is already a legal EDIF identifier is kept verbatim, so an EDIF round
			// trip is byte-faithful. A KiCad uuid opens with a digit and carries hyphens, so it is
			// minted and the original rides along as the display name (agni issue 582).
			case seed != "":
				if !identOK(seed) {
					seed = mintID(seed)
				}
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
			if native != "" && native != id {
				t.display[s] = native
			}
			if native != "" {
				if _, ok := t.byNative[native]; !ok {
					t.byNative[native] = id
					t.byNativeSec[native] = s
				}
			}
			t.byRef[c.GetRefDes()] = append(t.byRef[c.GetRefDes()], s)
			// Registered from the SECTION rather than from the connections, because a component
			// connected to nothing still names its cell and EDIF still needs that cell declared. The
			// three mounting holes on the sample board have no nets and a cellRef each.
			if t.partOf(s) == nil {
				k := t.mintKey(s)
				if t.mintPorts[k] == nil {
					t.mintPorts[k] = map[string]bool{}
					t.mintOrder = append(t.mintOrder, k)
				}
			}
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
			port := t.portOf(c)
			pt := t.partOf(s)
			if pt == nil {
				// No part type at all, so the cell was minted above and gains its interface here.
				t.mintPorts[t.mintKey(s)][port] = true
				continue
			}
			if t.usedPorts[pt] == nil {
				t.usedPorts[pt] = map[string]bool{}
			}
			t.usedPorts[pt][port] = true
		}
	}
	return t
}

// displayOr returns the name the instance should PRESENT: the source's own id when that had to be
// minted into a legal identifier, so the original survives as the rename's display string, and the
// identifier itself otherwise. nameExpr then writes a bare atom in the second case and a
// (rename id "display") in the first.
func (t *instanceTable) displayOr(s *ir.ComponentSection) string {
	if d := t.display[s]; d != "" {
		return d
	}
	return t.name[s]
}

// anchor resolves the instance a connection hangs off, reporting false when the design carries none.
//
// Provenance wins outright, because a reader that filled it recorded the source's own instance id.
// EDIF keeps the id there with an empty ComponentRef for an instance carrying no ref-des
// (TestUnresolvedRefIsStable). An id naming no instance in the contents is written back verbatim
// rather than re-anchored.
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
	// The IR does not record which SECTION of a multi-section component a connection belongs to.
	// Where the sections have different part types (a relay's coil and contacts) the pin decides.
	// Where they share one (a multi-gate IC) the first section is taken, which changes which gate the
	// file names and never which part the connection reaches.
	if len(secs) > 1 {
		if s := t.sectionDeclaring(secs, c.GetPinRef()); s != nil {
			return t.name[s], true
		}
	}
	return t.name[secs[0]], true
}

// mintKey names the cell a section references. A section naming no library (IPC-2581 records none)
// puts its cell in the work library, which is where the top cell is, so a bare cellRef resolves
// against it.
func (t *instanceTable) mintKey(s *ir.ComponentSection) cellRef {
	lib := s.GetLibraryRef()
	if lib == "" {
		lib = t.work
	}
	return cellRef{lib: lib, name: localName(s.GetLibraryRef(), s.GetPartRef())}
}

// mintedCells lists the cells of one library that nothing declares, in first-reference order so the
// output is deterministic.
func (t *instanceTable) mintedCells(lib string) []cellRef {
	var out []cellRef
	for _, k := range t.mintOrder {
		if k.lib == lib {
			out = append(out, k)
		}
	}
	return out
}

// mintedLibraries lists the library names holding minted cells, in first-reference order.
func (t *instanceTable) mintedLibraries() []string {
	var out []string
	seen := map[string]bool{}
	for _, k := range t.mintOrder {
		if !seen[k.lib] {
			seen[k.lib] = true
			out = append(out, k.lib)
		}
	}
	return out
}

// mintedPortsOf lists one minted cell's ports, sorted so the output is stable.
func (t *instanceTable) mintedPortsOf(k cellRef) []string {
	var out []string
	for p := range t.mintPorts[k] {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
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
// portRef has to name. A conforming reader drops a portRef naming a pin designator, finding no port
// by that name, and only our reader recovers it through netOf's no-mapping fallback (agni issue 580).
//
// The pin is returned unchanged when the part type declares no matching designator. That covers a
// design read from a board file, which has no part types, and an EDIF source whose interface carries
// no designators (dup_ports.edn), which has no mapping to rebuild.
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
// none or several do, since the caller has a defined fallback for an ambiguous answer.
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

// portRefExpr is the inverse of portName, which NORMALIZES rather than parses, stripping a leading &
// escape and rewriting a (member NAME IDX) bus pin to NAME[IDX].
//
// portName reads a rename by its IDENTIFIER and discards the display string, unlike parseName, so
// (rename P "with space") reads back as "P". The bare atom is therefore the ONLY form of a pin
// reference that reads back unchanged. A reference the tokenizer would not return as one atom is
// unrepresentable and is sanitized, the one place this writer changes a value rather than its
// spelling. No EDIF-sourced design reaches it, since portName built every PinRef in the first place.
//
// A pin literally named DATA[0] and a member of bus DATA are indistinguishable in the IR, so this
// picks the member form. Both read back to the same PinRef.
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
// An identifier that differs from its display name is what (rename ID "D") encodes, so that
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

// identOK reports whether s is already a legal EDIF identifier, meaning a letter or underscore
// followed by letters, digits and underscores, or the "&" escape for a name that would otherwise open
// with a digit. It is stricter than atomOK, which asks only whether a string survives the TOKENIZER.
// Raw KiCad UUIDs clear atomOK, and a third-party reader skipped all 1123 instances named by them
// (agni issue 582).
func identOK(s string) bool {
	if s == "" {
		return false
	}
	body := s
	if s[0] == '&' {
		body = s[1:]
	} else if !(s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z' || s[0] == '_') {
		return false
	}
	for _, r := range body {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
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

// edifString renders a string literal. The dialect has NO escape mechanism (sexpr.EDIFStrings reads
// to the closing quote), so a quote inside a value is dropped. Go's %q would emit a backslash escape
// the reader hands back verbatim, corrupting the value silently instead of narrowing it.
func edifString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, "") + `"`
}

func joinPrefixed(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// sortedKeys orders a map's keys so the output is deterministic, since committed captures compare
// the writer's bytes across runs.
func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
