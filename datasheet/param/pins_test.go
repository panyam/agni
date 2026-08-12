package param

import (
	"errors"
	"slices"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// The multi-supply case the pin-binding contract exists for: VCCA and VCCB are two
// terminals with genuinely different ranges, and each recommended-operating row binds to
// its own pin. Without the binding both rows answer for one "supply" concept.
func TestTXB0104MultiSupplyPinsStayDistinct(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	recOp := map[string]*parampb.Parameter{}
	for _, p := range spec.Parameters {
		if p.LimitKind != parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING {
			continue
		}
		if len(p.PinRefs) == 1 {
			recOp[p.PinRefs[0]] = p
		}
	}

	a, b := recOp["vcca"], recOp["vccb"]
	if a == nil || b == nil {
		t.Fatalf("want recommended-operating rows bound to vcca and vccb, got %v", recOp)
	}
	if a.Value.GetMin() != 1.2 || a.Value.GetMax() != 3.6 {
		t.Errorf("VCCA recommended: want 1.2..3.6, got %+v", a.Value)
	}
	if b.Value.GetMin() != 1.65 || b.Value.GetMax() != 5.5 {
		t.Errorf("VCCB recommended: want 1.65..5.5, got %+v", b.Value)
	}
	if !MachineComparable(a) || !MachineComparable(b) {
		t.Errorf("the recommended rows carry a structured TA range; both must be machine-comparable")
	}
}

// A row may bind to a GROUP of terminals: the continuous-current limit is stated once for the two
// supplies and ground together, so each of the three has to find it.
func TestTXB0104GroupBinding(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	var group *parampb.Parameter
	for _, p := range spec.Parameters {
		if len(p.PinRefs) > 1 {
			group = p
		}
	}
	if group == nil {
		t.Fatal("want a row bound to several terminals")
	}
	if got := group.PinRefs; len(got) != 3 {
		t.Errorf("the continuous-current row covers VCCA, VCCB and GND; got %v", got)
	}
	for _, id := range group.PinRefs {
		if PinByID(spec, id) == nil {
			t.Errorf("group row binds to undeclared pin %q", id)
		}
		if !slices.ContainsFunc(PinParameters(spec, id), func(p *parampb.Parameter) bool { return p == group }) {
			t.Errorf("pin %q cannot find the row that binds it", id)
		}
	}
}

// Part-wide rows keep an EMPTY binding, which is what every spec seeded before pin binding
// existed says. The junction-temperature rating is a fact about the die, not a terminal.
func TestPartWideRowsCarryNoBinding(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")
	for _, p := range spec.Parameters {
		if p.Symbol == "TJ" || p.Symbol == "Tstg" || p.Symbol == "TA" {
			if len(p.PinRefs) != 0 {
				t.Errorf("%s is a part-wide rating; want no pin_refs, got %v", p.Symbol, p.PinRefs)
			}
		}
	}
}

// The repackaging case, from the real pinout: number 11 is the B3 data I/O in the TSSOP-14
// and the VCCB supply in the UQFN-12. Both readings are recorded, so a number-keyed join
// has to say which package it means.
func TestSameNumberIsDifferentPinsInDifferentPackages(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	inPW := PinsByNumber(spec, "pw", "11")
	if len(inPW) != 1 || inPW[0].Id != "b3" {
		t.Fatalf("pw number 11: want the b3 data I/O, got %v", pinIDs(inPW))
	}
	inRUT := PinsByNumber(spec, "rut", "11")
	if len(inRUT) != 1 || inRUT[0].Id != "vccb" {
		t.Fatalf("rut number 11: want the vccb supply, got %v", pinIDs(inRUT))
	}
	if inPW[0].Function == inRUT[0].Function {
		t.Errorf("the collision is only interesting because the two pins differ in function; both are %v", inPW[0].Function)
	}
}

// The name channel survives repackaging: VCCB is "VCCB" in every body, whatever number it
// carries there.
func TestNameSurvivesRepackaging(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")
	hits := PinsByName(spec, "VCCB")
	if len(hits) != 1 || hits[0].Id != "vccb" {
		t.Fatalf("name VCCB: want exactly the vccb pin, got %v", pinIDs(hits))
	}
	want := map[string]string{"pw": "14", "rut": "11", "yzt": "A2"}
	got := map[string]string{}
	for _, n := range hits[0].Numbers {
		got[n.PackageRef] = n.Number
	}
	for pkg, num := range want {
		if got[pkg] != num {
			t.Errorf("vccb in package %s: want number %q, got %q", pkg, num, got[pkg])
		}
	}
}

func TestResolvePin(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	cases := []struct {
		name       string
		pinName    string
		designator string
		packageRef string
		wantID     string
		wantErr    error
	}{
		{
			name:    "unique name resolves without consulting the number",
			pinName: "VCCA", designator: "", packageRef: "",
			wantID: "vcca",
		},
		{
			name:    "name wins over a number that would point elsewhere in another package",
			pinName: "VCCB", designator: "11", packageRef: "rut",
			wantID: "vccb",
		},
		{
			name:    "name is matched after normalization (producers split subscripts)",
			pinName: "v cca", designator: "", packageRef: "",
			wantID: "vcca",
		},
		{
			name:    "an ambiguous name is tie-broken by the number inside an identified package",
			pinName: "NC", designator: "9", packageRef: "pw",
			wantID: "nc9",
		},
		{
			// 6 is NC in the TSSOP-14 and GND in the UQFN-12, so the number cannot break the tie
			// either until a package is named.
			name:    "an ambiguous name with no package identified refuses",
			pinName: "NC", designator: "6", packageRef: "",
			wantErr: ErrPinAmbiguous,
		},
		{
			name:    "an ambiguous name with no number at all refuses",
			pinName: "NC", designator: "", packageRef: "pw",
			wantErr: ErrPinAmbiguous,
		},
		{
			name:    "name and number disagreeing refuses rather than picking a winner",
			pinName: "VCCB", designator: "11", packageRef: "pw",
			wantErr: ErrPinConflict,
		},
		{
			name:    "a number that means the same pin in every package resolves without one",
			pinName: "", designator: "1", packageRef: "",
			wantID: "vcca",
		},
		{
			name:    "a number that means different pins across packages refuses without one",
			pinName: "", designator: "11", packageRef: "",
			wantErr: ErrPinAmbiguous,
		},
		{
			name:    "a ball designator resolves like any other number",
			pinName: "", designator: "A2", packageRef: "yzt",
			wantID: "vccb",
		},
		{
			name:    "a number channel alone resolves when the symbol's name is not the datasheet's",
			pinName: "VCC_B_RAIL", designator: "14", packageRef: "pw",
			wantID: "vccb",
		},
		{
			name:    "nothing to go on",
			pinName: "", designator: "", packageRef: "",
			wantErr: ErrPinUnknown,
		},
		{
			name:    "neither channel matches",
			pinName: "VBAT", designator: "99", packageRef: "pw",
			wantErr: ErrPinUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pin, err := ResolvePin(spec, tc.pinName, tc.designator, tc.packageRef)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ResolvePin = (%v, %v), want error %v", pin, err, tc.wantErr)
				}
				if pin != nil {
					t.Errorf("a refusal must return no pin, got %q", pin.Id)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolvePin: unexpected error %v", err)
			}
			if pin.Id != tc.wantID {
				t.Errorf("ResolvePin = %q, want %q", pin.Id, tc.wantID)
			}
		})
	}
}

// Degrade-safety (C9): a spec with no pin data must not resolve to a guess. Every
// pre-pin-binding spec in the corpus is in this state, so the answer has to be a
// distinguishable "no pin data", letting a caller fall back to the part-level path rather
// than treating the miss as a failed lookup.
func TestResolvePinOnAPinlessSpec(t *testing.T) {
	for _, name := range []string{"lm1117.textproto", "bss138.textproto"} {
		spec := readFixture(t, name)
		if len(spec.Pins) != 0 {
			t.Fatalf("%s: fixture is expected to carry no pin data", name)
		}
		pin, err := ResolvePin(spec, "VIN", "3", "")
		if !errors.Is(err, ErrNoPinData) {
			t.Errorf("%s: ResolvePin = (%v, %v), want ErrNoPinData", name, pin, err)
		}
	}
}

func TestPinParameters(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	vccb := PinParameters(spec, "vccb")
	if len(vccb) != 3 {
		t.Fatalf("vccb: want its abs-max, the group current row, and its recommended row; got %d", len(vccb))
	}
	kinds := map[parampb.LimitKind]bool{}
	for _, p := range vccb {
		kinds[p.LimitKind] = true
	}
	if !kinds[parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX] || !kinds[parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING] {
		t.Errorf("vccb: want both limit kinds, got %v", kinds)
	}

	// A pin whose ONLY limit arrives through a group binding still finds it.
	if got := PinParameters(spec, "gnd"); len(got) != 1 || got[0].Symbol != "I" {
		t.Errorf("gnd: want just the group current row, got %+v", got)
	}
	// Part-wide rows belong to no pin: an empty binding must NOT read as "every pin", or a caller
	// would credit a die-level rating as a terminal's own limit.
	for _, p := range PinParameters(spec, "gnd") {
		if len(p.PinRefs) == 0 {
			t.Errorf("gnd: part-wide row %q must not be returned as a pin's parameter", p.Symbol)
		}
	}
	if got := PinParameters(spec, "nope"); got != nil {
		t.Errorf("an undeclared pin id has no parameters, got %d", len(got))
	}
}

func TestPackageForMPN(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")
	cases := []struct{ mpn, want string }{
		{"TXB0104PW", "pw"},
		{"txb0104rut", "rut"},
		{"TXB0104PWR", "pw"}, // the reel suffix follows the package code
		{"TXB0104", ""},      // no package stated: the number channel stays unavailable
		{"TXB0104ZXU", ""},   // a real package this spec does not declare
	}
	for _, tc := range cases {
		pkg := PackageForMPN(spec, tc.mpn)
		got := ""
		if pkg != nil {
			got = pkg.Id
		}
		if got != tc.want {
			t.Errorf("PackageForMPN(%q) = %q, want %q", tc.mpn, got, tc.want)
		}
	}
}

func pinIDs(pins []*parampb.Pin) []string {
	out := make([]string, 0, len(pins))
	for _, p := range pins {
		out = append(out, p.Id)
	}
	return out
}

// A relation belongs to neither end, so it must be reachable from both. The TXB0104 states its
// supply ordering once, and a caller asking either supply what constrains it has to find it.
func TestPinRelations(t *testing.T) {
	spec := readFixture(t, "txb0104.textproto")

	subject := PinRelations(spec, "vcca")
	reference := PinRelations(spec, "vccb")
	if len(subject) != 1 || len(reference) != 1 {
		t.Fatalf("want the one relation from both ends; got %d from vcca and %d from vccb", len(subject), len(reference))
	}
	if subject[0] != reference[0] {
		t.Error("both ends must return the same relation, not a copy per end")
	}

	// The bound is on subject MINUS reference, so the direction is the part a caller must read
	// rather than assume: VCCA - VCCB <= 0 says VCCA is the one held down.
	rel := subject[0]
	if rel.GetSubjectPinRef() != "vcca" || rel.GetReferencePinRef() != "vccb" {
		t.Errorf("direction: got %q vs %q, want vcca vs vccb", rel.GetSubjectPinRef(), rel.GetReferencePinRef())
	}
	if rel.Difference.GetMax() != 0 || rel.Difference.Min != nil {
		t.Errorf("want a max-of-zero one-sided bound, got %+v", rel.Difference)
	}
	if rel.GetModality() != parampb.Modality_MODALITY_REQUIRED {
		t.Errorf("modality = %v, want REQUIRED; the datasheet says must", rel.GetModality())
	}

	// A pin in no relation, and a spec with none at all, both answer empty rather than panicking.
	if got := PinRelations(spec, "gnd"); len(got) != 0 {
		t.Errorf("gnd is in no relation, got %d", len(got))
	}
	if got := PinRelations(readFixture(t, "lm1117.textproto"), "anything"); len(got) != 0 {
		t.Errorf("a relationless spec must answer empty, got %d", len(got))
	}
}
