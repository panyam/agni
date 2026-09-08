package profiles

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// directPullUpDesign: U1's CS line is pulled to a WIDE rail by R1 sitting directly on both. Wide
// matters: the reach walk refuses to enter a net whose fan-out exceeds maxWalkFan (WS3-108), and a
// real rail is wide, so this is the ordinary case rather than an awkward one.
func directPullUpDesign(csNet string) *ir.Design {
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	conn := func(r, p string) *ir.Connection { return &ir.Connection{ComponentRef: r, PinRef: p} }
	rail := &ir.Net{
		Name: "+3V3", Prov: &ir.Provenance{SourceFile: "t"},
		Attributes:  map[string]string{"global": "true"},
		Connections: []*ir.Connection{conn("R1", "2")},
	}
	comps := []*ir.Component{comp("U1"), comp("U2"), comp("R1")}
	for i := 0; i < 20; i++ {
		ref := "C" + string(rune('A'+i))
		comps = append(comps, comp(ref))
		rail.Connections = append(rail.Connections, conn(ref, "1"))
	}
	return &ir.Design{
		Components: comps,
		Nets: []*ir.Net{
			{Name: csNet, Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{conn("U1", "1"), conn("R1", "1")}},
			{Name: "SPI_SCLK", Prov: &ir.Provenance{SourceFile: "t"}, Connections: []*ir.Connection{conn("U1", "2"), conn("U2", "2")}},
			rail,
		},
	}
}

func spiProfile() Profile {
	return Profile{
		Name: "SPI_NOR",
		Signals: []Signal{
			{Name: "CS", Suffix: "_CS", PullUp: true},
			{Name: "SCLK", Suffix: "_SCLK"},
		},
	}
}

// THE ASYMMETRY issue 516 names. A profile's pull-up finding proved a pass with the net's own name
// while the built-in proved one with the path it walked, so "show me the pull-up" got an answer on
// I2C and a restatement of the question everywhere else.
func TestAProfilePassNamesTheResistorAndTheRail(t *testing.T) {
	m := check.NewModel(directPullUpDesign("SPI_CS"))
	rule := spiProfile().pullupRule()
	if rule == nil {
		t.Fatal("no pull-up rule compiled")
	}
	for _, v := range rule.Eval(m) {
		if check.EntityRef(v.Subjects[0]) != "SPI_CS" {
			continue
		}
		if v.Outcome != check.Pass {
			t.Fatalf("outcome = %v, want pass (R1 pulls SPI_CS to +3V3)", v.Outcome)
		}
		if v.Witness == nil || !strings.Contains(v.Witness.Statement, "R1") || !strings.Contains(v.Witness.Statement, "+3V3") {
			t.Fatalf("witness = %+v, want it to name the resistor and the rail", v.Witness)
		}
		var roles []string
		for _, c := range v.Context {
			roles = append(roles, c.Role)
		}
		if len(roles) != 2 || roles[0] != "pull-up" || roles[1] != "rail" {
			t.Errorf("context roles = %v, want the resistor and the rail as clickable entities", roles)
		}
		return
	}
	t.Fatal("SPI_CS produced no verdict")
}

// The two routes now answer identically because they are the same function, so this compares them
// rather than asserting one string twice.
func TestTheProfileAndTheBuiltinProveAPassTheSameWay(t *testing.T) {
	m := check.NewModel(directPullUpDesign("SPI_CS"))
	var net *ir.Net
	for _, n := range m.Nets() {
		if n.Name == "SPI_CS" {
			net = n
		}
	}
	wantOutcome, wantWitness, wantCtx := check.PullUpVerdict(m, net)
	for _, v := range spiProfile().pullupRule().Eval(m) {
		if check.EntityRef(v.Subjects[0]) != "SPI_CS" {
			continue
		}
		if v.Outcome != wantOutcome {
			t.Errorf("outcome = %v, want %v", v.Outcome, wantOutcome)
		}
		if v.Witness.Statement != wantWitness.Statement {
			t.Errorf("witness = %q, want %q", v.Witness.Statement, wantWitness.Statement)
		}
		if len(v.Context) != len(wantCtx) {
			t.Errorf("context = %+v, want %+v", v.Context, wantCtx)
		}
		return
	}
	t.Fatal("SPI_CS produced no verdict")
}

// A FAILING net still carries its witness, which is what says the answer rests on the hop limit
// rather than on nothing.
func TestAProfileFailureStatesTheRadiusItSearchedTo(t *testing.T) {
	d := directPullUpDesign("SPI_CS")
	d.Nets[0].Connections = d.Nets[0].Connections[:1] // drop R1 from the CS net: no pull-up left
	for _, v := range spiProfile().pullupRule().Eval(check.NewModel(d)) {
		if check.EntityRef(v.Subjects[0]) != "SPI_CS" {
			continue
		}
		if v.Outcome != check.Fail || v.Finding == nil {
			t.Fatalf("outcome = %v with finding %v, want a failure", v.Outcome, v.Finding)
		}
		if v.Witness == nil || !strings.Contains(v.Witness.Statement, "no rail is reachable") {
			t.Errorf("witness = %+v, want it to state what it searched and how far", v.Witness)
		}
		return
	}
	t.Fatal("SPI_CS produced no verdict")
}

// THE COVERAGE BUG, as a regression test. The panel implemented only the reaches clause and claimed
// in a comment it matched the rule, so a direct pull-up onto a wide rail scored pullup_missing in the
// panel while the rule said nothing. A correct board read as defective in the one surface a reviewer
// looks at before the findings.
func TestCoverageAgreesWithTheRuleOnADirectPullUp(t *testing.T) {
	m := check.NewModel(directPullUpDesign("SPI_CS"))
	cov := Coverage(spiProfile(), m)
	if cov == nil {
		t.Fatal("the interface was not detected, so this proves nothing")
	}
	for _, s := range cov.Signals {
		if s.Name != "CS" {
			continue
		}
		if s.State != StatePresent {
			t.Errorf("CS scored %q, want %q; R1 pulls it to +3V3 and the rule agrees", s.State, StatePresent)
		}
		return
	}
	t.Fatal("CS is absent from the coverage matrix")
}

// The gate the datalog form conjoined has to survive the move, or a profile whose signals are not on
// this board reports a page of failures about an interface that is not there.
func TestNoVerdictsWhenTheInterfaceIsNotInUse(t *testing.T) {
	d := directPullUpDesign("UNRELATED_NET")
	d.Nets[1].Name = "OTHER" // and drop the second matching signal, so InUse is false
	if got := spiProfile().pullupRule().Eval(check.NewModel(d)); len(got) != 0 {
		t.Errorf("verdicts = %+v, want none on a board this profile is not on", got)
	}
}
