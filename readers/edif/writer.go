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

// atomOK reports whether the tokenizer returns s as a single atom, which is the condition for writing
// a name bare rather than wrapping it in a rename.
//
// It is deliberately LOOSER than the EDIF identifier grammar. The shared s-expression tokenizer
// splits on whitespace and parens and treats a quote as a string delimiter, returning everything
// else as one atom, which is why the fixtures spell a numeric pin as a bare `1` rather than as the
// &1 escape the strict grammar would demand. Holding to the strict grammar here would wrap every
// such name in a rename and change its Prov.NativeId from "1" to "&1", failing the round-trip oracle
// for a difference that is purely cosmetic in the source.
func atomOK(s string) bool {
	return s != "" && !strings.ContainsAny(s, " \t\r\n()\"")
}

// emitter accumulates LINES rather than one growing buffer, because a close has to reach back and
// append its parens to the line already written -- EDIF trails a run of closing parens on the last
// leaf rather than giving them lines of their own. Reaching into a strings.Builder means rebuilding
// it, which is a full copy per close and quadratic over a real export; reaching into a slice is not.
type emitter struct {
	lines []string
	ind   int
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

	e.open("(edif %s", root)
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
	decl := fmt.Sprintf("(design %s (cellRef %s (libraryRef %s))", nameExpr(name, ""), top, work)
	inCell := hasCell(d, work, top)
	if inCell {
		e.line("%s)", decl)
	} else {
		e.open("%s", decl)
		e.contents(d)
		e.close(1)
	}
	for _, lib := range d.GetLibraries() {
		e.open("(library %s", nameExpr(lib.GetName(), ""))
		for _, pt := range lib.GetParts() {
			e.cell(pt, d, inCell && lib.GetName() == work && pt.GetName() == top)
		}
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
	// A pin with no name is not a pin the source declared. It is what partTypeOf produces for an
	// array port: parseName recognizes four name forms and (array DATA 8) is none of them, so the
	// port lands in the part type nameless while the bus itself is picked up separately as a
	// BusNotModeled diagnostic. Array declarations are not written (see WriteNetlist), so the pin
	// they produce is not either, and writer_test.go drops nameless pins from both sides for the
	// same reason. Writing one would be worse than dropping it: there is no name to write, so it
	// would come back named after whatever placeholder was invented for it.
	var pins []*ir.Pin
	for _, p := range pt.GetPins() {
		if p.GetName() != "" {
			pins = append(pins, p)
		}
	}
	if len(pins) == 0 {
		e.line("(interface)")
	} else {
		e.open("(interface")
		for _, p := range pins {
			e.port(p)
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
	e.line("(port %s%s)", nameExpr(p.GetName(), p.GetProv().GetNativeId()), joinPrefixed(parts))
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
	ref := s.GetPartRef()
	if lib := s.GetLibraryRef(); lib != "" {
		ref = fmt.Sprintf("%s (libraryRef %s)", ref, lib)
	}
	head := fmt.Sprintf("(instance %s (viewRef %s (cellRef %s))",
		instanceName(s), viewName, ref)
	// A designator-less instance is a real state the reader models (refdes.Unannotated reports it),
	// so an empty ref-des emits no designator rather than an empty one.
	if r := c.GetRefDes(); r != "" {
		head += fmt.Sprintf(" (designator %s)", edifString(r))
	}
	props := sortedKeys(s.GetAttributes())
	if len(props) == 0 {
		e.line("%s)", head)
		return
	}
	e.open("%s", head)
	for _, k := range props {
		// Every property is written as a string. Read collapses the string, integer and boolean
		// value forms into one map of strings (propValue), so the source's form is not recoverable
		// and the string form is the one that reads back to the same value.
		e.line("(property %s (string %s))", nameExpr(k, ""), edifString(s.GetAttributes()[k]))
	}
	e.close(1)
}

// instanceName recovers the instance identifier from provenance, which is where the reader put it.
// It is the join key nets reference through instanceRef, so a design whose sections carry none gets
// a deterministic one derived from the ref-des and section index rather than a counter.
func instanceName(s *ir.ComponentSection) string {
	if id := s.GetProv().GetNativeId(); id != "" {
		return id
	}
	return fmt.Sprintf("I%d", s.GetIndex())
}

func (e *emitter) net(n *ir.Net) {
	var refs []string
	for _, c := range n.GetConnections() {
		// The instance id lives in the connection's provenance, including for a connection whose id
		// resolved to no ref-des (a power symbol, an off-page marker, a top-level port). Those keep
		// the id here and an empty ComponentRef in the IR, so writing the id back is what keeps the
		// re-read stable (TestUnresolvedRefIsStable).
		if id := c.GetProv().GetNativeId(); id != "" {
			refs = append(refs, fmt.Sprintf("(portRef %s (instanceRef %s))", portRefExpr(c.GetPinRef()), id))
			continue
		}
		refs = append(refs, fmt.Sprintf("(portRef %s)", portRefExpr(c.GetPinRef())))
	}
	e.line("(net %s (joined %s))", nameExpr(n.GetName(), n.GetProv().GetNativeId()), strings.Join(refs, " "))
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
		return fmt.Sprintf("(rename %s %s)", nativeID, edifString(name))
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
