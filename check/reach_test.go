package check

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// reachFixture: J1 -> F1(fuse) -> VIN2 -> FB1(ferrite) -> VIN3 -> U1(power_in), with a
// TVS on VIN2, a pull-up R2 onto the VCC rail (the walk must NOT cross into a rail), a
// resistor loop R3/R4 (cycle guard), and a 4-pin "resistor" RN1 spanning three nets (the
// two-terminal guard must refuse to cross it).
func reachFixture() *ir.Design {
	lib := &ir.PartLibrary{Name: "lib", Parts: []*ir.PartType{
		{Name: "REG", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN}}},
	}}
	comp := func(ref string) *ir.Component {
		c := &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
		if ref == "U1" {
			c.Sections = []*ir.ComponentSection{{PartRef: "REG", LibraryRef: "lib"}}
		}
		return c
	}
	vcc := tnet("VCC", "R2.2")
	vcc.Attributes = map[string]string{"global": "true"}
	driven := tnet("VIN2", "F1.2", "FB1.1", "D1.1")
	driven.Attributes = map[string]string{"power_driven": "true"}
	return &ir.Design{
		Libraries: []*ir.PartLibrary{lib},
		Components: []*ir.Component{
			comp("J1"), comp("F1"), comp("FB1"), comp("U1"), comp("D1"),
			comp("R2"), comp("R3"), comp("R4"), comp("RN1"), comp("C1"),
		},
		Nets: []*ir.Net{
			tnet("VIN1", "J1.1", "F1.1", "R2.1", "R3.1", "R4.1", "RN1.1"),
			driven,
			tnet("VIN3", "FB1.2", "U1.1", "C1.1"),
			vcc,
			tnet("LOOP", "R3.2", "R4.2"),
			tnet("RN_A", "RN1.2"),
			tnet("RN_B", "RN1.3", "RN1.4"),
		},
	}
}

func TestReachWalk(t *testing.T) {
	m := NewModel(reachFixture())
	start := m.Nets()[0] // VIN1
	r := m.Reach(start, 3)

	names := map[string]bool{}
	for _, n := range r.Nets {
		names[n.Name] = true
	}
	for _, want := range []string{"VIN1", "VIN2", "VIN3", "LOOP"} {
		if !names[want] {
			t.Errorf("reach missing %s (have %v)", want, names)
		}
	}
	if names["VCC"] {
		t.Error("reach crossed R2 into the VCC rail; rails are stops")
	}
	if !names["VIN2"] {
		t.Error("power_driven is a PWR_FLAG on the power ENTRY path, not bus evidence; the walk must cross into VIN2")
	}
	if names["RN_A"] || names["RN_B"] {
		t.Error("reach crossed the 4-pin RN1; only two-net pass elements cross")
	}
	if len(r.Nets) != 4 {
		t.Errorf("reached %d nets %v, want exactly 4", len(r.Nets), names)
	}
	if !r.Crossed["F1"] || !r.Crossed["FB1"] {
		t.Errorf("crossed = %v, want F1 and FB1", r.Crossed)
	}
	if r.Crossed["R2"] {
		t.Error("R2 leads only to a rail and must not be a crossed path element")
	}

	if h1 := m.Reach(start, 1); len(h1.Nets) != 3 { // VIN1 + VIN2 + LOOP
		var got []string
		for _, n := range h1.Nets {
			got = append(got, n.Name)
		}
		t.Errorf("hops=1 reached %s, want VIN1 VIN2 LOOP", strings.Join(got, " "))
	}
}

func TestReachBetween(t *testing.T) {
	m := NewModel(reachFixture())
	nets := map[string]*ir.Net{}
	for _, n := range m.Nets() {
		nets[n.Name] = n
	}
	if !m.Between(nets["VIN1"], nets["VIN3"], ClassFuse, 3) {
		t.Error("F1 sits on the VIN1->VIN3 path")
	}
	if m.Between(nets["VIN1"], nets["LOOP"], ClassFuse, 3) {
		t.Error("no fuse on the VIN1->LOOP path (R3/R4 only)")
	}
	if m.Between(nets["VIN1"], nets["VCC"], ClassResistor, 3) {
		t.Error("VCC is unreachable (rail stop); Between must be false, not panic")
	}
}
