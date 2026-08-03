package check

import (
	"fmt"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// benchDesign synthesizes a netlist big enough to time the two evaluation paths: nNets nets
// of 4 typed connections over nNets/2 components, with classes, guard attributes, and
// naming patterns spread deterministically so every rule does real work.
func benchDesign(nNets int) *ir.Design {
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "MCU", Pins: []*ir.Pin{
				{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Designator: "2", Direction: ir.PinDirection_PIN_DIRECTION_INPUT},
				{Designator: "3", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT},
				{Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_INOUT},
			}},
		}}},
	}
	nComps := nNets / 2
	prefixes := []string{"U", "R", "C", "J", "TVS", "F"}
	for i := range nComps {
		c := &ir.Component{RefDes: fmt.Sprintf("%s%d", prefixes[i%len(prefixes)], i)}
		if i%len(prefixes) == 0 {
			c.Sections = []*ir.ComponentSection{{PartRef: "MCU", LibraryRef: "lib"}}
		}
		d.Components = append(d.Components, c)
	}
	names := []string{"NET%d", "SDA%d", "VCC%d", "DATA%d_P", "GND%d"}
	for i := range nNets {
		n := &ir.Net{Name: fmt.Sprintf(names[i%len(names)], i)}
		if i%7 == 0 {
			n.Attributes = map[string]string{"power_driven": "true"}
		}
		for p := range 4 {
			ref := d.Components[(i*4+p)%nComps].RefDes
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: fmt.Sprint(p + 1)})
		}
		d.Nets = append(d.Nets, n)
	}
	return d
}

// BenchmarkRulesGo and BenchmarkRulesSpec time the full built-in catalog over the same
// Model through each evaluation path. The delta is the interpreter's overhead, and the
// absolute numbers are the evidence for whether an indexed fact base (WS3-004) is worth
// building; keep them in sync with the table in the WS3-003 PR.
func BenchmarkRulesGo(b *testing.B) {
	m := NewModel(benchDesign(2000))
	b.ResetTimer()
	for b.Loop() {
		for _, r := range Rules {
			r.Eval(m)
		}
	}
}

func BenchmarkRulesSpec(b *testing.B) {
	m := NewModel(benchDesign(2000))
	b.ResetTimer()
	for b.Loop() {
		for _, r := range Rules {
			if s, ok := Specs[r.Name]; ok {
				s.Eval(m)
			} else {
				r.Eval(m) // spec-only rule: its Eval already is the interpreter
			}
		}
	}
}
