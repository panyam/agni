package telesis

import (
	"os"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func readFixture(t *testing.T, name string) *ir.Design {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := Read(f, "testdata/"+name)
	if err != nil {
		t.Fatalf("Read(%s): %v", name, err)
	}
	return d
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

func partByName(d *ir.Design, name string) *ir.PartType {
	for _, l := range d.Libraries {
		for _, p := range l.Parts {
			if p.Name == name {
				return p
			}
		}
	}
	return nil
}

func pinByDesignator(p *ir.PartType, des string) *ir.Pin {
	for _, pin := range p.Pins {
		if pin.Designator == des {
			return pin
		}
	}
	return nil
}

// TestBangDiscriminatesPackageFromProperty is the assertion that catches the format's nastiest
// trap. The fixture's $PACKAGES section contains a line with the property grammar and no `!`. A
// scanner keying on quotes alone reads it as a package and invents a component called NotAPackage,
// which is a design that parses cleanly and is wrong.
func TestBangDiscriminatesPackageFromProperty(t *testing.T) {
	d := readFixture(t, "basic.tel")

	want := []string{"R1", "R2", "R3", "R4", "R5", "R6", "R7", "C1", "U1", "U7", "J1", "D1", "D2"}
	if len(d.Components) != len(want) {
		var got []string
		for _, c := range d.Components {
			got = append(got, c.RefDes)
		}
		t.Fatalf("components = %v (%d), want %v (%d)", got, len(d.Components), want, len(want))
	}
	for _, ref := range want {
		if compByRef(d, ref) == nil {
			t.Errorf("component %s missing", ref)
		}
	}
	if partByName(d, "NotAPackage") != nil {
		t.Error("a property line inside $PACKAGES was read as a part type")
	}
}

// TestContinuationAcrossLines covers the format's only continuation marker, on both a package's
// ref-des run and a net's endpoint run, each spanning three lines.
func TestContinuationAcrossLines(t *testing.T) {
	d := readFixture(t, "basic.tel")

	for _, ref := range []string{"R4", "R5"} {
		c := compByRef(d, ref)
		if c == nil {
			t.Fatalf("%s missing; the package ref-des run did not continue past its first line", ref)
		}
		if got := c.Sections[0].PartRef; got != "RES-10K" {
			t.Errorf("%s part = %q, want RES-10K", ref, got)
		}
	}

	sda := netByName(d, "I2C_SDA")
	if sda == nil {
		t.Fatal("I2C_SDA missing")
	}
	if len(sda.Connections) != 8 {
		var got []string
		for _, c := range sda.Connections {
			got = append(got, c.ComponentRef+"."+c.PinRef)
		}
		t.Errorf("I2C_SDA connections = %v (%d), want 8 across three lines", got, len(sda.Connections))
	}
}

// TestPinDesignatorShapes covers the three legal designator shapes. The BGA and pure-letter cases
// matter because narrowing the pin pattern to digits routes them into component scope, where they
// become attributes on a component that does not exist.
func TestPinDesignatorShapes(t *testing.T) {
	d := readFixture(t, "basic.tel")
	sda := netByName(d, "I2C_SDA")

	want := map[string]string{"U1": "5", "U7": "L1", "J1": "A"}
	got := map[string]string{}
	for _, c := range sda.Connections {
		if _, ok := want[c.ComponentRef]; ok {
			got[c.ComponentRef] = c.PinRef
		}
	}
	for ref, pin := range want {
		if got[ref] != pin {
			t.Errorf("%s pin = %q, want %q", ref, got[ref], pin)
		}
	}
}

// TestPropertiesTransposeOntoComponents checks the inverted index is inverted: the file groups by
// value and the IR hangs attributes off each component.
func TestPropertiesTransposeOntoComponents(t *testing.T) {
	d := readFixture(t, "basic.tel")

	for _, ref := range []string{"R1", "R2", "R3", "R4", "R5"} {
		if got := compByRef(d, ref).GetAttributes()["Resistance"]; got != "10k" {
			t.Errorf("%s Resistance = %q, want 10k", ref, got)
		}
	}
	if got := compByRef(d, "C1").GetAttributes()["Capacitance"]; got != "100nF" {
		t.Errorf("C1 Capacitance = %q, want 100nF", got)
	}
	if got := compByRef(d, "U1").GetAttributes()["Description"]; got != "Dual supply translator" {
		t.Errorf("U1 Description = %q", got)
	}
	// A component-scoped property must not leak onto a pin, and vice versa.
	if _, bad := compByRef(d, "U1").GetAttributes()["Pin Type"]; bad {
		t.Error("a pin-scoped property landed on the component")
	}
}

// TestPinTypeBecomesDirection is the reason the property sections are not optional: this block is
// the only place the format states direction, so skipping it leaves every pin UNSPECIFIED and
// silently disables every direction-dependent rule.
func TestPinTypeBecomesDirection(t *testing.T) {
	d := readFixture(t, "basic.tel")

	ic := partByName(d, "IC-DUAL")
	if ic == nil {
		t.Fatal("IC-DUAL part type missing")
	}
	for _, tc := range []struct {
		pin  string
		want ir.PinDirection
	}{
		{"1", ir.PinDirection_PIN_DIRECTION_INPUT},
		{"5", ir.PinDirection_PIN_DIRECTION_INPUT},
		{"14", ir.PinDirection_PIN_DIRECTION_OUTPUT},
	} {
		p := pinByDesignator(ic, tc.pin)
		if p == nil {
			t.Errorf("IC-DUAL pin %s missing", tc.pin)
			continue
		}
		if p.Direction != tc.want {
			t.Errorf("IC-DUAL pin %s direction = %v, want %v", tc.pin, p.Direction, tc.want)
		}
	}

	conn := partByName(d, "CONN-2")
	if p := pinByDesignator(conn, "2"); p == nil || p.Direction != ir.PinDirection_PIN_DIRECTION_POWER_IN {
		t.Errorf("CONN-2 pin 2 = %v, want POWER_IN (GROUND consumes, like POWER)", p.GetDirection())
	}
}

// TestPinTypeCaseIsFolded covers a spelling inconsistency present in a real export: the same value
// appears as both TERMINAL and Terminal. A case-sensitive map drops half those pins to UNSPECIFIED,
// and worse, treating the pair as a library inconsistency would report noise.
func TestPinTypeCaseIsFolded(t *testing.T) {
	d := readFixture(t, "basic.tel")
	res := partByName(d, "RES-10K")
	if res == nil {
		t.Fatal("RES-10K missing")
	}
	p := pinByDesignator(res, "1")
	if p == nil {
		t.Fatal("RES-10K pin 1 missing")
	}
	if p.Direction != ir.PinDirection_PIN_DIRECTION_PASSIVE {
		t.Errorf("pin 1 direction = %v, want PASSIVE", p.Direction)
	}
	if got, bad := p.GetAttributes()["direction_conflict"]; bad {
		t.Errorf("TERMINAL vs Terminal recorded as a conflict (%q); case alone is not a disagreement", got)
	}
}

// TestPinTypeConflictIsRecorded is the other half: a genuine disagreement between two instances of
// one part type is kept rather than silently resolved, since a library inconsistency that leaves no
// trace is unfindable. Pin 2 carries it, so this stays independent of the case-folding case on
// pin 1: one fixture pin asserting both would let either behaviour mask the other.
func TestPinTypeConflictIsRecorded(t *testing.T) {
	d := readFixture(t, "basic.tel")
	res := partByName(d, "RES-10K")
	p := pinByDesignator(res, "2")
	if p.GetAttributes()["direction_conflict"] == "" {
		t.Fatal("R4.2 declares BI where R1.2 declares TERMINAL; the disagreement was dropped")
	}
	if !strings.Contains(p.GetAttributes()["direction_conflict"], "BI") {
		t.Errorf("direction_conflict = %q, want it to name the losing value", p.GetAttributes()["direction_conflict"])
	}
}

// TestUnknownPinTypeDegrades checks a second exporter's vocabulary is not lost. An unrecognised
// value yields UNSPECIFIED and keeps the raw spelling, rather than being dropped.
func TestUnknownPinTypeDegrades(t *testing.T) {
	d := readFixture(t, "basic.tel")
	bga := partByName(d, "BGA-PART")
	p := pinByDesignator(bga, "L1")
	if p == nil {
		t.Fatal("BGA-PART pin L1 missing")
	}
	if p.Direction != ir.PinDirection_PIN_DIRECTION_UNSPECIFIED {
		t.Errorf("direction = %v, want UNSPECIFIED for an unrecognised Pin Type", p.Direction)
	}
	if got := p.GetAttributes()["direction_raw"]; got != "WEIRD_FUTURE_VALUE" {
		t.Errorf("direction_raw = %q, want the source spelling preserved", got)
	}
}

// TestGeneratedNetsAreKeptAndTagged pins the decision that a `$`-named net stays in the design with
// a marker, rather than being dropped at the reader. Dropping is quieter and throws away every real
// dangling connection along with the noise, with no way for a consumer to know.
func TestGeneratedNetsAreKeptAndTagged(t *testing.T) {
	d := readFixture(t, "basic.tel")

	gen := netByName(d, "$1N0001")
	if gen == nil {
		t.Fatal("$-prefixed net was dropped; it should be kept and tagged")
	}
	if got := gen.GetAttributes()[GeneratedNameAttr]; got != "true" {
		t.Errorf("%s attribute = %q, want true", GeneratedNameAttr, got)
	}
	if _, tagged := netByName(d, "VCC_3V3").GetAttributes()[GeneratedNameAttr]; tagged {
		t.Error("a human-named net was tagged as generated")
	}
}

// TestUnknownSectionIsSkipped checks the reader tolerates a section it has never seen. The grammar
// here is known from real exports rather than a specification, so a second writer emitting an extra
// section must degrade rather than fail or, worse, have its contents read as something else.
func TestUnknownSectionIsSkipped(t *testing.T) {
	d := readFixture(t, "basic.tel")
	if got := compByRef(d, "R1").GetAttributes()["Whatever"]; got != "" {
		t.Errorf("content of $UNKNOWN_SECTION was read as a property (%q)", got)
	}
}

// TestEmptyPinsSection checks the empty $PINS in the fixture, which is what every real export
// carries, does not break the section walk that follows it.
func TestEmptyPinsSection(t *testing.T) {
	d := readFixture(t, "basic.tel")
	if partByName(d, "IC-DUAL").GetPins() == nil {
		t.Error("the $A_PROPERTIES block AFTER the empty $PINS was not read")
	}
}

// TestLatin1Fallback covers an encoding real files use. Go substitutes U+FFFD for each invalid
// byte, so a UTF-8-only read corrupts whatever value the byte sat in instead of failing.
func TestLatin1Fallback(t *testing.T) {
	d := readFixture(t, "latin1.tel")
	got := compByRef(d, "C1").GetAttributes()["Capacitance"]
	if got != "100µF" {
		t.Errorf("Capacitance = %q, want 100µF decoded from latin-1", got)
	}
	if strings.ContainsRune(got, '�') {
		t.Error("value contains the replacement rune, so the bytes were read as UTF-8")
	}
}

// TestDiagnosticsAreDeclared checks the reader states what it looked for. An empty list means both
// "looked and found nothing" and "never looked", and a rule reading the second as the first reports
// a clean pass over a question nobody asked (agni issue 309).
func TestDiagnosticsAreDeclared(t *testing.T) {
	d := readFixture(t, "basic.tel")
	supplied := d.GetInputDiagnostics().GetSupplied()
	if len(supplied) == 0 {
		t.Fatal("reader declares no supplied diagnostics")
	}
	found := false
	for _, s := range supplied {
		if s == "ref_des_collisions" {
			found = true
		}
	}
	if !found {
		t.Errorf("supplied = %v, want ref_des_collisions, which this format can detect", supplied)
	}
}

func TestSourceFormatAndProvenance(t *testing.T) {
	d := readFixture(t, "basic.tel")
	if d.SourceFormat != SourceFormat {
		t.Errorf("source format = %q, want %q", d.SourceFormat, SourceFormat)
	}
	if got := netByName(d, "GND").GetProv().GetSourceFile(); !strings.HasSuffix(got, "basic.tel") {
		t.Errorf("net provenance = %q, want the source file", got)
	}
}

// TestNotATelesisFile checks a file with no recognisable section produces an error rather than an
// empty design, which would read as a design that genuinely has nothing in it.
func TestNotATelesisFile(t *testing.T) {
	if _, err := Read(strings.NewReader("(edif SOMETHING)\n"), "x.tel"); err == nil {
		t.Fatal("a non-Telesis file parsed into a design instead of erroring")
	}
}

// TestPackageWithoutAPartName covers a head shape found only by running against real exports: an
// entry written `! 'MPN' ; targets`, naming a part solely by its manufacturer number.
//
// This is not a cosmetic omission. A head shape the scanner does not recognise also breaks the
// continuation chain, so the entry's whole ref-des run is skipped along with it. In one real export
// that silently dropped 184 capacitors from the component set while every net still referenced
// them, which reads as a design with dangling connections rather than as a parse failure.
func TestPackageWithoutAPartName(t *testing.T) {
	d := readFixture(t, "basic.tel")

	for _, ref := range []string{"D1", "D2"} {
		c := compByRef(d, ref)
		if c == nil {
			t.Fatalf("%s missing; an entry with no part name was skipped", ref)
		}
		if got := c.Sections[0].PartRef; got != "MPN-ONLY-1N4148" {
			t.Errorf("%s part = %q, want the MPN standing in for the absent name", ref, got)
		}
	}
	pt := partByName(d, "MPN-ONLY-1N4148")
	if pt == nil {
		t.Fatal("part type keyed by MPN missing")
	}
	if pt.GetAttributes()["name_from_mpn"] != "true" {
		t.Error("the MPN-for-name substitution is not recorded, so the IR claims the library named it")
	}
}

// TestPackageWithExtraFields covers the other head shape real exports carry: more than two
// bang-separated fields. Their meaning is positional and stated nowhere, so they are kept by
// position rather than guessed at.
func TestPackageWithExtraFields(t *testing.T) {
	d := readFixture(t, "basic.tel")

	for _, ref := range []string{"R6", "R7"} {
		if compByRef(d, ref) == nil {
			t.Fatalf("%s missing; an entry with extra head fields was skipped", ref)
		}
	}
	pt := partByName(d, "RES-4K7")
	if pt == nil {
		t.Fatal("RES-4K7 missing")
	}
	if got := pt.GetAttributes()["mpn"]; got != "RC0603FR-074K7L" {
		t.Errorf("mpn = %q; the second field is still the MPN when more follow it", got)
	}
	if got := pt.GetAttributes()["field_3"]; got != "4.7k" {
		t.Errorf("field_3 = %q, want 4.7k", got)
	}
	if got := pt.GetAttributes()["field_4"]; got != "1%" {
		t.Errorf("field_4 = %q, want 1%%", got)
	}
}

// TestQuotedFieldContainingSeparators checks the scanner respects quoting: a field whose text
// contains a bang or a semicolon must not end the head early.
func TestQuotedFieldContainingSeparators(t *testing.T) {
	const src = "$PACKAGES\n\n'ODD;NAME!X' ! 'MPN-1' ;  U9\n\n$NETS\n\n'N1' ; U9.1 U9.2\n\n$END\n"
	d, err := Read(strings.NewReader(src), "odd.tel")
	if err != nil {
		t.Fatal(err)
	}
	if compByRef(d, "U9") == nil {
		t.Fatal("U9 missing; a quoted separator ended the head early")
	}
	if partByName(d, "ODD;NAME!X") == nil {
		t.Error("part name with embedded separators was truncated")
	}
}
