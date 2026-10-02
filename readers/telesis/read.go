// Package telesis reads the flat Telesis netlist (`.tel`) that the Mentor/Siemens schematic flow
// emits into the neutral IR. The format carries connectivity and properties and no geometry, so
// this is a Design-only reader (CONSTRAINTS C21).
//
// Fidelity: lossy-bounded (CONSTRAINTS C6). Read extracts the components and their part types (with
// the manufacturer part number the `$PACKAGES` head carries), the nets and their pin membership, and
// both property blocks transposed onto the components and pins they name. It drops the file's own
// ordering and grouping, because a property is stored on each component it named and the
// inverted-index entry it arrived on is not recoverable. There is no geometry to lose.
//
// The sections, the entry grammar and the IR mapping are in GRAMMAR.md beside this file. Three
// rules there produce a design that parses cleanly and is wrong when missed: only a `!` entry is a
// package (addEntry), a property target is routed by its SHAPE and not its block (pinTargetRe), and
// a property section is an inverted index that has to be transposed (addProperty).
package telesis

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// SourceFormat tags designs this reader produces.
const SourceFormat = "telesis"

// Section markers this reader consumes. Any other `$SECTION` is skipped rather than treated as an
// error, since the grammar is learned from real exports and another writer may emit sections this
// one has never seen. noteUnparsed records the skip.
const (
	secPackages   = "$PACKAGES"
	secProperties = "$A_PROPERTIES"
	secNets       = "$NETS"
	secEnd        = "$END"
)

// generatedNamePrefix marks a net name the exporter invented. Hundreds appear in a real export,
// nearly all single-endpoint. They are KEPT and tagged, because dropping them would also drop every
// genuinely dangling connection the single-pin-net rule reports. Filtering is left to consumers.
const generatedNamePrefix = "$"

// GeneratedNameAttr is set to "true" on a net whose name the exporter generated.
const GeneratedNameAttr = "generated_name"

// PinTypeProperty is the property name carrying pin direction, the ONLY place this format states
// it. Skip the property sections and every pin reads PIN_DIRECTION_UNSPECIFIED, which silently
// disables every direction-dependent rule.
const PinTypeProperty = "Pin Type"

// PinLabelProperty carries the pin's printed label.
const PinLabelProperty = "PinLabel"

// UnparsedSectionsAttr names the design attribute listing sections that carried content this reader
// did not consume, comma-separated and in file order. Absent when everything was consumed.
const UnparsedSectionsAttr = "telesis.unparsed_sections"

// pinTargetRe matches a pin-scoped target, REFDES.PIN. The DOT is the discriminator, since a
// ref-des never contains one. The pin half is permissive on purpose (U1.14, BGA U7.L1, connector
// shell J1.A), because narrowing it to digits would route a letter pin into COMPONENT scope as an
// attribute on a component that does not exist.
var pinTargetRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\.([A-Za-z0-9_]+)$`)

// splitEntryLine parses one entry line into its quoted head fields and its target run. The five
// head shapes it accepts are listed in GRAMMAR.md section 2. It is a scanner rather than a regex
// because a line a pattern fails to match is skipped, and a skipped line also breaks the
// continuation chain, so the entry's whole target run vanishes with it.
//
// Quoting is respected while scanning for `;` and `!`. Returns ok=false for a line that opens no
// entry, which the caller treats as a continuation or as content to skip.
func splitEntryLine(line string) (fields []string, bang bool, targets string, ok bool) {
	var cur strings.Builder
	inQuote, sawQuote, started := false, false, false
	i := 0
	for ; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if c == '\'' {
				inQuote = false
				fields = append(fields, cur.String())
				cur.Reset()
				sawQuote = false
				continue
			}
			cur.WriteByte(c)
			continue
		}
		switch c {
		case '\'':
			inQuote, sawQuote, started = true, true, true
			// A bang before the FIRST field is the MPN-only package entry. Record the empty
			// field zero so callers index fields positionally.
			if bang && len(fields) == 0 {
				fields = append(fields, "")
			}
		case '!':
			bang, started = true, true
		case ';':
			if !started {
				return nil, false, "", false
			}
			return fields, bang, line[i+1:], true
		case ' ', '\t', '\r':
		default:
			// Anything else outside a quote is not part of an entry head.
			return nil, false, "", false
		}
		_ = sawQuote
	}
	return nil, false, "", false
}

// Read parses a Telesis netlist into an ir.Design. sourceFile is used for provenance only; this
// function never opens a file (CONSTRAINTS C1, I/O lives at the edge).
func Read(r io.Reader, sourceFile string) (*ir.Design, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("telesis %s: %w", sourceFile, err)
	}
	b := newBuilder(sourceFile)
	if err := b.parse(decodeText(raw)); err != nil {
		return nil, err
	}
	return b.design(), nil
}

// decodeText turns the file's bytes into a string. Valid UTF-8 is taken as is and anything else is
// read as latin-1, which real exports carry and which cannot fail. Without it Go substitutes U+FFFD
// for each invalid byte and corrupts the identifier it sat in.
func decodeText(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}

// IsTelesis reports whether the content looks like a Telesis netlist, for the registry's sniffing
// path. It requires a line-initial $NETS or $PACKAGES, which no other format this engine reads has.
func IsTelesis(head []byte) bool {
	for _, line := range strings.Split(string(head), "\n") {
		switch strings.TrimSpace(line) {
		case secNets, secPackages:
			return true
		}
	}
	return false
}

// entry is one logical record: a head, its optional second quoted field, and its target run, with
// continuation lines already folded in.
type entry struct {
	head  string
	value string
	// extra holds head fields beyond the second, such as a value or tolerance after the MPN. The
	// format never states their meaning, so they are kept by position and not interpreted.
	extra   []string
	hasBang bool
	targets []string
}

// builder accumulates the design as sections are read.
type builder struct {
	source string

	// partOf maps ref-des to the part identity it was declared under, so a property entry can
	// reach the part type a component belongs to.
	partOf   map[string]string
	partMPN  map[string]string
	partOrd  []string
	refOrder []string
	refSeen  map[string]bool
	// dupRefs records a ref-des declared under more than one package entry.
	dupRefs []string

	// unnamedParts records part types identified by MPN because the entry carried no part name.
	unnamedParts map[string]bool
	// partExtra holds a package entry's positional fields beyond the MPN.
	partExtra map[string][]string
	// partPackage holds the package name of a part type keyed apart from it, because the package
	// was already declared with a different MPN (see addPackage).
	partPackage map[string]string

	compAttrs map[string]map[string]string
	// pinAttrs is keyed by ref-des then pin designator. Pin facts arrive per INSTANCE and are
	// reconciled onto the shared part type in partType.
	pinAttrs map[string]map[string]map[string]string

	nets     []*ir.Net
	netNames map[string]bool

	// unparsed records sections that carried content this reader did not consume, in file order.
	// See noteUnparsed.
	unparsed     []string
	unparsedSeen map[string]bool
}

func newBuilder(source string) *builder {
	return &builder{
		source:       source,
		partOf:       map[string]string{},
		partMPN:      map[string]string{},
		refSeen:      map[string]bool{},
		unnamedParts: map[string]bool{},
		partExtra:    map[string][]string{},
		partPackage:  map[string]string{},
		compAttrs:    map[string]map[string]string{},
		pinAttrs:     map[string]map[string]map[string]string{},
		netNames:     map[string]bool{},
		unparsedSeen: map[string]bool{},
	}
}

// parse walks the file section by section.
func (b *builder) parse(text string) error {
	var current string
	var pending []string

	flush := func() {
		if len(pending) == 0 {
			return
		}
		for _, e := range foldEntries(pending) {
			b.addEntry(current, e)
		}
		pending = nil
	}

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, "$") {
			flush()
			current = strings.Fields(trimmed)[0]
			if current == secEnd {
				break
			}
			continue
		}
		if current == "" || trimmed == "" {
			continue
		}
		pending = append(pending, line)
	}
	flush()

	if len(b.refOrder) == 0 && len(b.nets) == 0 {
		return fmt.Errorf("telesis %s: no $PACKAGES or $NETS content; not a Telesis netlist", b.source)
	}
	return nil
}

// foldEntries turns raw lines into logical entries, joining a line onto the previous one when that
// previous line ended with a comma. The comma is the format's only continuation marker, so a run of
// targets may span any number of lines.
func foldEntries(lines []string) []entry {
	var out []entry
	var cur *entry
	continuing := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if trimmed == "" {
			continue
		}
		next := strings.HasSuffix(trimmed, ",")

		if continuing && cur != nil {
			cur.targets = append(cur.targets, splitTargets(trimmed)...)
			continuing = next
			continue
		}

		fields, bang, targets, ok := splitEntryLine(line)
		if !ok {
			// A line that opens no entry and continues none is skipped rather than failing the
			// read, since the grammar is learned from exports and not a specification.
			continuing = false
			cur = nil
			continue
		}
		e := entry{hasBang: bang, targets: splitTargets(targets)}
		if len(fields) > 0 {
			e.head = fields[0]
		}
		if len(fields) > 1 {
			e.value = fields[1]
		}
		if len(fields) > 2 {
			e.extra = fields[2:]
		}
		out = append(out, e)
		cur = &out[len(out)-1]
		continuing = next
	}
	return out
}

// splitTargets breaks a target run into tokens, dropping the continuation commas.
func splitTargets(s string) []string {
	var out []string
	for _, f := range strings.Fields(s) {
		if t := strings.Trim(f, ","); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// addEntry routes one entry by the section it appeared in.
func (b *builder) addEntry(section string, e entry) {
	switch section {
	case secPackages:
		// Only a `!` entry is a package. Reading a non-bang entry here as one would invent a
		// component named after a property.
		if e.hasBang {
			b.addPackage(e)
		}
	case secNets:
		b.addNet(e)
	case secProperties:
		b.addProperty(e)
	default:
		b.noteUnparsed(section)
	}
}

// noteUnparsed records a section that carried content this reader did not consume.
//
// $PINS is the case in hand. It is EMPTY in every real export examined, so its body grammar is
// unknown and the reader skips it. Without this record, an export that populates it would parse
// cleanly and lose its content with no signal. It is an attribute on the design and not an error,
// because the connectivity that was read is still correct, only incomplete.
func (b *builder) noteUnparsed(section string) {
	if section == "" || b.unparsedSeen[section] {
		return
	}
	b.unparsedSeen[section] = true
	b.unparsed = append(b.unparsed, section)
}

// addPackage records a part type and the components declared under it.
//
// An entry with no part name is identified by its MPN, which keeps two unnamed parts with
// different MPNs distinct. The substitution is recorded as name_from_mpn on the part type.
//
// Two entries may share a package (footprint) name with different MPNs, parts that share a
// footprint. Each keeps its own MPN: the first takes the package name, and a later one is keyed
// "<package>:<mpn>" with the package recorded as its `package` attribute (agni issue 835). Keying by
// the package alone gave every part of the later entry the first entry's MPN, with no warning.
func (b *builder) addPackage(e entry) {
	part, named := e.head, true
	if part == "" {
		part, named = e.value, false
	}
	if part == "" {
		return
	}
	if mpn, seen := b.partMPN[part]; seen && named && mpn != e.value {
		pkg := part
		part = pkg + ":" + e.value
		b.partPackage[part] = pkg
	}
	if !named {
		b.unnamedParts[part] = true
	}
	if len(e.extra) > 0 {
		b.partExtra[part] = e.extra
	}
	if _, ok := b.partMPN[part]; !ok {
		b.partMPN[part] = e.value
		b.partOrd = append(b.partOrd, part)
	}
	for _, ref := range e.targets {
		if b.refSeen[ref] {
			b.dupRefs = append(b.dupRefs, ref)
			continue
		}
		b.refSeen[ref] = true
		b.refOrder = append(b.refOrder, ref)
		b.partOf[ref] = part
	}
}

// addNet records a net and its connections.
func (b *builder) addNet(e entry) {
	name := e.head
	if name == "" || b.netNames[name] {
		return
	}
	b.netNames[name] = true

	net := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: b.source}}
	if strings.HasPrefix(name, generatedNamePrefix) {
		net.Attributes = map[string]string{GeneratedNameAttr: "true"}
	}
	for _, t := range e.targets {
		m := pinTargetRe.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		net.Connections = append(net.Connections, &ir.Connection{
			ComponentRef: m[1],
			PinRef:       m[2],
			Prov:         &ir.Provenance{SourceFile: b.source},
		})
	}
	b.nets = append(b.nets, net)
}

// addProperty transposes one inverted-index entry onto its targets. A `refdes.pin` target is a pin
// fact and a bare `refdes` is a component fact. Routing by shape and not by block means a file that
// orders or merges the two property blocks differently still reads correctly.
func (b *builder) addProperty(e entry) {
	if e.head == "" {
		return
	}
	for _, t := range e.targets {
		if m := pinTargetRe.FindStringSubmatch(t); m != nil {
			ref, pin := m[1], m[2]
			if b.pinAttrs[ref] == nil {
				b.pinAttrs[ref] = map[string]map[string]string{}
			}
			if b.pinAttrs[ref][pin] == nil {
				b.pinAttrs[ref][pin] = map[string]string{}
			}
			b.pinAttrs[ref][pin][e.head] = e.value
			continue
		}
		if b.compAttrs[t] == nil {
			b.compAttrs[t] = map[string]string{}
		}
		b.compAttrs[t][e.head] = e.value
	}
}

// design assembles the accumulated state into the IR.
func (b *builder) design() *ir.Design {
	d := &ir.Design{
		IrVersion:    "0",
		SourceFormat: SourceFormat,
		Prov:         &ir.Provenance{SourceFile: b.source},
	}

	lib := &ir.PartLibrary{Name: SourceFormat, Prov: &ir.Provenance{SourceFile: b.source}}
	for _, part := range b.partOrd {
		lib.Parts = append(lib.Parts, b.partType(part))
	}
	if len(lib.Parts) > 0 {
		d.Libraries = []*ir.PartLibrary{lib}
	}

	for _, ref := range b.refOrder {
		c := &ir.Component{
			RefDes: ref,
			Prov:   &ir.Provenance{SourceFile: b.source},
			Sections: []*ir.ComponentSection{{
				Index:      0,
				PartRef:    b.partOf[ref],
				LibraryRef: SourceFormat,
			}},
		}
		if attrs := b.compAttrs[ref]; len(attrs) > 0 {
			c.Attributes = attrs
		}
		d.Components = append(d.Components, c)
	}

	d.Nets = b.nets
	if len(b.unparsed) > 0 {
		d.Attributes = map[string]string{UnparsedSectionsAttr: strings.Join(b.unparsed, ",")}
	}
	d.InputDiagnostics = b.diagnostics()
	return d
}

// partType builds one part type, folding every instance's pin facts onto the shared pin list.
//
// Pin facts arrive per INSTANCE (`U1.14 'Pin Type' 'IN'`) while the IR hangs pins off the shared
// part type. Where two instances disagree the first wins, and the disagreement is recorded on the
// pin (see applyPinAttrs) so a library inconsistency stays findable.
func (b *builder) partType(part string) *ir.PartType {
	pt := &ir.PartType{
		Name: part,
		Kind: SourceFormat,
		Prov: &ir.Provenance{SourceFile: b.source},
	}
	if mpn := b.partMPN[part]; mpn != "" {
		pt.Mpn = mpn
	}
	for i, v := range b.partExtra[part] {
		if v == "" {
			continue
		}
		if pt.Attributes == nil {
			pt.Attributes = map[string]string{}
		}
		// Named by 1-based position in the head, so field_3 follows the part name and MPN.
		pt.Attributes[fmt.Sprintf("field_%d", i+3)] = v
	}
	if b.unnamedParts[part] {
		if pt.Attributes == nil {
			pt.Attributes = map[string]string{}
		}
		pt.Attributes["name_from_mpn"] = "true"
	}
	if pkg := b.partPackage[part]; pkg != "" {
		if pt.Attributes == nil {
			pt.Attributes = map[string]string{}
		}
		pt.Attributes["package"] = pkg
	}

	byDesignator := map[string]*ir.Pin{}
	var order []string
	for _, ref := range b.refOrder {
		if b.partOf[ref] != part {
			continue
		}
		pins := b.pinAttrs[ref]
		designators := make([]string, 0, len(pins))
		for pin := range pins {
			designators = append(designators, pin)
		}
		sort.Strings(designators)
		for _, pin := range designators {
			attrs := pins[pin]
			p, ok := byDesignator[pin]
			if !ok {
				p = &ir.Pin{Designator: pin, Prov: &ir.Provenance{SourceFile: b.source}}
				byDesignator[pin] = p
				order = append(order, pin)
			}
			applyPinAttrs(p, attrs)
		}
	}

	sort.Slice(order, func(i, j int) bool { return naturalLess(order[i], order[j]) })
	for _, pin := range order {
		pt.Pins = append(pt.Pins, byDesignator[pin])
	}
	return pt
}

// applyPinAttrs folds one instance's facts onto a shared pin, first-writer-wins with the conflict
// recorded.
func applyPinAttrs(p *ir.Pin, attrs map[string]string) {
	for name, value := range attrs {
		switch name {
		case PinTypeProperty:
			raw := p.GetAttributes()["direction_raw"]
			if raw == "" {
				setPinAttr(p, "direction_raw", value)
				p.Direction = pinDirection(value)
				continue
			}
			// Case alone is not a conflict. One real export spells the same value both ANALOG
			// and Analog.
			if !strings.EqualFold(raw, value) {
				setPinAttr(p, "direction_conflict", raw+"|"+value)
			}
		case PinLabelProperty:
			if p.Name == "" {
				p.Name = value
			}
		default:
			setPinAttr(p, name, value)
		}
	}
}

func setPinAttr(p *ir.Pin, k, v string) {
	if p.Attributes == nil {
		p.Attributes = map[string]string{}
	}
	p.Attributes[k] = v
}

// pinDirection maps the format's Pin Type vocabulary onto the IR enum.
//
// Case is folded because one real export spells a value two ways (ANALOG and Analog). GROUND maps
// to POWER_IN like POWER, since a ground pin consumes from its net and POWER_OUT is for a source.
// TERMINAL and ANALOG map to PASSIVE so direction-based rules treat them as conducting legs and not
// as logic inputs. An unrecognised value returns UNSPECIFIED, and the caller keeps the raw
// spelling in direction_raw.
func pinDirection(v string) ir.PinDirection {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "IN":
		return ir.PinDirection_PIN_DIRECTION_INPUT
	case "OUT":
		return ir.PinDirection_PIN_DIRECTION_OUTPUT
	case "BI":
		return ir.PinDirection_PIN_DIRECTION_INOUT
	case "POWER", "GROUND":
		return ir.PinDirection_PIN_DIRECTION_POWER_IN
	case "TERMINAL", "ANALOG":
		return ir.PinDirection_PIN_DIRECTION_PASSIVE
	}
	return ir.PinDirection_PIN_DIRECTION_UNSPECIFIED
}

// diagnostics declares what this reader looked for, whether or not it found anything.
//
// This format can only answer ref_des_collisions, so that is the one it supplies. The others are
// properties of a drawing it does not carry. Why `supplied` is declared even when the list is empty
// is in docsite/content/architecture/ingestion-and-ir.md#input-diagnostics (agni issue 309).
func (b *builder) diagnostics() *ir.InputDiagnostics {
	diag := &ir.InputDiagnostics{Supplied: []string{"ref_des_collisions"}}
	for _, ref := range b.dupRefs {
		diag.RefDesCollisions = append(diag.RefDesCollisions, &ir.RefDesCollision{
			RefDes:    ref,
			Instances: []*ir.Provenance{{SourceFile: b.source}},
		})
	}
	return diag
}

// naturalLess orders pin designators so 2 sorts before 10, and A1 before A2 before B1.
func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ad, bd := isDigit(a[ai]), isDigit(b[bi])
		if ad && bd {
			as, bs := ai, bi
			for ai < len(a) && isDigit(a[ai]) {
				ai++
			}
			for bi < len(b) && isDigit(b[bi]) {
				bi++
			}
			an, bn := strings.TrimLeft(a[as:ai], "0"), strings.TrimLeft(b[bs:bi], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			continue
		}
		if a[ai] != b[bi] {
			return a[ai] < b[bi]
		}
		ai++
		bi++
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
