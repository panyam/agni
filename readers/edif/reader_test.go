package edif

import (
	"bytes"
	"strings"
	"testing"

	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func readEDN(t *testing.T, name string) *ir.Design {
	t.Helper()
	d, err := Read(bytes.NewReader(readFixture(t, name)), name)
	if err != nil {
		t.Fatalf("Read %s: %v", name, err)
	}
	return d
}

// parseNode tokenizes a single name form (written as source text) into its *node, so the
// grammar oracles exercise the real parser rather than a hand-built tree. The form is wrapped
// in parens and the first child returned, matching how a name sits as arg(1) of a net/cell.
func parseNode(t *testing.T, src string) *node {
	t.Helper()
	root, err := parse(strings.NewReader("(" + src + ")"))
	if err != nil {
		t.Fatalf("parseNode %q: %v", src, err)
	}
	return root.Arg(0)
}

func compByRef(d *ir.Design, ref string) *ir.Component {
	for _, c := range d.Components {
		if c.RefDes == ref {
			return c
		}
	}
	return nil
}

func netByName(d *ir.Design, name string) *ir.Net {
	for _, n := range d.Nets {
		if n.Name == name {
			return n
		}
	}
	return nil
}

// hasConn reports whether net has a connection with the given component ref and pin.
func hasConn(n *ir.Net, ref, pin string) bool {
	if n == nil {
		return false
	}
	for _, c := range n.Connections {
		if c.ComponentRef == ref && c.PinRef == pin {
			return true
		}
	}
	return false
}

// TestReadNetlist is the baseline: normal parse, section grouping, connectivity.
// TestNormalizeMPN (WS3-075): an OrCAD export carries the part number under a Manufacturer_PN
// property (renamed to display "Manufacturer PN", or bare), not "MPN", so the datasheet join read
// 0 MPNs. The reader now normalizes both spellings to the canonical "MPN" attribute, and an
// explicit "MPN" wins over the alias.
func TestNormalizeMPN(t *testing.T) {
	d := readEDN(t, "mpn.edn")
	for ref, want := range map[string]string{
		"U1": "PARTX", // renamed property -> display key "Manufacturer PN"
		"U2": "PARTY", // bare property -> id key "Manufacturer_PN"
		"U3": "PARTZ", // explicit MPN wins over the alias's "IGNORED"
	} {
		if c := compByRef(d, ref); c == nil || c.Attributes["MPN"] != want {
			t.Errorf("%s MPN = %q, want %q", ref, c.Attributes["MPN"], want)
		}
	}
	// A component with no part-number property gets no synthesized MPN (empty, not guessed).
	if r1 := compByRef(d, "R1"); r1 == nil || r1.Attributes["MPN"] != "" {
		t.Errorf("R1 MPN = %q, want empty (no alias property present)", r1.Attributes["MPN"])
	}
}

func TestReadNetlist(t *testing.T) {
	d := readEDN(t, "basic.edn")
	if d.SourceFormat != "edif-2.0.0" || d.IrVersion != "0" {
		t.Errorf("source_format=%q ir_version=%q, want edif-2.0.0 / 0", d.SourceFormat, d.IrVersion)
	}
	if len(d.Components) != 3 {
		t.Fatalf("components = %d, want 3 (R1, R2, U1)", len(d.Components))
	}
	if u1 := compByRef(d, "U1"); u1 == nil || len(u1.Sections) != 2 {
		t.Errorf("U1 = %v, want 2 sections (U1A + U1B)", u1)
	}
	if !hasConn(netByName(d, "GND"), "R1", "2") || !hasConn(netByName(d, "GND"), "R2", "2") {
		t.Errorf("GND connections = %v, want R1.2 + R2.2", netByName(d, "GND"))
	}

	// Provenance is recorded on the design and every extracted entity: the source file on
	// all, plus the EDIF native-id kind on library-scoped entities (parts/nets) so a later
	// re-export can be reconciled against the original ids.
	if d.Prov.GetSourceFile() != "basic.edn" {
		t.Errorf("design prov source = %q, want basic.edn", d.Prov.GetSourceFile())
	}
	if r1 := compByRef(d, "R1"); r1.GetProv().GetSourceFile() != "basic.edn" {
		t.Errorf("R1 prov source = %q, want basic.edn", r1.GetProv().GetSourceFile())
	}
	if gnd := netByName(d, "GND"); gnd.GetProv().GetSourceFile() != "basic.edn" ||
		gnd.GetProv().GetNativeIdKind() != "edif-rename-id" {
		t.Errorf("GND prov = %+v, want source basic.edn / kind edif-rename-id", gnd.GetProv())
	}
}

// TestReadSchematicNetlist: an EDIF SCHEMATIC (.eds) export wraps designators and property
// values in (stringDisplay "V" ...); the netlist reader must unwrap that form the same way the
// geometry reader does (refDesOf/propText), or every .eds component reads with an empty ref_des,
// no MPN, and no property values (WS1-046). The .edn writes bare atoms, so this is additive there.
func TestReadSchematicNetlist(t *testing.T) {
	d := readEDN(t, "schematic-netlist.eds")

	u7 := compByRef(d, "U7")
	if u7 == nil {
		t.Fatalf("component U7 not found (stringDisplay instance designator not unwrapped); got %d components", len(d.Components))
	}
	if got := u7.Attributes["MPN"]; got != "PART-123" {
		t.Errorf("U7 MPN = %q, want PART-123 (normalized from a stringDisplay Manufacturer PN)", got)
	}
	if got := u7.Attributes["Value"]; got != "10k" {
		t.Errorf("U7 Value = %q, want 10k (stringDisplay property value)", got)
	}

	// PartType identity comes from the cell, whose designator prefix is stringDisplay-wrapped on
	// the schematic view too.
	if len(d.Libraries) == 0 || len(d.Libraries[0].Parts) == 0 {
		t.Fatalf("no part types extracted")
	}
	if pfx := d.Libraries[0].Parts[0].DesignatorPrefix; pfx != "U" {
		t.Errorf("PartA designator prefix = %q, want U", pfx)
	}

	// Pin identity on a net comes from the instance's portInstance designators (physical pin
	// numbers), which the schematic view wraps as well: port "1" maps to physical pin "3".
	if !hasConn(netByName(d, "SIGA"), "U7", "3") {
		t.Errorf("SIGA should carry U7.3 (portInstance stringDisplay designator), got %v", netByName(d, "SIGA"))
	}
	if !hasConn(netByName(d, "SIGB"), "U7", "5") {
		t.Errorf("SIGB should carry U7.5, got %v", netByName(d, "SIGB"))
	}
}

// TestCellMPNFallback (WS1-046 Piece B): OrCAD carries a shared part-type's Manufacturer_PN on the
// CELL (the cell is named by the part number), not on every placed instance. A component with no
// inline MPN must inherit its cell's; an inline MPN still wins; a cell with no MPN leaves the
// component's MPN empty (no false fill). The join keys on the section's PartRef, resolving the cell
// by either its display name or its &-stripped native id.
func TestCellMPNFallback(t *testing.T) {
	d := readEDN(t, "cell-mpn.edn")
	if got := compByRef(d, "U1").Attributes["MPN"]; got != "CELL-MPN-1" {
		t.Errorf("U1 MPN = %q, want CELL-MPN-1 (inherited from its cell, keyed by &-stripped native id)", got)
	}
	if got := compByRef(d, "U2").Attributes["MPN"]; got != "INLINE-MPN-2" {
		t.Errorf("U2 MPN = %q, want INLINE-MPN-2 (inline MPN wins over the cell)", got)
	}
	if got := compByRef(d, "R1").Attributes["MPN"]; got != "" {
		t.Errorf("R1 MPN = %q, want empty (its cell carries no MPN, so no fallback)", got)
	}
}

// TestSchematicCellMPNFallback (WS1-046 Piece A + B combined): the real-world .eds case where the
// shared cell's Manufacturer_PN is stringDisplay-wrapped AND the instance carries no inline MPN, so
// the value must be BOTH unwrapped (Piece A) and inherited via the cell join (Piece B). The
// cellRef here names the part by DISPLAY name, exercising the name key of the dual-keyed index. An
// inline MPN on the .eds view still wins.
func TestSchematicCellMPNFallback(t *testing.T) {
	d := readEDN(t, "schematic-cell-mpn.eds")
	u9 := compByRef(d, "U9")
	if u9 == nil {
		t.Fatalf("component U9 not found; got %d components", len(d.Components))
	}
	if got := u9.Attributes["MPN"]; got != "CELL-PART-9" {
		t.Errorf("U9 MPN = %q, want CELL-PART-9 (stringDisplay cell property, inherited via the cell join)", got)
	}
	if got := compByRef(d, "U10").Attributes["MPN"]; got != "INLINE-10" {
		t.Errorf("U10 MPN = %q, want INLINE-10 (inline MPN wins over the cell on the schematic view too)", got)
	}
}

// TestUnresolvedRefIsStable: a portRef to an instance with no ref_des (a power symbol) must
// NOT be keyed by the export-unstable internal id; it becomes the "" no-ref marker.
func TestUnresolvedRefIsStable(t *testing.T) {
	d := readEDN(t, "edge.edn")
	gnd := netByName(d, "GND")
	if !hasConn(gnd, "R1", "2") {
		t.Errorf("GND missing R1.2: %v", gnd)
	}
	if !hasConn(gnd, "", "A") {
		t.Errorf("GND should carry the power-symbol pin as no-ref (\"\", \"A\"), not the internal &id; got %v", gnd)
	}
	// The unstable internal id must not leak into the connection key.
	if hasConn(gnd, "&pwr", "A") {
		t.Error("connection keyed on the export-unstable internal id &pwr")
	}
}

// TestMemberPort: (portRef (member NAME IDX) ...) must yield a stable bus-pin identity.
func TestMemberPort(t *testing.T) {
	d := readEDN(t, "edge.edn")
	if !hasConn(netByName(d, "DATABUS"), "U1", "DATA[3]") {
		t.Errorf("DATABUS should carry U1.DATA[3] (member port), got %v", netByName(d, "DATABUS"))
	}
}

// TestHierarchyDetected: a design whose top cell instantiates a sub-cell with its own
// contents is flagged, and extraction is scoped to the top cell (no sub-cell instances merged).
func TestHierarchyDetected(t *testing.T) {
	d := readEDN(t, "hier.edn")
	if d.Attributes["edif_hierarchical"] != "true" {
		t.Errorf("edif_hierarchical = %q, want true", d.Attributes["edif_hierarchical"])
	}
	if compByRef(d, "X1") != nil {
		t.Error("sub-cell instance X1 must not be merged into the top netlist")
	}
	if compByRef(d, "U1") == nil {
		t.Error("top-cell instance U1 missing")
	}
}

// TestParseNameForms is the grammar oracle: one row per EDIF name form parseName accepts.
// A form the parser stops recognizing (or a new form added without a row) shows up here,
// which is the guard the hand-rolled predecessor lacked (WS1-026). parseNode reuses the real
// tokenizer so a row is written as source text, not a hand-built node tree.
func TestParseNameForms(t *testing.T) {
	for _, tc := range []struct {
		form        string
		wantID      string
		wantDisplay string
		wantBest    string
	}{
		{"FOO", "FOO", "FOO", "FOO"},                                                       // bare atom
		{`(rename FOO "Display")`, "FOO", "Display", "Display"},                            // simple rename
		{"(name FOO (display (origin (pt 0 0))))", "FOO", "", "FOO"},                       // name + display
		{`(rename (name &X (display (origin (pt 0 0)))) "+3.3V")`, "&X", "+3.3V", "+3.3V"}, // nested
		{"(member BUS 3)", "", "", ""},                                                     // member: not an entity name
		{"(unknown FORM)", "", "", ""},                                                     // unrecognized -> empty
	} {
		got := parseName(parseNode(t, tc.form))
		if got.ID != tc.wantID || got.Display != tc.wantDisplay || got.best() != tc.wantBest {
			t.Errorf("parseName(%s) = {ID:%q Display:%q best:%q}, want {ID:%q Display:%q best:%q}",
				tc.form, got.ID, got.Display, got.best(), tc.wantID, tc.wantDisplay, tc.wantBest)
		}
	}
}

// TestPortNameForms is the grammar oracle for the pin-identity projection: the & escape is
// stripped and a (member NAME IDX) becomes NAME[IDX], where parseName leaves both verbatim.
func TestPortNameForms(t *testing.T) {
	for _, tc := range []struct{ form, want string }{
		{"&5", "5"},                    // bare atom, & stripped
		{"CLK", "CLK"},                 // bare atom, no escape
		{`(rename &7 "Display")`, "7"}, // rename, inner id & stripped
		{"(member DATA 3)", "DATA[3]"}, // bus element
		{"(unknown FORM)", ""},         // unrecognized -> empty
	} {
		if got := portName(parseNode(t, tc.form)); got != tc.want {
			t.Errorf("portName(%s) = %q, want %q", tc.form, got, tc.want)
		}
	}
}

// TestNetNameForms: every EDIF net-name form resolves to a non-empty name. The two
// (name ...) forms are what OrCAD's cap2edif emits (the dominant net form in the corpus);
// before WS1-026 they dropped to an empty name and vanished from diff/check keying.
func TestNetNameForms(t *testing.T) {
	d := readEDN(t, "names.edn")
	for _, tc := range []struct{ ref, pin, want string }{
		{"R1", "1", "PLAIN"},   // bare atom
		{"R1", "2", "SIG/A"},   // (rename ID "Display")
		{"R1", "3", "ACT_LED"}, // (name ID (display ...)) -- id is the name, no string
		{"R1", "4", "+3.3V"},   // (rename (name ID (display ...)) "Display")
	} {
		if n := netByName(d, tc.want); !hasConn(n, tc.ref, tc.pin) {
			t.Errorf("net %q missing %s.%s (name form not resolved): %v", tc.want, tc.ref, tc.pin, n)
		}
	}
	for _, n := range d.Nets {
		if n.Name == "" {
			t.Errorf("net with empty name (unresolved form): %v", n)
		}
	}
}

// TestWrappedName: a hard-wrap newline inside a quoted name (cap2edi splits the file at a
// fixed column mid-token) is joined, so identities carry no control characters (WS1-026).
func TestWrappedName(t *testing.T) {
	d := readEDN(t, "wrapped.edn")
	if d.Name != "SCHEMATIC1" {
		t.Errorf("design name = %q, want %q (wrap newline not joined)", d.Name, "SCHEMATIC1")
	}
	if strings.ContainsAny(d.Name, "\r\n") {
		t.Errorf("design name %q contains a control character", d.Name)
	}
}

// A cell whose EDIF id is numeric is escaped with a leading & in the source (&87844225); the instance
// references it by that escaped id, but the part-type is NAMED with the id un-escaped (parseName strips
// the &). If instanceOf resolves the cellRef differently from how the part-type is named + indexed, the
// component never links to its part-type and has NO part-type pins — connected (its net portRefs
// resolve) but invisible to every pin-level rule. This asserts the instance -> part-type link resolves.
func TestEscapedCellIdLinksToPartType(t *testing.T) {
	d := readEDN(t, "escaped-cell-id.edn")

	// Index part-types exactly as the model does (classify.PartIndex): "library/name" and "/name".
	parts := map[string]*ir.PartType{}
	for _, lib := range d.Libraries {
		for _, p := range lib.Parts {
			parts[lib.Name+"/"+p.Name] = p
			parts["/"+p.Name] = p
		}
	}

	c := compByRef(d, "U1")
	if c == nil || len(c.Sections) == 0 {
		t.Fatal("U1 not read")
	}
	sec := c.Sections[0]
	pt := parts[sec.LibraryRef+"/"+sec.PartRef]
	if pt == nil {
		pt = parts["/"+sec.PartRef]
	}
	if pt == nil {
		t.Fatalf("U1 (cellRef %q, lib %q) did not resolve to a part-type — the & escape broke the link",
			sec.PartRef, sec.LibraryRef)
	}
	var vcc *ir.Pin
	for _, pin := range pt.Pins {
		if pin.Name == "VCC" {
			vcc = pin
		}
	}
	if vcc == nil {
		t.Fatalf("resolved part-type has no VCC pin: %+v", pt.Pins)
	}
	if vcc.Designator != "1" {
		t.Errorf("VCC pin designator = %q, want 1", vcc.Designator)
	}
}

// TestRenameCellIdResolvesPins (WS1-045): a cell `(rename ID "Display")` whose display DIFFERS from its
// id and carries no & escape is keyed by Display, but the instance references it by the ID — so the
// section only resolves through the native-id alias PartIndex now adds. Before the fix the part's pins
// were silently dropped (the real MC2016Z50 oscillator surfaced 0 pins). Assert the section resolves via
// the ACTUAL classify.PartIndex (not a hand-rolled one) and its Vcc pin is present.
func TestRenameCellIdResolvesPins(t *testing.T) {
	d := readEDN(t, "rename-cell-id.edn")
	parts := classify.PartIndex(d)

	c := compByRef(d, "X1")
	if c == nil || len(c.Sections) == 0 {
		t.Fatal("X1 not read")
	}
	sec := c.Sections[0]
	pt := parts[sec.LibraryRef+"/"+sec.PartRef]
	if pt == nil {
		pt = parts["/"+sec.PartRef]
	}
	if pt == nil {
		t.Fatalf("X1 (cellRef %q, lib %q) did not resolve — the rename id≠display link is broken",
			sec.PartRef, sec.LibraryRef)
	}
	var vcc *ir.Pin
	for _, pin := range pt.Pins {
		if pin.Name == "Vcc" {
			vcc = pin
		}
	}
	if vcc == nil {
		t.Fatalf("resolved part-type has no Vcc pin (pins dropped): %+v", pt.Pins)
	}
	if vcc.Designator != "4" {
		t.Errorf("Vcc pin designator = %q, want 4", vcc.Designator)
	}
}
