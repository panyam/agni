package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ioMapDesign is one MCU whose part type names its pins, a PMIC, and a series resistor between them,
// so the fixture can answer a pin question, a net question and a far-end question at once.
//
//	U101.41 (PTC11) --- ADC_BATT_SENSE
//	U101.42 (PTE7)  --- PMIC_PG --[R3]-- PMIC_PG_MCU --- U7000.9 (PG)
//	U101.43 (GND)   --- GND     (and U101.44 is also named GND, which is what makes a name ambiguous)
func ioMapDesign() *ir.Design {
	lib := &ir.PartLibrary{Name: "lib", Parts: []*ir.PartType{
		{Name: "MCU", Pins: []*ir.Pin{
			{Designator: "41", Name: "PTC11"},
			{Designator: "42", Name: "PTE7"},
			{Designator: "43", Name: "GND"},
			{Designator: "44", Name: "GND"},
		}},
		{Name: "PMIC", Pins: []*ir.Pin{{Designator: "9", Name: "PG"}}},
	}}
	comp := func(ref, part string) *ir.Component {
		c := &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
		if part != "" {
			c.Sections = []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}
		}
		return c
	}
	net := func(name string, conns ...string) *ir.Net {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			ref, pin, _ := strings.Cut(c, ".")
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: pin})
		}
		return n
	}
	return &ir.Design{
		SourceFormat: "kicad-sch",
		Libraries:    []*ir.PartLibrary{lib},
		Components: []*ir.Component{
			comp("U101", "MCU"), comp("U7000", "PMIC"),
			{RefDes: "R3", Sections: []*ir.ComponentSection{{PartRef: "R"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			net("ADC_BATT_SENSE", "U101.41"),
			net("PMIC_PG", "U101.42", "R3.1"),
			net("PMIC_PG_MCU", "R3.2", "U7000.9"),
			net("GND", "U101.43", "U101.44"),
		},
	}
}

func ioMapDecl(t *testing.T, rows string) Declaration {
	t.Helper()
	return declOf(t, "name: I\nio_map:\n"+rows)
}

func onlyVerdict(t *testing.T, decl Declaration, rule string) check.Verdict {
	t.Helper()
	vs := verdictsFor(t, decl, check.NewModel(ioMapDesign()), rule)
	if len(vs) != 1 {
		t.Fatalf("%s produced %d verdicts, want 1", rule, len(vs))
	}
	return vs[0]
}

func TestIOMapPinPassesWhenTheNetIsWhereTheMapSays(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: '41'}"), RuleIOMapPin)
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s, reason %q", v.Outcome, v.Reason)
	}
	if v.Witness == nil || !strings.Contains(v.Witness.Statement, "ADC_BATT_SENSE") {
		t.Errorf("a pass with no witness naming the net is the silence this rule removes: %+v", v.Witness)
	}
}

// The whole point of the rule: a pin assignment that moved without the schematic being redrawn. The
// design is electrically fine, so nothing else would report it.
func TestIOMapPinFailsWhenTheNetMoved(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: '42'}"), RuleIOMapPin)
	if v.Outcome != check.Fail {
		t.Fatalf("outcome = %s, want fail", v.Outcome)
	}
	if v.Finding == nil || !strings.Contains(v.Finding.Message, "PMIC_PG") {
		t.Errorf("the finding should name the net the pin actually carries: %+v", v.Finding)
	}
}

// A map is authored in the datasheet's vocabulary and the netlist answers in package designators.
// Both spellings must resolve, or an author has to know which one the checker wants.
func TestIOMapPinResolvesTheFunctionalName(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11}"), RuleIOMapPin)
	if v.Outcome != check.Pass {
		t.Fatalf("a pin named the way a datasheet names it did not resolve: %s %q", v.Outcome, v.Reason)
	}
}

// The spelling differences that produced every false warning in the run we measured. Each must pass,
// and each must SAY what was assumed, because a match a reader cannot check is the thing being fixed.
func TestIOMapPinToleratesSpellingAndSaysSo(t *testing.T) {
	for _, spelling := range []string{"pte7", "PTE07", "PTE7\u200b"} {
		t.Run(spelling, func(t *testing.T) {
			v := onlyVerdict(t, ioMapDecl(t, "  - {net: PMIC_PG, device: U101, pin: \""+spelling+"\"}"), RuleIOMapPin)
			if v.Outcome != check.Pass {
				t.Fatalf("outcome = %s, reason %q", v.Outcome, v.Reason)
			}
			if v.Witness == nil || !strings.Contains(v.Witness.Statement, "pin-name match") {
				t.Errorf("a match that needed normalizing did not announce itself: %+v", v.Witness)
			}
		})
	}
	// An exact agreement must NOT announce anything, or the note stops meaning something.
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: PMIC_PG, device: U101, pin: PTE7}"), RuleIOMapPin)
	if v.Witness == nil || strings.Contains(v.Witness.Statement, "pin-name match") {
		t.Errorf("an exact match announced an inference: %+v", v.Witness)
	}
}

// A name matching several pins is inconclusive rather than a fail, and emphatically not a pass. A
// part type may name several pins the same (four GND pins is ordinary), so the row may well be
// correct and the comparison simply cannot decide.
func TestIOMapPinIsInconclusiveOnAnAmbiguousName(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: GND, device: U101, pin: GND}"), RuleIOMapPin)
	if v.Outcome != check.Inconclusive {
		t.Fatalf("outcome = %s, want inconclusive", v.Outcome)
	}
	if v.Finding == nil || !v.Finding.Inconclusive {
		t.Fatalf("the finding must carry Inconclusive, or a consumer counts it as a defect: %+v", v.Finding)
	}
	for _, want := range []string{"43", "44"} {
		if !strings.Contains(v.Finding.Message, want) {
			t.Errorf("the finding should name the candidate pins, missing %q: %s", want, v.Finding.Message)
		}
	}
}

func TestIOMapPinFailsOnAPinTheDeviceDoesNotHave(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: PTZ99}"), RuleIOMapPin)
	if v.Outcome != check.Fail {
		t.Fatalf("outcome = %s, want fail", v.Outcome)
	}
	if v.Finding == nil || v.Finding.Inconclusive {
		t.Error("a pin the part does not declare is decidable, so it is a fail rather than inconclusive")
	}
}

// The missing-net case is answered next door. Reporting it here as well would put one defect under
// two review items, and reading this silence as "the pin is fine" would be wrong.
func TestIOMapPinDeclinesTheMissingNetCase(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: NOPE, device: U101, pin: '41'}"), RuleIOMapPin)
	if v.Outcome != check.NotConsidered {
		t.Fatalf("outcome = %s, want not-considered", v.Outcome)
	}
	if !strings.Contains(v.Reason, RuleIOMapNetAbsent) {
		t.Errorf("the reason should name the rule that DOES report it: %q", v.Reason)
	}
}

func TestIOMapNetAbsentFailsOnAMissingNet(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: NEVER_DRAWN, device: U101, pin: '41'}"), RuleIOMapNetAbsent)
	if v.Outcome != check.Fail {
		t.Fatalf("outcome = %s, want fail", v.Outcome)
	}
}

// A misspelling and a disconnection are opposite defects, and the expensive mistake is sending a
// reviewer to look for a net that is present under a name differing by a character they cannot see.
func TestIOMapNetAbsentIsInconclusiveOnAMisspelling(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t, "  - {net: \"ADC_BATT_SENSE\u200b\", device: U101, pin: '41'}"), RuleIOMapNetAbsent)
	if v.Outcome != check.Inconclusive {
		t.Fatalf("outcome = %s, want inconclusive: a net differing only by spelling is not a disconnection", v.Outcome)
	}
	if v.Finding == nil || !v.Finding.Inconclusive || !strings.Contains(v.Finding.Message, "ADC_BATT_SENSE") {
		t.Errorf("the finding should name the net it found: %+v", v.Finding)
	}
}

// A map naming one net on several rows is ordinary, and repeating the absence per row would inflate
// the number a reader uses to judge how bad the disagreement is.
func TestIOMapNetAbsentReportsOneVerdictPerNet(t *testing.T) {
	decl := ioMapDecl(t, "  - {net: GND, device: U101, pin: '43'}\n  - {net: GND, device: U101, pin: '44'}")
	vs := verdictsFor(t, decl, check.NewModel(ioMapDesign()), RuleIOMapNetAbsent)
	if len(vs) != 1 {
		t.Errorf("two rows naming one net produced %d verdicts, want 1", len(vs))
	}
}

func TestIOMapFarEndPassesAndReportsTheRoute(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t,
		"  - {net: PMIC_PG, device: U101, pin: PTE7, to: {device: U7000, pin: PG}}"), RuleIOMapFarEnd)
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s, reason %q", v.Outcome, v.Reason)
	}
	// The route is the evidence: a verdict a reviewer cannot check is worth much less than one they
	// can, and the series resistor is exactly what they want to see.
	if v.Witness == nil || !strings.Contains(v.Witness.Statement, "[R3]") {
		t.Errorf("the witness should carry the route it walked: %+v", v.Witness)
	}
}

func TestIOMapFarEndFailsWhenItArrivesElsewhere(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t,
		"  - {net: ADC_BATT_SENSE, device: U101, pin: '41', to: {device: U7000, pin: PG}}"), RuleIOMapFarEnd)
	if v.Outcome != check.Fail {
		t.Fatalf("outcome = %s, want fail", v.Outcome)
	}
}

// Coverage. A map whose far-end columns are empty must read as UNEXAMINED rather than as clean, or a
// green result means only that the columns were blank. In the map we measured those columns were
// filled on roughly a third of rows.
func TestIOMapFarEndReportsRowsWithNoFarEnd(t *testing.T) {
	decl := ioMapDecl(t,
		"  - {net: ADC_BATT_SENSE, device: U101, pin: '41'}\n"+
			"  - {net: PMIC_PG, device: U101, pin: PTE7, to: {device: U7000, pin: PG}}")
	vs := verdictsFor(t, decl, check.NewModel(ioMapDesign()), RuleIOMapFarEnd)
	if len(vs) != 2 {
		t.Fatalf("got %d verdicts, want one per row so the denominator is visible", len(vs))
	}
	var declaredNone int
	for _, v := range vs {
		if v.Outcome == check.NotConsidered && strings.Contains(v.Reason, "no far end") {
			declaredNone++
		}
	}
	if declaredNone != 1 {
		t.Errorf("a row declaring no far end must say so, got %d such verdicts", declaredNone)
	}
}

// The field is carried so a map is authored once, and its presence must never read as verification.
func TestDeclaredFunctionIsReportedAsNotEvaluated(t *testing.T) {
	v := onlyVerdict(t, ioMapDecl(t,
		"  - {net: ADC_BATT_SENSE, device: U101, pin: '41', function: ADC0_S17}"), RuleIOMapPin)
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s", v.Outcome)
	}
	if v.Witness == nil || !strings.Contains(v.Witness.Statement, "NOT evaluated") {
		t.Errorf("a declared function passed silently, so filling the column in reads as verified: %+v", v.Witness)
	}
}

func TestIOMapLoadRejectsIncompleteRows(t *testing.T) {
	for _, c := range []struct{ name, rows, want string }{
		{"no net", "  - {device: U101, pin: '41'}", `"net"`},
		{"no device", "  - {net: N, pin: '41'}", `"device"`},
		{"no pin", "  - {net: N, device: U101}", `"pin"`},
		{"a far end with no pin", "  - {net: N, device: U101, pin: '41', to: {device: U7000}}", "no pin"},
		{"a far end with no device", "  - {net: N, device: U101, pin: '41', to: {pin: PG}}", "no device"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte("name: I\nio_map:\n" + c.rows))
			if err == nil {
				t.Fatal("want an error naming the missing field")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to mention %s", err, c.want)
			}
		})
	}
}

// A declaration carrying only an io_map is a complete declaration; it must not be rejected as empty.
func TestIOMapAloneIsAValidDeclaration(t *testing.T) {
	d := ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: '41'}")
	if len(d.IOMap) != 1 {
		t.Fatalf("IOMap = %+v", d.IOMap)
	}
	var names []string
	for _, r := range Compile(d) {
		names = append(names, r.Name)
	}
	for _, want := range []string{RuleIOMapPin, RuleIOMapNetAbsent, RuleIOMapFarEnd} {
		if !contains(names, want) {
			t.Errorf("Compile emitted %v, missing %s", names, want)
		}
	}
}

func contains(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}
