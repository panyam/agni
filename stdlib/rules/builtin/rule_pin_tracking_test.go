package builtin

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/datasheet/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// trackSpec is xlatSpec plus one pin-to-pin relation, built by the caller so each test states the
// bound it is about. The relation shape mirrors the seeded TXB0104's VCCA <= VCCB.
func trackSpec(mpn string, rel *parampb.PinRelation) *parampb.PartSpec {
	spec := xlatSpec(mpn)
	spec.Relations = []*parampb.PinRelation{rel}
	return spec
}

func relProv() *parampb.ParamProvenance {
	return &parampb.ParamProvenance{DocRef: "ds", Page: 4, TableOrFigure: "Pin Functions", Method: "hand", Confidence: 1}
}

// tracking builds a required tracking relation on (VCCA - VCCB) with the given bounds; a nil bound
// is unbounded on that side.
func tracking(min, max *float64, modality parampb.Modality) *parampb.PinRelation {
	return &parampb.PinRelation{
		SubjectPinRef: "vcca", ReferencePinRef: "vccb",
		Kind:       parampb.PinRelationKind_PIN_RELATION_KIND_TRACKING,
		Difference: &parampb.RangeValue{Min: min, Max: max},
		Unit:       "V", Modality: modality, Raw: "VCCA <= VCCB", Prov: relProv(),
	}
}

func f64(v float64) *float64 { return &v }

// trackModel places the part with VCCA and VCCB on the named nets. Passing the SAME name for both
// puts them on one net, which is the connectivity tier.
func trackModel(mpn string, rel *parampb.PinRelation, netA, netB string) check.Model {
	d := xlatDesign(mpn, netA, netB)
	if netA == netB {
		d.Nets = []*ir.Net{{
			Name: netA,
			Connections: []*ir.Connection{
				{ComponentRef: "U1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "14"},
			},
			Prov: &ir.Provenance{SourceFile: "t"},
		}}
	}
	return check.NewModelWithParams(d, nil, param.ParamSet{mpn: trackSpec(mpn, rel)})
}

// The connectivity tier, satisfied. VCCA <= VCCB is a max of 0, and tying the two terminals makes
// the difference exactly 0, which is inside the bound. No net name is read to reach that verdict.
func TestPinTrackingSharedNetSatisfiesAMaxOfZero(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED), "VCC", "VCC")
	if fs := pinTrackingViolated.Findings(m); len(fs) != 0 {
		t.Errorf("tying two terminals satisfies a max-0 bound; want 0 findings, got %d: %+v", len(fs), fs)
	}
}

// The connectivity tier, violated, and the case naming cannot answer at all. A bound requiring the
// subject to sit at least 1V above the reference is broken with certainty by tying them, and the
// nets here carry no parseable voltage token, so the name tier would have been silent.
func TestPinTrackingSharedNetViolatesAMinimum(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(f64(1), nil, parampb.Modality_MODALITY_REQUIRED), "VCC_MAIN", "VCC_MAIN")
	fs := pinTrackingViolated.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("a shared net breaks a min-1V bound; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if fs[0].Inconclusive {
		t.Error("a shared net fixes the difference at 0 from connectivity; the verdict is certain")
	}
	for _, want := range []string{"VCC_MAIN", "VCCA", "VCCB", "1V"} {
		if !strings.Contains(fs[0].Message, want) {
			t.Errorf("message must mention %q, got %q", want, fs[0].Message)
		}
	}
	if len(fs[0].DatasheetProv) == 0 {
		t.Error("every param-backed finding carries a datasheet citation")
	}
}

// The name tier: two different rails, both named for their voltage, breaking a max-0 bound.
func TestPinTrackingNameTierComparesTwoRails(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED), "+3V3", "+1V8")
	fs := pinTrackingViolated.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("3.3 - 1.8 = 1.5V exceeds a max of 0; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "1.5") {
		t.Errorf("message must state the difference it computed, got %q", fs[0].Message)
	}
}

// The same two rails the other way round satisfy the bound. This is the direction test: the bound is
// on subject MINUS reference, so swapping the rails must flip the verdict rather than keep it.
func TestPinTrackingRespectsSubtractionOrder(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED), "+1V8", "+3V3")
	if fs := pinTrackingViolated.Findings(m); len(fs) != 0 {
		t.Errorf("1.8 - 3.3 = -1.5V is within a max of 0; want 0 findings, got %d: %+v", len(fs), fs)
	}
}

// Modality picks the rule, and each ignores the other's rows. A finding reported at one severity for
// both a "shall never exceed" and a "should, for best operation" misstates one of them.
func TestPinTrackingModalitySelectsTheRule(t *testing.T) {
	required := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED), "+3V3", "+1V8")
	if n := len(pinTrackingViolated.Findings(required)); n != 1 {
		t.Errorf("a required bound belongs to the error rule; want 1, got %d", n)
	}
	if n := len(pinTrackingAdvisory.Findings(required)); n != 0 {
		t.Errorf("the advisory rule must not report a required bound; want 0, got %d", n)
	}

	recommended := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_RECOMMENDED), "+3V3", "+1V8")
	if n := len(pinTrackingAdvisory.Findings(recommended)); n != 1 {
		t.Errorf("a recommended bound belongs to the warning rule; want 1, got %d", n)
	}
	if n := len(pinTrackingViolated.Findings(recommended)); n != 0 {
		t.Errorf("the error rule must not report a recommended bound; want 0, got %d", n)
	}
}

// A bound the datasheet scopes to a regime cannot be evaluated against DC rail nominals, so a breach
// is reported inconclusive rather than as a violation. The S32K3xx's "100mV, transient only, not for
// DC" is the real instance, and it is also the millivolt case.
func TestPinTrackingRegimeScopedBoundIsInconclusive(t *testing.T) {
	rel := tracking(nil, f64(100), parampb.Modality_MODALITY_REQUIRED)
	rel.Unit = "mV"
	rel.Conditions = []*parampb.Condition{{Raw: "transient only (not for DC)"}}
	m := trackModel("ACME-XLAT", rel, "+3V3", "+1V8")

	fs := pinTrackingViolated.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("1.5V breaches a 100mV bound; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !fs[0].Inconclusive {
		t.Error("a regime the rule cannot evaluate must report inconclusive, not a violation")
	}
	if !strings.Contains(fs[0].Message, "transient only") {
		t.Errorf("the finding must name the regime it could not evaluate, got %q", fs[0].Message)
	}
	// 100mV must have been reduced to 0.1V; printing the raw 100 would read as a 100V allowance.
	if !strings.Contains(fs[0].Message, "0.1V") {
		t.Errorf("a millivolt bound must be compared and printed in volts, got %q", fs[0].Message)
	}
}

// The same regime-scoped relation, not breached, stays silent. Reporting every scoped relation would
// convert a coverage gap into noise (the Finding.Inconclusive contract).
func TestPinTrackingRegimeScopedBoundIsSilentWhenWithin(t *testing.T) {
	rel := tracking(nil, f64(100), parampb.Modality_MODALITY_REQUIRED)
	rel.Unit = "mV"
	rel.Conditions = []*parampb.Condition{{Raw: "transient only (not for DC)"}}
	m := trackModel("ACME-XLAT", rel, "+3V3", "+3V3")

	if fs := pinTrackingViolated.Findings(m); len(fs) != 0 {
		t.Errorf("a scoped bound that is not breached says nothing; want 0 findings, got %+v", fs)
	}
}

// param.Validate requires a relation's kind, bound and provenance but NOT its modality, so a spec can
// carry a breach whose severity is unknown. It must not vanish: the required rule takes it and says
// what is missing rather than inventing an error or passing in silence.
func TestPinTrackingUnstatedModalityIsReportedInconclusive(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_UNSPECIFIED), "+3V3", "+1V8")

	fs := pinTrackingViolated.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("an unstated modality must still be reported; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !fs[0].Inconclusive {
		t.Error("severity is unknown without a modality, so the finding is inconclusive")
	}
	if n := len(pinTrackingAdvisory.Findings(m)); n != 0 {
		t.Errorf("only one rule may claim an unstated modality; want 0 from advisory, got %d", n)
	}
}

// The issue-194 guard. A voltage token anywhere in a name parses as a nominal, so a SIGNAL net whose
// name encodes a level would be compared as though it were a supply rail. The name tier reads a name
// only after the net classifies as a rail; the connectivity tier is unaffected because it reads no
// name at all.
//
// THE SPEC'S PIN FUNCTIONS ARE STRIPPED ON PURPOSE, to isolate the name tier. With them present the
// datasheet establishes the rail role directly (agni issue 280 phase 2) and the name is never
// consulted, which is correct but tests the other tier. The original fixture here wired a
// signal-NAMED net to a pin the spec types as a supply input, so it only read as a signal while the
// name was the sole evidence; the case where a datasheet overrules the name is asserted below.
func TestPinTrackingNameTierRequiresBothNetsToBeRails(t *testing.T) {
	spec := trackSpec("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED))
	for _, p := range spec.Pins {
		p.Function = parampb.PinFunction_PIN_FUNCTION_UNSPECIFIED
	}
	m := check.NewModelWithParams(xlatDesign("ACME-XLAT", "U3_12_U7_4_3V3", "+1V8"), nil,
		param.ParamSet{"ACME-XLAT": spec})

	if fs := pinTrackingViolated.Findings(m); len(fs) != 0 {
		t.Errorf("a signal net carrying a voltage token is not a rail nominal; want 0 findings, got %+v", fs)
	}
}

// The other side of that, and the point of the datasheet evidence tier: a net whose NAME reads as a
// signal is a rail anyway when it feeds a terminal the vendor types as a power input. The name is a
// claim about spelling; the pin function is evidence about the circuit, and it wins.
func TestPinTrackingDatasheetEvidenceBeatsASignalLookingName(t *testing.T) {
	m := trackModel("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED),
		"U3_12_U7_4_3V3", "+1V8")

	fs := pinTrackingViolated.Findings(m)
	if len(fs) != 1 {
		t.Fatalf("the spec types both terminals as supply inputs, so both nets are rails; want 1 finding, got %d: %+v", len(fs), fs)
	}
	if !strings.Contains(fs[0].Message, "1.5") {
		t.Errorf("the comparison should proceed on both rails, got %q", fs[0].Message)
	}
}

// Skip-not-false-pass across every missing input, and the degrade-safe case that matters most: a
// spec with no relations behaves exactly as it did before these rules existed.
func TestPinTrackingSilentWithoutItsInputs(t *testing.T) {
	rel := tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED)
	cases := []struct {
		name string
		m    check.Model
	}{
		{"no params tier at all", check.NewModel(xlatDesign("ACME-XLAT", "+3V3", "+1V8"))},
		{"no seeded spec for the mpn", check.NewModelWithParams(
			xlatDesign("ACME-XLAT", "+3V3", "+1V8"), nil, param.ParamSet{})},
		{"a spec with no relations", check.NewModelWithParams(
			xlatDesign("ACME-XLAT", "+3V3", "+1V8"), nil,
			param.ParamSet{"ACME-XLAT": xlatSpec("ACME-XLAT")})},
		{"no voltage evidence on either rail", trackModel("ACME-XLAT", rel, "VDD_MAIN", "VDD_AUX")},
		{"a bound stated in a non-voltage unit", func() check.Model {
			r := tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED)
			r.Unit = "A"
			return trackModel("ACME-XLAT", r, "+3V3", "+1V8")
		}()},
	}
	for _, tc := range cases {
		if n := len(pinTrackingViolated.Findings(tc.m)); n != 0 {
			t.Errorf("%s: want 0 findings from the required rule, got %d", tc.name, n)
		}
		if n := len(pinTrackingAdvisory.Findings(tc.m)); n != 0 {
			t.Errorf("%s: want 0 findings from the advisory rule, got %d", tc.name, n)
		}
	}
}

// A relation whose terminal the spec cannot resolve unambiguously must produce nothing. Two spec pins
// print the name the design uses and no package identifies them, so ResolvePin refuses; a rule that
// fell back to the first match would report a confident finding about a guessed terminal.
func TestPinTrackingSkipsAnUnresolvableTerminal(t *testing.T) {
	spec := trackSpec("ACME-XLAT", tracking(nil, f64(0), parampb.Modality_MODALITY_REQUIRED))
	spec.Pins[1].Name = "VCCA" // both terminals now print VCCA
	spec.Packages = nil        // and no package is declared, so the number cannot break the tie
	for _, p := range spec.Pins {
		p.Numbers = nil
	}
	m := check.NewModelWithParams(xlatDesign("ACME-XLAT", "+3V3", "+1V8"), nil,
		param.ParamSet{"ACME-XLAT": spec})

	if n := len(pinTrackingViolated.Findings(m)); n != 0 {
		t.Errorf("an ambiguous terminal must be skipped, not guessed; got %d findings", n)
	}
}
