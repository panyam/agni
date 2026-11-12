package edif

import (
	"bytes"
	"path/filepath"
	"slices"
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

// readEDNIngested reads a fixture and then runs the Loader's shared MPN promotion. Since agni issue
// 519 the reader only captures the part number where its grammar puts it. A format-neutral pass
// decides which attribute the rest of the engine reads. So a test asserting the component-level MPN
// asserts an INGESTION outcome, not a reader one, and has to run both of ingestion's steps. The
// tests still read the fixtures. They should show that a real OrCAD export resolves, and a
// hand-built IR would not show that.
func readEDNIngested(t *testing.T, name string) *ir.Design {
	t.Helper()
	d := readEDN(t, name)
	classify.StampMPN(d)
	return d
}

// readSrc reads an EDIF document written inline. A structural test then states its exact shape in
// place, and the person reading the test has no fixture file to cross-reference.
func readSrc(t *testing.T, src string) *ir.Design {
	t.Helper()
	d, err := Read(strings.NewReader("("+src+")"), "inline.edn")
	if err != nil {
		t.Fatalf("Read inline: %v", err)
	}
	if len(d.Libraries) == 0 || len(d.Libraries[0].Parts) == 0 {
		t.Fatalf("no part types extracted from inline source")
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

// TestNormalizeMPN (WS3-075) covers an OrCAD export. Such an export puts the part number under a
// Manufacturer_PN property, not "MPN". The property is either renamed to display "Manufacturer PN"
// or bare. The datasheet join therefore read 0 MPNs.
// The reader records the property under the format's own key and classify.StampMPN promotes it to
// the typed ir.Component.mpn field, preferring an explicit "MPN" over the aliases. This test runs
// the pass (readEDNIngested) because the promotion is an ingestion outcome, not a reader one.
func TestNormalizeMPN(t *testing.T) {
	d := readEDNIngested(t, "mpn.edn")
	for ref, want := range map[string]string{
		"U1": "PARTX", // renamed property -> display key "Manufacturer PN"
		"U2": "PARTY", // bare property -> id key "Manufacturer_PN"
		"U3": "PARTZ", // explicit MPN wins over the alias's "IGNORED"
	} {
		if c := compByRef(d, ref); c == nil || c.GetMpn() != want {
			t.Errorf("%s MPN = %q, want %q", ref, c.GetMpn(), want)
		}
	}
	// A component with no part-number property gets no synthesized MPN (empty, not guessed).
	if r1 := compByRef(d, "R1"); r1 == nil || r1.GetMpn() != "" {
		t.Errorf("R1 MPN = %q, want empty (no alias property present)", r1.GetMpn())
	}
}

// TestReadNetlist is the baseline, covering a normal parse, section grouping and connectivity.
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

	// Provenance is recorded on the design and every extracted entity, with the source file on
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

// TestReadSchematicNetlist covers an EDIF SCHEMATIC (.eds) export. That export wraps designators
// and property values in (stringDisplay "V" ...). The netlist reader must unwrap that form the same
// way the geometry reader does (refDesOf/propText). Otherwise every .eds component reads with an
// empty ref_des, no MPN, and no property values (WS1-046). The .edn writes bare atoms, so this is
// additive there.
func TestReadSchematicNetlist(t *testing.T) {
	d := readEDNIngested(t, "schematic-netlist.eds")

	u7 := compByRef(d, "U7")
	if u7 == nil {
		t.Fatalf("component U7 not found (stringDisplay instance designator not unwrapped); got %d components", len(d.Components))
	}
	if got := u7.GetMpn(); got != "PART-123" {
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

	// Pin identity on a net comes from the instance's portInstance designators, the physical pin
	// numbers. The schematic view wraps those as well. Port "1" maps to physical pin "3".
	if !hasConn(netByName(d, "SIGA"), "U7", "3") {
		t.Errorf("SIGA should carry U7.3 (portInstance stringDisplay designator), got %v", netByName(d, "SIGA"))
	}
	if !hasConn(netByName(d, "SIGB"), "U7", "5") {
		t.Errorf("SIGB should carry U7.5, got %v", netByName(d, "SIGB"))
	}
}

// TestCellMPNFallback (WS1-046 Piece B) covers the cell-level part number. OrCAD puts a shared
// part-type's Manufacturer_PN on the CELL, not on every placed instance, and names the cell by the
// part number. A component with no inline MPN must inherit its cell's. An inline MPN still wins. A
// cell with no MPN leaves the component's MPN empty (no false fill). The join keys on the section's
// PartRef. It resolves the cell by either its display name or its &-stripped native id.
func TestCellMPNFallback(t *testing.T) {
	d := readEDNIngested(t, "cell-mpn.edn")
	if got := compByRef(d, "U1").GetMpn(); got != "CELL-MPN-1" {
		t.Errorf("U1 MPN = %q, want CELL-MPN-1 (inherited from its cell, keyed by &-stripped native id)", got)
	}
	if got := compByRef(d, "U2").GetMpn(); got != "INLINE-MPN-2" {
		t.Errorf("U2 MPN = %q, want INLINE-MPN-2 (inline MPN wins over the cell)", got)
	}
	if got := compByRef(d, "R1").GetMpn(); got != "" {
		t.Errorf("R1 MPN = %q, want empty (its cell carries no MPN, so no fallback)", got)
	}
}

// TestSchematicCellMPNFallback (WS1-046 Piece A + B combined) covers a real-world .eds case. The
// shared cell's Manufacturer_PN is stringDisplay-wrapped AND the instance has no inline MPN. So the
// value must be BOTH unwrapped (Piece A) and inherited via the cell join (Piece B). The cellRef
// here names the part by DISPLAY name, which exercises the name key of the dual-keyed index. An
// inline MPN on the .eds view still wins.
func TestSchematicCellMPNFallback(t *testing.T) {
	d := readEDNIngested(t, "schematic-cell-mpn.eds")
	u9 := compByRef(d, "U9")
	if u9 == nil {
		t.Fatalf("component U9 not found; got %d components", len(d.Components))
	}
	if got := u9.GetMpn(); got != "CELL-PART-9" {
		t.Errorf("U9 MPN = %q, want CELL-PART-9 (stringDisplay cell property, inherited via the cell join)", got)
	}
	if got := compByRef(d, "U10").GetMpn(); got != "INLINE-10" {
		t.Errorf("U10 MPN = %q, want INLINE-10 (inline MPN wins over the cell on the schematic view too)", got)
	}
}

// TestUnresolvedRefIsStable checks that a portRef to an instance with no ref_des (a power symbol)
// is NOT keyed by the export-unstable internal id; it becomes the "" no-ref marker.
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

// TestMemberPort checks that (portRef (member NAME IDX) ...) yields a stable bus-pin identity.
func TestMemberPort(t *testing.T) {
	d := readEDN(t, "edge.edn")
	if !hasConn(netByName(d, "DATABUS"), "U1", "DATA[3]") {
		t.Errorf("DATABUS should carry U1.DATA[3] (member port), got %v", netByName(d, "DATABUS"))
	}
}

// TestHierarchyDetected reads a design whose top cell instantiates a sub-cell with its own
// contents. It checks that the read records that sub-cell as unexpanded, with its instance count.
// It also checks that extraction is scoped to the top cell, with no sub-cell instances merged.
func TestHierarchyDetected(t *testing.T) {
	d := readEDN(t, "hier.edn")
	got := d.GetInputDiagnostics().GetUnexpandedHierarchy()
	if len(got) != 1 {
		t.Fatalf("unexpanded_hierarchy = %v, want exactly the sub-cell SUB", got)
	}
	if h := got[0]; h.GetName() != "SUB" || h.GetKind() != "edif_cell" || h.GetInstanceCount() != 1 {
		t.Errorf("unexpanded_hierarchy[0] = %v, want SUB, edif_cell, 1 instance", h)
	}
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "unexpanded_hierarchy") {
		t.Errorf("supplied = %v, want it to name unexpanded_hierarchy", d.GetInputDiagnostics().GetSupplied())
	}
	if compByRef(d, "X1") != nil {
		t.Error("sub-cell instance X1 must not be merged into the top netlist")
	}
	if compByRef(d, "U1") == nil {
		t.Error("top-cell instance U1 missing")
	}
}

// TestFlatDesignListsNoHierarchy is the control for TestHierarchyDetected. A flat design still says
// the reader looked, and lists nothing, so an empty list means flat rather than unexamined.
func TestFlatDesignListsNoHierarchy(t *testing.T) {
	d := readEDN(t, "basic.edn")
	if got := d.GetInputDiagnostics().GetUnexpandedHierarchy(); len(got) != 0 {
		t.Errorf("unexpanded_hierarchy = %v, want none on a flat design", got)
	}
	if !slices.Contains(d.GetInputDiagnostics().GetSupplied(), "unexpanded_hierarchy") {
		t.Errorf("supplied = %v, want it to name unexpanded_hierarchy", d.GetInputDiagnostics().GetSupplied())
	}
}

// TestParseNameForms is the grammar oracle, with one row per EDIF name form parseName accepts.
// Suppose the parser stops recognizing a form, or someone adds a new form without a row.
// Either one shows up in this table. The hand-rolled predecessor had no such guard
// (WS1-026). parseNode reuses the real tokenizer, so a row is written as source text, not a
// hand-built node tree.
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
		{"(member BUS 3)", "", "", ""},                                                     // member, not an entity name
		{"(unknown FORM)", "", "", ""},                                                     // unrecognized -> empty
	} {
		got := parseName(parseNode(t, tc.form))
		if got.ID != tc.wantID || got.Display != tc.wantDisplay || got.best() != tc.wantBest {
			t.Errorf("parseName(%s) = {ID:%q Display:%q best:%q}, want {ID:%q Display:%q best:%q}",
				tc.form, got.ID, got.Display, got.best(), tc.wantID, tc.wantDisplay, tc.wantBest)
		}
	}
}

// TestPortNameForms is the grammar oracle for the pin-identity projection. The & escape is
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

// TestNetNameForms checks that every EDIF net-name form resolves to a non-empty name. OrCAD's
// cap2edif emits the two (name ...) forms, and they are the dominant net form in the corpus.
// Before WS1-026 they dropped to an empty name and vanished from diff/check keying.
func TestNetNameForms(t *testing.T) {
	d := readEDN(t, "names.edn")
	for _, tc := range []struct{ ref, pin, want string }{
		{"R1", "1", "PLAIN"},   // bare atom
		{"R1", "2", "SIG/A"},   // (rename ID "Display")
		{"R1", "3", "ACT_LED"}, // (name ID (display ...)); id is the name, no string
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

// TestWrappedName checks that a hard-wrap newline inside a quoted name is joined, so identities
// hold no control characters (WS1-026). cap2edif splits the file at a fixed column mid-token.
func TestWrappedName(t *testing.T) {
	d := readEDN(t, "wrapped.edn")
	if d.Name != "SCHEMATIC1" {
		t.Errorf("design name = %q, want %q (wrap newline not joined)", d.Name, "SCHEMATIC1")
	}
	if strings.ContainsAny(d.Name, "\r\n") {
		t.Errorf("design name %q contains a control character", d.Name)
	}
}

// A cell whose EDIF id is numeric is escaped with a leading & in the source (&87844225). The
// instance references it by that escaped id. The part-type is NAMED with the id un-escaped
// (the rename's display string supplies the bare name). Suppose instanceOf resolves the cellRef differently from how the
// part-type is named and indexed. The component then never links to its part-type and has NO
// part-type pins. It is connected, since its net portRefs resolve, but every pin-level rule misses
// it. This test asserts that the instance -> part-type link resolves.
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

// TestRenameCellIdResolvesPins (WS1-045) covers a cell `(rename ID "Display")` whose display
// DIFFERS from its id and has no & escape. The cell is keyed by Display, but the instance
// references it by the ID. So the section resolves only through the native-id alias that
// PartIndex now adds. Before the fix the part's pins were silently dropped, and the real MC2016Z50
// oscillator surfaced 0 pins. The test asserts that the section resolves via the ACTUAL
// classify.PartIndex, not a hand-rolled one, and that its Vcc pin is present.
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

// TestEveryPinHasAJoinableDesignator (issue 71) holds that no part-type pin has an EMPTY
// designator. A connection joins on that designator.
//
// The Model indexes pins by `refDes + "\x00" + Designator`. An EDIF connection holds the portRef's
// name, and the portRef names the PORT. Leaving Designator empty collapsed every pin on such a part
// onto one key, and none of them resolved. PinRole then answered unknown for the whole design. Every
// pin-role rule was silently inert on EDIF, including diode orientation, gate/source/drain and LED
// polarity.
//
// The test sweeps every committed fixture rather than one. The gap shows only on parts whose ports
// omit a designator, and it went unseen while tests read a single fixture.
func TestEveryPinHasAJoinableDesignator(t *testing.T) {
	files, err := filepath.Glob("testdata/*.edn")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures to sweep: %v", err)
	}
	pins := 0
	for _, f := range files {
		d := readEDN(t, filepath.Base(f))
		for _, lib := range d.GetLibraries() {
			for _, p := range lib.GetParts() {
				for _, pin := range p.GetPins() {
					// A port with no NAME either has nothing to fall back to. That situation is
					// pre-existing and different. It appears on a top-level cell's own ports, not on
					// a component's pins. Both sides of the join are then the empty string, so it
					// resolves degenerately rather than failing. It is out of scope here, because
					// this change does not claim to fix it.
					if pin.GetName() == "" {
						continue
					}
					pins++
					if pin.GetDesignator() == "" {
						t.Errorf("%s: part %s pin %q has an empty designator, so no connection can join to it",
							filepath.Base(f), p.GetName(), pin.GetName())
					}
				}
			}
		}
	}
	if pins == 0 {
		t.Fatal("swept every fixture and found no part-type pins, so this proves nothing")
	}
	t.Logf("swept %d fixtures, %d pins", len(files), pins)
}

// TestExplicitPinDesignatorIsNotOverwritten pins the half that must NOT change. An explicit designator
// is the physical pin NUMBER, and a connection on such a part already uses that number. Setting
// Designator = Name unconditionally would break a join that already worked.
//
// It adds no COVERAGE. Setting the designator unconditionally already fails
// TestEscapedCellIdLinksToPartType and TestRenameCellIdResolvesPins, verified by mutation. Those two
// do not say WHY the value matters, and this test does. Their failure reads
// `VCC pin designator = "VCC", want 1` and looks like a cell-id resolution problem. Do not delete
// those two believing this one covers them. Do not delete this one believing it is duplication
// either, since it is the named statement of the constraint.
func TestExplicitPinDesignatorIsNotOverwritten(t *testing.T) {
	d := readEDN(t, "rename-cell-id.edn")
	for _, lib := range d.GetLibraries() {
		for _, p := range lib.GetParts() {
			for _, pin := range p.GetPins() {
				if pin.GetName() != "" && pin.GetDesignator() != "" && pin.GetName() != pin.GetDesignator() {
					return // an explicit designator survived distinct from its port name
				}
			}
		}
	}
	t.Error("rename-cell-id.edn declares pin numbers distinct from port names (Vcc/4, OUTPUT/3, GND/2); the fallback has overwritten them")
}

// TestCellDesignatorIgnoresPortNumbers pins agni issue 109. A cell that declares no prefix of its
// own must report none, not the pin number of its first port. Getting this wrong is silent at every
// step. The pin number goes into DesignatorPrefix, and Lexicon.Classify prefers it over the
// component's ref-des prefix. prefixClasses has no entry for "1", so every component of that cell
// reads unknown. Each class-quantified rule then selects zero members and reports nothing.
func TestCellDesignatorIgnoresPortNumbers(t *testing.T) {
	const noOwnPrefix = `edif T (edifVersion 2 0 0)
	  (library P
	    (cell CAP (cellType GENERIC) (view V (viewType NETLIST)
	      (interface (port 1 (direction INOUT) (designator "1"))
	                 (port 2 (direction INOUT) (designator "2"))))))`
	d := readSrc(t, noOwnPrefix)
	if pfx := d.Libraries[0].Parts[0].DesignatorPrefix; pfx != "" {
		t.Errorf("cell with only port designators: prefix = %q, want %q (a pin number is not a ref-des prefix)", pfx, "")
	}
}

// TestCellDesignatorFromInterface keeps the real-corpus form working. Mentor and OrCAD put the
// cell's prefix INSIDE the interface, as a sibling of the ports rather than above them. A fix that
// simply refused to enter the interface would drop the prefix on every such file.
func TestCellDesignatorFromInterface(t *testing.T) {
	const interfaceLevel = `edif T (edifVersion 2 0 0)
	  (library P
	    (cell TPD (cellType GENERIC) (view NORMAL (viewType SCHEMATIC)
	      (interface (designator "U")
	                 (port IO1 (direction INOUT) (designator "1"))
	                 (port IO2 (direction INOUT) (designator "2"))))))`
	d := readSrc(t, interfaceLevel)
	if pfx := d.Libraries[0].Parts[0].DesignatorPrefix; pfx != "U" {
		t.Errorf("interface-level designator: prefix = %q, want U", pfx)
	}
}

// TestCellDesignatorFromCellLevel accepts the cell-level form, which a hand-written netlist
// reasonably uses.
func TestCellDesignatorFromCellLevel(t *testing.T) {
	const cellLevel = `edif T (edifVersion 2 0 0)
	  (library P
	    (cell CAP (cellType GENERIC) (designator "C") (view V (viewType NETLIST)
	      (interface (port 1 (direction INOUT) (designator "1"))))))`
	d := readSrc(t, cellLevel)
	if pfx := d.Libraries[0].Parts[0].DesignatorPrefix; pfx != "C" {
		t.Errorf("cell-level designator: prefix = %q, want C", pfx)
	}
}

// TestReadUnannotatedComponents reads parts whose designator is still a placeholder. It checks that
// the read records them as an input diagnostic, grouped per PLACEHOLDER rather than per part. A
// consumer can then say "2 parts are still called R?" rather than repeating one sentence per part.
// An assigned designator contributes nothing, so the diagnostic remains a signal rather than a
// census.
func TestReadUnannotatedComponents(t *testing.T) {
	d := readEDN(t, "unannotated.edn")
	got := map[string]int{}
	for _, u := range d.GetInputDiagnostics().GetUnannotatedComponents() {
		got[u.GetRefDes()] = len(u.GetInstances())
	}
	want := map[string]int{"R?": 2, "C?": 1}
	if len(got) != len(want) {
		t.Fatalf("unannotated = %v, want %v (R7 is assigned and must not appear)", got, want)
	}
	for ref, n := range want {
		if got[ref] != n {
			t.Errorf("placeholder %q covers %d parts, want %d", ref, got[ref], n)
		}
	}
	// Every instance is recorded so a finding can point at all of them, not just the first.
	for _, u := range d.GetInputDiagnostics().GetUnannotatedComponents() {
		for i, p := range u.GetInstances() {
			if p.GetSourceFile() == "" {
				t.Errorf("%s instance %d has no provenance; a finding could not locate it", u.GetRefDes(), i)
			}
		}
	}
}

// TestReadDiagnosticsAccumulate guards an old bug. The reader used to assign a fresh
// InputDiagnostics for the bus signal alone. Any second signal added beside it would then silently
// drop the first. Both signals now share one struct.
func TestReadDiagnosticsAccumulate(t *testing.T) {
	d := readEDN(t, "unannotated.edn")
	if len(d.GetInputDiagnostics().GetUnannotatedComponents()) == 0 {
		t.Error("unannotated components were dropped from the diagnostics")
	}
	b := readEDN(t, "bus.edn")
	if len(b.GetInputDiagnostics().GetUnmodeledBuses()) == 0 {
		t.Error("unmodeled buses were dropped from the diagnostics")
	}
}
