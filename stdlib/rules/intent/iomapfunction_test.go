package intent

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// functionModel is ioMapDesign with U101 carrying a part number and a datasheet entry whose pin
// PTC11 lists three functions and whose PTE7 lists none.
func functionModel(withSpec bool) check.Model {
	d := ioMapDesign()
	d.Components[0].Mpn = "ACME-MCU"
	if !withSpec {
		return check.NewModel(d)
	}
	prov := &parampb.ParamProvenance{DocRef: "src", Page: 58, TableOrFigure: "Table 58. Package pinout", Confidence: 0.9}
	spec := &parampb.PartSpec{
		Mpn:  "ACME-MCU",
		Docs: []*parampb.SourceDoc{{Id: "src", Title: "ACME-MCU data sheet Rev. 4"}},
		Pins: []*parampb.Pin{
			{Id: "ptc11", Name: "PTC11", Prov: prov, Alternates: []*parampb.PinAlternate{
				{Name: "ADC0_S17"}, {Name: "PTC11"}, {Name: "LPUART0_RX"}, {Name: "ADC0_A10/CMP0_IN0"},
			}},
			{Id: "pte7", Name: "PTE7", Prov: prov},
		},
	}
	return check.NewModel(d, check.WithParamProvider(param.ParamSet{"ACME-MCU": spec}))
}

func functionVerdict(t *testing.T, m check.Model, row string) check.Verdict {
	t.Helper()
	vs := verdictsFor(t, ioMapDecl(t, row), m, RuleIOMapFunction)
	if len(vs) != 1 {
		t.Fatalf("%d verdicts, want 1", len(vs))
	}
	return vs[0]
}

func TestADeclaredFunctionTheDatasheetListsPasses(t *testing.T) {
	v := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}")
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s (%q)", v.Outcome, v.Reason)
	}
	if !strings.Contains(v.Witness.Statement, `"ADC0_S17"`) || strings.Contains(v.Witness.Statement, "match") {
		t.Errorf("an exact pass names the function and no inferred match: %q", v.Witness.Statement)
	}
	if len(v.Witness.Datasheet) != 1 || v.Witness.Datasheet[0] == nil {
		t.Errorf("a pass cites the datasheet table it read: %+v", v.Witness.Datasheet)
	}
}

// One slot naming two signals is split, so either half passes.
func TestOneHalfOfASlotNamingTwoSignalsPasses(t *testing.T) {
	v := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: '41', function: CMP0_IN0}")
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s (%q)", v.Outcome, v.Reason)
	}
}

// A match the comparison had to infer passes and says so.
func TestALooseFunctionMatchPassesWithANote(t *testing.T) {
	v := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: adc0_s17}")
	if v.Outcome != check.Pass {
		t.Fatalf("outcome = %s (%q)", v.Outcome, v.Reason)
	}
	if !strings.Contains(v.Witness.Statement, "normalized match") {
		t.Errorf("a normalized match must say so: %q", v.Witness.Statement)
	}
}

func TestAFunctionTheDatasheetDoesNotListFails(t *testing.T) {
	v := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: LPI2C1_SDA}")
	if v.Outcome != check.Fail || v.Finding == nil {
		t.Fatalf("outcome = %s, finding %v", v.Outcome, v.Finding)
	}
	for _, want := range []string{"LPI2C1_SDA", "ADC0_S17", "LPUART0_RX"} {
		if !strings.Contains(v.Finding.Message, want) {
			t.Errorf("the finding must name %s, so the listed functions are in front of the reader: %q", want, v.Finding.Message)
		}
	}
	if len(v.Finding.DatasheetProv) != 1 {
		t.Errorf("a fail cites the datasheet: %+v", v.Finding.DatasheetProv)
	}
}

// Without a datasheet to compare against, a declared function is never reported as checked: the row
// is not-considered with the reason, and no other IO-map rule mentions the function.
func TestADeclaredFunctionWithNothingToCompareIsNotConsidered(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    check.Model
		row  string
		want string
	}{
		{"no datasheet entry", functionModel(false), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}", "no entry for U101's part ACME-MCU"},
		{"no function table for the pin", functionModel(true), "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", "lists no alternate functions"},
		{"no function declared", functionModel(true), "  - {net: PMIC_PG, device: U101, pin: PTE7}", "declares no function"},
	} {
		decl := ioMapDecl(t, tc.row+"\n  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}")
		vs := verdictsFor(t, decl, tc.m, RuleIOMapFunction)
		if len(vs) != 2 {
			t.Fatalf("%s: %d verdicts, want 2", tc.name, len(vs))
		}
		v := vs[0]
		if v.Outcome != check.NotConsidered || !strings.Contains(v.Reason, tc.want) {
			t.Errorf("%s: outcome %s, reason %q; want not-considered naming %q", tc.name, v.Outcome, v.Reason, tc.want)
		}
	}
	for _, v := range verdictsFor(t, ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}"),
		functionModel(false), RuleIOMapPin) {
		if strings.Contains(v.Witness.Statement+v.Reason, "ADC0_S17") {
			t.Errorf("the pin rule speaks about the function, which is io-map-function's: %q", v.Witness.Statement)
		}
	}
}

// A map with no function column compiles no function rule, which could only say not-considered.
func TestAMapWithNoFunctionsCompilesNoFunctionRule(t *testing.T) {
	for _, r := range Compile(ioMapDecl(t, "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11}")) {
		if r.Name == RuleIOMapFunction {
			t.Fatal("io-map-function compiled for a map declaring no function")
		}
	}
}

// Every step that can stop io-map-function reads back to its own kind, from the reason the rule
// itself wrote, so a report counting kinds stays right when a sentence is reworded (agni issue 1196).
func TestEveryNotConsideredReasonReadsBackToItsKind(t *testing.T) {
	noPins := ioMapDesign()
	noPins.Components[0].Mpn = "ACME-MCU"
	emptySpec := check.NewModel(noPins, check.WithParamProvider(param.ParamSet{"ACME-MCU": {
		Mpn: "ACME-MCU", Docs: []*parampb.SourceDoc{{Id: "src"}},
	}}))
	otherPins := ioMapDesign()
	otherPins.Components[0].Mpn = "ACME-MCU"
	prov := &parampb.ParamProvenance{DocRef: "src", Page: 1, Confidence: 0.9}
	onlyPTC11 := check.NewModel(otherPins, check.WithParamProvider(param.ParamSet{"ACME-MCU": {
		Mpn: "ACME-MCU", Docs: []*parampb.SourceDoc{{Id: "src"}},
		Pins: []*parampb.Pin{{Id: "ptc11", Name: "PTC11", Prov: prov, Numbers: []*parampb.PinNumber{}}},
	}}))
	noMPN := check.NewModel(ioMapDesign())
	withJ9 := ioMapDesign()
	withJ9.Components = append(withJ9.Components, &ir.Component{RefDes: "J9", Prov: &ir.Provenance{SourceFile: "t"}})
	pinless := check.NewModel(withJ9)
	for _, tc := range []struct {
		m    check.Model
		row  string
		want FunctionReason
	}{
		{functionModel(true), "  - {net: PMIC_PG, device: U101, pin: PTE7}", FunctionReasonNotDeclared},
		{functionModel(true), "  - {net: PMIC_PG, device: U999, pin: '1', function: GPIO}", FunctionReasonPinUnresolved},
		{pinless, "  - {net: PMIC_PG, device: J9, pin: '1', function: GPIO}", FunctionReasonPinUnresolved},
		{noMPN, "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", FunctionReasonNoPartNumber},
		{functionModel(false), "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", FunctionReasonNoDatasheet},
		{emptySpec, "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", FunctionReasonNoDatasheetPins},
		{onlyPTC11, "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", FunctionReasonSpecPinUnknown},
		{functionModel(true), "  - {net: PMIC_PG, device: U101, pin: PTE7, function: GPIO}", FunctionReasonNoPinMux},
	} {
		vs := verdictsFor(t, ioMapDecl(t, tc.row+"\n  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}"), tc.m, RuleIOMapFunction)
		if len(vs) == 0 || vs[0].Outcome != check.NotConsidered {
			t.Errorf("%s: want a not-considered first verdict, got %+v", tc.want, vs)
			continue
		}
		if got := FunctionReasonOf(vs[0].Reason); got != tc.want {
			t.Errorf("%q reads back as %s, want %s", vs[0].Reason, got, tc.want)
		}
	}
	if FunctionReasonOf("something else entirely") != FunctionReasonOther {
		t.Error("text the rule did not write must read as other")
	}
}

// A witness that notes a normalized or fuzzy match reads as inferred, and an exact one does not.
func TestInferredMatchReadsTheWitnessNote(t *testing.T) {
	exact := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: ADC0_S17}")
	loose := functionVerdict(t, functionModel(true), "  - {net: ADC_BATT_SENSE, device: U101, pin: PTC11, function: adc0_s17}")
	if InferredMatch(exact.Witness.Statement) || !InferredMatch(loose.Witness.Statement) {
		t.Errorf("exact %q inferred=%v, loose %q inferred=%v", exact.Witness.Statement, InferredMatch(exact.Witness.Statement),
			loose.Witness.Statement, InferredMatch(loose.Witness.Statement))
	}
}
