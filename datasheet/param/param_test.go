package param

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

func readFixture(t *testing.T, name string) *parampb.PartSpec {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	defer f.Close()
	spec, err := Load(f)
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return spec
}

func TestFixturesValidate(t *testing.T) {
	for _, name := range []string{"lm1117.textproto", "bss138.textproto", "txb0104.textproto"} {
		spec := readFixture(t, name)
		if err := Validate(spec); err != nil {
			t.Errorf("%s: Validate: %v", name, err)
		}
	}
}

// findParam returns the parameters matching a symbol and limit kind (a datasheet
// specs the same symbol under several kinds and condition sets).
func findParams(spec *parampb.PartSpec, symbol string, kind parampb.LimitKind) []*parampb.Parameter {
	var out []*parampb.Parameter
	for _, p := range spec.Parameters {
		if p.Symbol == symbol && p.LimitKind == kind {
			out = append(out, p)
		}
	}
	return out
}

// The LM1117 fixture must carry all three limit kinds for the input voltage story:
// abs-max 20 V, recommended-operating 15 V, and the dropout characteristic rows with
// their output-current and temperature conditions. Values from SNOS412Q (Jan 2023).
func TestLM1117Encoding(t *testing.T) {
	spec := readFixture(t, "lm1117.textproto")
	if spec.Mpn != "LM1117" {
		t.Fatalf("mpn = %q, want LM1117", spec.Mpn)
	}

	absMax := findParams(spec, "VIN", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX)
	if len(absMax) != 1 || absMax[0].Value.GetMax() != 20 {
		t.Errorf("abs-max VIN: want exactly one row with max 20, got %+v", absMax)
	}
	if len(absMax) == 1 {
		if p := absMax[0].Prov; p == nil || p.Page != 4 || !strings.Contains(p.TableOrFigure, "Absolute Maximum") {
			t.Errorf("abs-max VIN provenance should point at the page-4 abs-max table, got %+v", p)
		}
		if absMax[0].Value.Min != nil || absMax[0].Value.Typ != nil {
			t.Errorf("abs-max VIN is max-only in the source; min/typ must be absent, got %+v", absMax[0].Value)
		}
	}

	recOp := findParams(spec, "VIN", parampb.LimitKind_LIMIT_KIND_RECOMMENDED_OPERATING)
	if len(recOp) != 1 || recOp[0].Value.GetMax() != 15 {
		t.Errorf("recommended-operating VIN: want exactly one row with max 15, got %+v", recOp)
	}

	// Two dropout rows: typ 1.2 at TJ=25C and max 1.3 over 0..125C, both at
	// IOUT = 800 mA. Distinct condition sets stay distinct rows.
	drop := findParams(spec, "VIN - VOUT", parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC)
	var sawTyp, sawMax bool
	for _, p := range drop {
		var iout *parampb.Condition
		for _, c := range p.Conditions {
			if c.Symbol == "IOUT" {
				iout = c
			}
		}
		if iout == nil || iout.GetEq() != 800 || iout.Unit != "mA" {
			continue
		}
		if p.Value.Typ != nil && p.Value.GetTyp() == 1.2 {
			sawTyp = true
		}
		if p.Value.Max != nil && p.Value.GetMax() == 1.3 {
			sawMax = true
			var tj *parampb.Condition
			for _, c := range p.Conditions {
				if c.Symbol == "TJ" {
					tj = c
				}
			}
			if tj == nil || tj.GetMin() != 0 || tj.GetMax() != 125 {
				t.Errorf("dropout max row must carry the TJ 0..125 range condition, got %+v", tj)
			}
		}
	}
	if !sawTyp || !sawMax {
		t.Errorf("dropout at IOUT=800mA: want typ-1.2 and max-1.3 rows, sawTyp=%v sawMax=%v (%d rows)", sawTyp, sawMax, len(drop))
	}
}

// The BSS138 fixture must keep RDS(on)'s three condition sets as three rows,
// including the TJ=125C one. Values from the Fairchild Rev C(W) sheet (Oct 2005).
func TestBSS138Encoding(t *testing.T) {
	spec := readFixture(t, "bss138.textproto")

	vgss := findParams(spec, "VGSS", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX)
	if len(vgss) != 1 || vgss[0].Value.GetMin() != -20 || vgss[0].Value.GetMax() != 20 {
		t.Errorf("VGSS abs-max: want one row spanning -20..20, got %+v", vgss)
	}

	rds := findParams(spec, "RDS(on)", parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC)
	if len(rds) != 3 {
		t.Fatalf("RDS(on): want the 3 condition-set rows from the sheet, got %d", len(rds))
	}
	var hot *parampb.Parameter
	for _, p := range rds {
		for _, c := range p.Conditions {
			if c.Symbol == "TJ" && c.GetEq() == 125 {
				hot = p
			}
		}
	}
	if hot == nil {
		t.Fatal("RDS(on): missing the TJ=125C row")
	}
	if hot.Value.GetTyp() != 1.1 || hot.Value.GetMax() != 5.8 {
		t.Errorf("RDS(on) @ TJ=125C: want typ 1.1 max 5.8, got %+v", hot.Value)
	}
	if hot.ConditionCoverage != parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE {
		t.Errorf("RDS(on) @ TJ=125C carries all its printed conditions; coverage = %v", hot.ConditionCoverage)
	}
	if p := hot.Prov; p == nil || p.Page != 2 || p.Method != "hand" || p.Confidence != 1 {
		t.Errorf("RDS(on) provenance: want page 2, hand, confidence 1, got %+v", p)
	}
}

func TestUnderSpecified(t *testing.T) {
	base := func() *parampb.Parameter {
		return &parampb.Parameter{
			Conditions: []*parampb.Condition{{Symbol: "TJ", Eq: f64(25), Unit: "C"}},
		}
	}
	cases := []struct {
		name string
		mut  func(*parampb.Parameter)
		want bool
	}{
		{"conditions and complete coverage", func(p *parampb.Parameter) {
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE
		}, false},
		{"conditions but coverage unknown", func(p *parampb.Parameter) {}, true},
		{"conditions but known-partial", func(p *parampb.Parameter) {
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_PARTIAL
		}, true},
		{"no conditions, source states none", func(p *parampb.Parameter) {
			p.Conditions = nil
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL
		}, false},
		{"no conditions, coverage unknown", func(p *parampb.Parameter) {
			p.Conditions = nil
		}, true},
	}
	for _, tc := range cases {
		p := base()
		tc.mut(p)
		if got := UnderSpecified(p); got != tc.want {
			t.Errorf("%s: UnderSpecified = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMachineComparable(t *testing.T) {
	structured := func() *parampb.Parameter {
		return &parampb.Parameter{
			Conditions:        []*parampb.Condition{{Symbol: "TJ", Eq: f64(25), Unit: "C"}},
			ConditionCoverage: parampb.ConditionCoverage_CONDITION_COVERAGE_COMPLETE,
		}
	}
	cases := []struct {
		name string
		mut  func(*parampb.Parameter)
		want bool
	}{
		{"structured conditions, complete", func(p *parampb.Parameter) {}, true},
		{"range-bound condition", func(p *parampb.Parameter) {
			p.Conditions[0] = &parampb.Condition{Symbol: "TJ", Min: f64(0), Max: f64(125), Unit: "C"}
		}, true},
		{"raw-only condition", func(p *parampb.Parameter) {
			p.Conditions = append(p.Conditions, &parampb.Condition{Symbol: "VDS", Raw: "VDS = VGS"})
		}, false},
		{"genuinely unconditional", func(p *parampb.Parameter) {
			p.Conditions = nil
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_UNCONDITIONAL
		}, true},
		{"under-specified is never comparable", func(p *parampb.Parameter) {
			p.ConditionCoverage = parampb.ConditionCoverage_CONDITION_COVERAGE_PARTIAL
		}, false},
	}
	for _, tc := range cases {
		p := structured()
		tc.mut(p)
		if got := MachineComparable(p); got != tc.want {
			t.Errorf("%s: MachineComparable = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// The fixtures carry both comparable and text-only-condition rows; the predicate must
// split them the way a consumer would need.
func TestMachineComparableOnFixtures(t *testing.T) {
	bss := readFixture(t, "bss138.textproto")
	for _, p := range findParams(bss, "RDS(on)", parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC) {
		if !MachineComparable(p) {
			t.Errorf("RDS(on) rows have fully structured conditions; MachineComparable = false: %+v", p.Conditions)
		}
	}
	vgsth := findParams(bss, "VGS(th)", parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC)
	if len(vgsth) != 1 || MachineComparable(vgsth[0]) {
		t.Errorf("VGS(th) carries the raw-only VDS = VGS condition; it must not be machine-comparable")
	}
	lm := readFixture(t, "lm1117.textproto")
	absMax := findParams(lm, "VIN", parampb.LimitKind_LIMIT_KIND_ABSOLUTE_MAX)
	if len(absMax) != 1 || MachineComparable(absMax[0]) {
		t.Errorf("abs-max VIN carries the raw-only temperature-range condition; it must not be machine-comparable")
	}
	if UnderSpecified(absMax[0]) {
		t.Errorf("abs-max VIN is fully captured (coverage COMPLETE); text-only conditions must not make it under-specified")
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*parampb.PartSpec)
		want string
	}{
		{"missing mpn", func(s *parampb.PartSpec) { s.Mpn = "" }, "mpn"},
		{"unspecified limit kind", func(s *parampb.PartSpec) {
			s.Parameters[0].LimitKind = parampb.LimitKind_LIMIT_KIND_UNSPECIFIED
		}, "limit_kind"},
		{"missing provenance", func(s *parampb.PartSpec) { s.Parameters[0].Prov = nil }, "prov"},
		{"dangling doc_ref", func(s *parampb.PartSpec) { s.Parameters[0].Prov.DocRef = "nope" }, "doc_ref"},
		{"zero confidence", func(s *parampb.PartSpec) { s.Parameters[0].Prov.Confidence = 0 }, "confidence"},
		{"empty value", func(s *parampb.PartSpec) { s.Parameters[0].Value = &parampb.RangeValue{} }, "value"},
		{"min above max", func(s *parampb.PartSpec) {
			s.Parameters[0].Value = &parampb.RangeValue{Min: f64(2), Max: f64(1)}
		}, "min"},
	}
	for _, tc := range cases {
		spec := readFixture(t, "lm1117.textproto")
		tc.mut(spec)
		err := Validate(spec)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want error mentioning %q", tc.name, err, tc.want)
		}
	}
}

// Pin binding is only worth having if an incoherent one is caught at load: a parameter
// naming a pin the spec never declares, or a pin numbered into a package that does not
// exist, would otherwise sit in a corpus until a rule silently resolved nothing.
func TestValidateRejectsPinBinding(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*parampb.PartSpec)
		want string
	}{
		{"parameter binds to an undeclared pin", func(s *parampb.PartSpec) {
			s.Parameters[0].PinRefs = []string{"no_such_pin"}
		}, "pin_refs"},
		{"pin without an id cannot be bound to", func(s *parampb.PartSpec) {
			s.Pins[0].Id = ""
		}, "id"},
		{"two pins claiming one id make a binding ambiguous", func(s *parampb.PartSpec) {
			s.Pins[1].Id = s.Pins[0].Id
		}, "duplicate"},
		{"pin without a name loses the channel that survives repackaging", func(s *parampb.PartSpec) {
			s.Pins[0].Name = ""
		}, "name"},
		{"pin numbered into an undeclared package", func(s *parampb.PartSpec) {
			s.Pins[0].Numbers[0].PackageRef = "nope"
		}, "package_ref"},
		{"a number claimed by two pins in one package", func(s *parampb.PartSpec) {
			s.Pins[1].Numbers[0].PackageRef = s.Pins[0].Numbers[0].PackageRef
			s.Pins[1].Numbers[0].Number = s.Pins[0].Numbers[0].Number
		}, "number"},
		{"package without an id cannot be referenced", func(s *parampb.PartSpec) {
			s.Packages[0].Id = ""
		}, "id"},
		{"two packages claiming one id", func(s *parampb.PartSpec) {
			s.Packages[1].Id = s.Packages[0].Id
		}, "duplicate"},
		{"pin without provenance", func(s *parampb.PartSpec) {
			s.Pins[0].Prov = nil
		}, "prov"},
		{"pin provenance with a dangling doc_ref", func(s *parampb.PartSpec) {
			s.Pins[0].Prov.DocRef = "nope"
		}, "doc_ref"},
		{"pin provenance nobody stands behind", func(s *parampb.PartSpec) {
			s.Pins[0].Prov.Confidence = 0
		}, "confidence"},
	}
	for _, tc := range cases {
		spec := readFixture(t, "txb0104.textproto")
		tc.mut(spec)
		err := Validate(spec)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Validate = %v, want error mentioning %q", tc.name, err, tc.want)
		}
	}
}

// Degrade-safety (C9): the pin rules must fire only on specs that carry pin data. A spec
// seeded before pin binding existed validates exactly as it did.
func TestValidateAcceptsPinlessSpecs(t *testing.T) {
	for _, name := range []string{"lm1117.textproto", "bss138.textproto"} {
		spec := readFixture(t, name)
		if len(spec.Pins) != 0 || len(spec.Packages) != 0 {
			t.Fatalf("%s: fixture is expected to carry no pin or package data", name)
		}
		if err := Validate(spec); err != nil {
			t.Errorf("%s: Validate: %v", name, err)
		}
	}
}

func f64(v float64) *float64 { return &v }
