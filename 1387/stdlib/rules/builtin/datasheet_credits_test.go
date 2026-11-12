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

// usbEntry is a USB connector J1 whose VBUS reaches U1, a current-limiting power switch, through the
// ferrite FB1, as Glasgow wires its TPD3S014: J1 -> VBUS -> FB1 -> VUSB -> U1 IN, U1 OUT -> +5V ->
// U2. withRegulator puts a regulator U3 on VUSB beside the switch.
func usbEntry(withRegulator bool) *ir.Design {
	sw := &ir.PartType{Name: "SW", Pins: []*ir.Pin{
		{Name: "IN", Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
		{Name: "OUT", Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_POWER_OUT},
	}}
	load := &ir.PartType{Name: "LOAD", Pins: []*ir.Pin{
		{Name: "VDD", Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
	}}
	ferrite := &ir.PartType{Name: "FB", Pins: []*ir.Pin{{Name: "1", Designator: "1"}, {Name: "2", Designator: "2"}}}
	comp := func(ref, part, mpn string) *ir.Component {
		c := &ir.Component{RefDes: ref, Mpn: mpn, Prov: &ir.Provenance{SourceFile: "t"}}
		if part != "" {
			c.Sections = []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}
		}
		return c
	}
	d := &ir.Design{
		Libraries:  []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{sw, load, ferrite}}},
		Components: []*ir.Component{comp("J1", "", ""), comp("FB1", "FB", ""), comp("U1", "SW", "DEMO-USBSW"), comp("U2", "LOAD", "")},
		Nets: []*ir.Net{
			tnet("VBUS", "J1.1", "FB1.1"),
			tnet("VUSB", "FB1.2", "U1.3"),
			tnet("+5V", "U1.4", "U2.1"),
		},
	}
	if withRegulator {
		d.Components = append(d.Components, comp("U3", "LOAD", "DEMO-LDO"))
		d.Nets[1].Connections = append(d.Nets[1].Connections, &ir.Connection{ComponentRef: "U3", PinRef: "1"})
	}
	return d
}

// switchSpec is a current-limiting USB power switch's datasheet, with a reverse-current row when
// blocks is set.
func switchSpec(mpn, class string, blocks bool) *parampb.PartSpec {
	s := &parampb.PartSpec{
		Mpn: mpn, DeviceClass: class,
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: mpn + " datasheet Rev B"}},
		Parameters: []*parampb.Parameter{{
			Name: "Current limit", Symbol: "IOS", LimitKind: parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value: &parampb.RangeValue{Min: ptr(0.67), Typ: ptr(0.85), Max: ptr(1.01)}, Unit: "A",
			Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 5, TableOrFigure: "Electrical Characteristics"},
		}},
	}
	if blocks {
		s.Parameters = append(s.Parameters, &parampb.Parameter{
			Name: "Reverse leakage current", Symbol: "IREV", LimitKind: parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value: &parampb.RangeValue{Typ: ptr(0.2), Max: ptr(1)}, Unit: "µA",
			Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 5, TableOrFigure: "Electrical Characteristics"},
		})
	}
	return s
}

func entryVerdict(t *testing.T, rule *check.Rule, m check.Model) check.Verdict {
	t.Helper()
	for _, v := range rule.Eval(context.Background(), m) {
		if check.EntityRef(v.Subjects[0]) == "VBUS" {
			return v
		}
	}
	t.Fatalf("%s gave no verdict on VBUS", rule.Name)
	return check.Verdict{}
}

// Here the entry's only load is a USB power switch with a current limit and reverse blocking. It
// answers both power-entry rules, and each pass cites its datasheet (agni issue 1248).
func TestACurrentLimitingSwitchGuardsAPowerEntry(t *testing.T) {
	m := check.NewModel(usbEntry(false), check.WithParamProvider(param.ParamSet{"DEMO-USBSW": switchSpec("DEMO-USBSW", "USB power switch", true)}))
	ip := entryVerdict(t, inputProtection, m)
	if ip.Outcome != check.Pass || !strings.Contains(ip.Witness.Statement, "U1 is a current-limiting power switch that limits at 0.85 A") || len(ip.Witness.Datasheet) != 1 {
		t.Errorf("input-protection = %s, %+v, want a pass citing U1's 0.85 A limit", ip.Outcome, ip.Witness)
	}
	rb := entryVerdict(t, reverseBlockingAbsent, m)
	if rb.Outcome != check.Pass || !strings.Contains(rb.Witness.Statement, "U1 blocks reverse current, at most 1 µA") ||
		!strings.Contains(rb.Witness.Statement, "not a reversed supply") || len(rb.Witness.Datasheet) != 1 {
		t.Errorf("reverse-blocking-absent = %s, %+v, want a pass citing U1's reverse current and saying it does not stop reverse polarity", rb.Outcome, rb.Witness)
	}
}

// A switch whose datasheet states no reverse current guards the entry against overcurrent and blocks
// nothing, as many eFuses do.
func TestASwitchWithoutReverseBlockingAnswersOnlyInputProtection(t *testing.T) {
	m := check.NewModel(usbEntry(false), check.WithParamProvider(param.ParamSet{"DEMO-USBSW": switchSpec("DEMO-USBSW", "efuse", false)}))
	if v := entryVerdict(t, inputProtection, m); v.Outcome != check.Pass {
		t.Errorf("input-protection = %s, want pass", v.Outcome)
	}
	if v := entryVerdict(t, reverseBlockingAbsent, m); v.Outcome != check.Fail {
		t.Errorf("reverse-blocking-absent = %s, want fail", v.Outcome)
	}
}

// A regulator limits its current too, and is no protection against its own failure, so only a part
// whose datasheet names it a current-limiting switch is credited. A switch beside a regulator on the
// entry does not excuse the regulator.
func TestOnlyASwitchThatIsTheWholeLoadIsCredited(t *testing.T) {
	ldo := check.NewModel(usbEntry(false), check.WithParamProvider(param.ParamSet{"DEMO-USBSW": switchSpec("DEMO-USBSW", "ldo", true)}))
	for _, r := range []*check.Rule{inputProtection, reverseBlockingAbsent} {
		if v := entryVerdict(t, r, ldo); v.Outcome != check.Fail {
			t.Errorf("%s with a current-limited regulator = %s, want fail", r.Name, v.Outcome)
		}
	}
	beside := check.NewModel(usbEntry(true), check.WithParamProvider(param.ParamSet{"DEMO-USBSW": switchSpec("DEMO-USBSW", "USB power switch", true)}))
	for _, r := range []*check.Rule{inputProtection, reverseBlockingAbsent} {
		if v := entryVerdict(t, r, beside); v.Outcome != check.Fail {
			t.Errorf("%s with a regulator beside the switch = %s, want fail", r.Name, v.Outcome)
		}
	}
}

// Without a datasheet both rules still fail, and each names the IC it could not read.
func TestAPowerEntryOnAnUnreadICIsReportedUnverified(t *testing.T) {
	m := check.NewModel(usbEntry(false))
	for _, c := range []struct {
		rule *check.Rule
		says string
	}{
		{inputProtection, "U1's datasheet was not read, so whether it limits its current is unverified"},
		{reverseBlockingAbsent, "U1's datasheet was not read, so whether it blocks reverse current is unverified"},
	} {
		v := entryVerdict(t, c.rule, m)
		if v.Outcome != check.Fail || !strings.Contains(v.Witness.Statement, c.says) {
			t.Errorf("%s = %s, %q, want a fail saying %q", c.rule.Name, v.Outcome, v.Witness.Statement, c.says)
		}
	}
}

// translatorBus is an I2C pair whose only pull-up is inside U2, a level translator. U2's datasheet
// binds an internal pull-up to its A1 and A2 pins, as the MNT Reform's TXS0108E pulls its SD lines up
// (agni issue 1298). boundTo names the pins the row is bound to, none for a part-wide row.
func translatorBus(boundTo ...string) check.Model {
	d := &ir.Design{
		Components: []*ir.Component{
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U2", Mpn: "DEMO-XLATE", Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{tnet("SDA", "U1.1", "U2.2"), tnet("SCL", "U1.2", "U2.3")},
	}
	pin := func(id, name, number string) *parampb.Pin {
		return &parampb.Pin{Id: id, Name: name, Numbers: []*parampb.PinNumber{{Number: number}}}
	}
	spec := &parampb.PartSpec{
		Mpn:  "DEMO-XLATE",
		Docs: []*parampb.SourceDoc{{Id: "ds", Title: "DEMO-XLATE datasheet Rev A"}},
		Pins: []*parampb.Pin{pin("a1", "A1", "2"), pin("a2", "A2", "3")},
		Parameters: []*parampb.Parameter{{
			Name: "Internal pull-up resistance", Symbol: "RPU", LimitKind: parampb.LimitKind_LIMIT_KIND_CHARACTERISTIC,
			Value: &parampb.RangeValue{Typ: ptr(10)}, Unit: "kΩ", PinRefs: boundTo,
			Prov: &parampb.ParamProvenance{DocRef: "ds", Page: 6, TableOrFigure: "Electrical Characteristics"},
		}},
	}
	return check.NewModel(d, check.WithParamProvider(param.ParamSet{"DEMO-XLATE": spec}))
}

// A bus line pulled up inside a part's pin passes i2c-pull-up citing the part. The declarative twin
// reads PullUpReachesRail and agrees.
func TestAnIntegratedPullUpPullsTheLineUp(t *testing.T) {
	m := translatorBus("a1", "a2")
	for _, v := range i2cPullUp.Eval(context.Background(), m) {
		if v.Outcome != check.Pass || !strings.Contains(v.Witness.Statement, "U2 pulls its") || !strings.Contains(v.Witness.Statement, "10 kΩ") || len(v.Witness.Datasheet) != 1 {
			t.Errorf("%s = %s, %+v, want a pass citing U2's 10 kΩ", check.EntityRef(v.Subjects[0]), v.Outcome, v.Witness)
		}
	}
	if fs := i2cPullUpSpec.Eval(context.Background(), m); len(fs) != 0 {
		t.Errorf("the twin reports %d findings on a bus pulled up inside U2: %+v", len(fs), fs)
	}
	part := translatorBus()
	if fs := check.RunBackground(part, []*check.Rule{i2cPullUp}); len(fs) != 2 {
		t.Errorf("a part-wide pull-up row credits a pin it is not bound to: want 2 findings, got %d", len(fs))
	}
}
