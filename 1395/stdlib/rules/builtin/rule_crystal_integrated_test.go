package builtin

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
)

// rtcDesign is a 32.768 kHz crystal Y1 straight across an RTC's oscillator pins, U1.1 (OSCI) and
// U1.2 (OSCO), with no capacitor on either, as the MNT Reform motherboard wires its PCF8523T.
func rtcDesign() *ir.Design {
	comp := func(ref, mpn string) *ir.Component {
		return &ir.Component{RefDes: ref, Mpn: mpn, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	return &ir.Design{
		Components: []*ir.Component{comp("Y1", ""), comp("U1", "DEMO-RTC")},
		Nets: []*ir.Net{
			tnet("OSCI", "Y1.1", "U1.1"),
			tnet("OSCO", "Y1.2", "U1.2"),
		},
	}
}

// rtcSpec is the RTC's datasheet, declaring its two oscillator pins and an integrated load
// capacitance row bound to the pins named in boundTo. An empty boundTo leaves the row part-wide.
func rtcSpec(boundTo ...string) *parampb.PartSpec {
	pin := func(id, name, number string) *parampb.Pin {
		return &parampb.Pin{Id: id, Name: name, Numbers: []*parampb.PinNumber{{Number: number}}}
	}
	return &parampb.PartSpec{
		Mpn:  "DEMO-RTC",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "DEMO-RTC datasheet Rev 1"}},
		Pins: []*parampb.Pin{pin("osci", "OSCI", "1"), pin("osco", "OSCO", "2")},
		Parameters: []*parampb.Parameter{{
			Name: "integrated load capacitance", Symbol: "CL(itg)",
			LimitKind: parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value:     &parampb.RangeValue{Min: ptr(4.2), Typ: ptr(7), Max: ptr(9.8)},
			Unit:      "pF", PinRefs: boundTo,
			Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 30, TableOrFigure: "Table 34. Static characteristics"},
		}},
	}
}

func ptr(v float64) *float64 { return &v }

func crystalVerdicts(t *testing.T, m check.Model) map[string]check.Verdict {
	t.Helper()
	out := map[string]check.Verdict{}
	for _, v := range crystalLoadCaps.Eval(context.Background(), m) {
		out[check.EntityRef(v.Subjects[0])] = v
	}
	return out
}

// An RTC whose datasheet binds integrated load capacitance to both oscillator pins needs no
// capacitor on the board, and the pass says so and cites the row (agni issue 1247).
func TestAnOscillatorWithIntegratedLoadCapsPassesTheCrystal(t *testing.T) {
	m := check.NewModel(rtcDesign(), check.WithParamProvider(param.ParamSet{"DEMO-RTC": rtcSpec("osci", "osco")}))
	vs := crystalVerdicts(t, m)
	if len(vs) != 2 {
		t.Fatalf("want a verdict per terminal, got %d: %+v", len(vs), vs)
	}
	for _, sub := range []string{"Y1.1", "Y1.2"} {
		v := vs[sub]
		if v.Outcome != check.Pass {
			t.Errorf("%s = %s, want pass", sub, v.Outcome)
			continue
		}
		if v.Witness == nil || !strings.Contains(v.Witness.Statement, "U1 integrates 7 pF of load capacitance") || len(v.Witness.Datasheet) != 1 {
			t.Errorf("%s witness = %+v, want U1's 7 pF cited from its datasheet", sub, v.Witness)
		}
	}
}

// A row bound to one oscillator pin credits only that terminal, and a part-wide row credits none,
// since an MCU often integrates capacitors on one oscillator and not the other.
func TestAnIntegratedLoadCapCreditsOnlyThePinItIsBoundTo(t *testing.T) {
	m := check.NewModel(rtcDesign(), check.WithParamProvider(param.ParamSet{"DEMO-RTC": rtcSpec("osci")}))
	vs := crystalVerdicts(t, m)
	if vs["Y1.1"].Outcome != check.Pass || vs["Y1.2"].Outcome != check.Fail {
		t.Errorf("row bound to OSCI: Y1.1 = %s, Y1.2 = %s, want pass and fail", vs["Y1.1"].Outcome, vs["Y1.2"].Outcome)
	}
	m = check.NewModel(rtcDesign(), check.WithParamProvider(param.ParamSet{"DEMO-RTC": rtcSpec()}))
	for sub, v := range crystalVerdicts(t, m) {
		if v.Outcome != check.Fail {
			t.Errorf("part-wide row: %s = %s, want fail", sub, v.Outcome)
		}
	}
}

// With no datasheet for the RTC the terminal still fails, and the witness says the absence is
// unverified and names the part it could not read. Its finding message still matches the message
// the datalog twin produces.
func TestACrystalOnAnUnreadICIsReportedUnverified(t *testing.T) {
	vs := crystalVerdicts(t, check.NewModel(rtcDesign()))
	for _, sub := range []string{"Y1.1", "Y1.2"} {
		v := vs[sub]
		if v.Outcome != check.Fail {
			t.Fatalf("%s = %s, want fail", sub, v.Outcome)
		}
		if !strings.Contains(v.Witness.Statement, "U1's datasheet was not read, so whether it integrates the load capacitance is unverified") {
			t.Errorf("%s witness = %q, want it to say U1 was not read", sub, v.Witness.Statement)
		}
		if v.Finding == nil || !strings.HasSuffix(v.Finding.Message, "has no load capacitor") {
			t.Errorf("%s finding = %+v, want the twin's message", sub, v.Finding)
		}
	}

	// A read datasheet that declares nothing is not unverified, it is a plain absence.
	bare := rtcSpec()
	bare.Parameters = nil
	vs = crystalVerdicts(t, check.NewModel(rtcDesign(), check.WithParamProvider(param.ParamSet{"DEMO-RTC": bare})))
	if s := vs["Y1.1"].Witness.Statement; strings.Contains(s, "unverified") {
		t.Errorf("with U1's datasheet read: witness = %q, want no unverified note", s)
	}
}
