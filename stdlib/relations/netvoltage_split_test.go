package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// voltageNameDesign places two nets whose names BOTH parse a voltage token, one a rail and one an
// ordinary signal between two pins. The signal name is the shape agni issue 194 verified: a house
// convention encoding endpoints and a level, which token-scans to 3.3 while carrying no rail role.
func voltageNameDesign() *ir.Design {
	return &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{{
			Name: "IC",
			Pins: []*ir.Pin{
				{Name: "VDD", Designator: "12", Direction: ir.PinDirection_PIN_DIRECTION_POWER_IN},
				{Name: "IO", Designator: "4", Direction: ir.PinDirection_PIN_DIRECTION_INOUT},
			},
		}}}},
		Components: []*ir.Component{
			{RefDes: "U3", Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
			{RefDes: "U7", Sections: []*ir.ComponentSection{{PartRef: "IC", LibraryRef: "lib"}}, Prov: &ir.Provenance{SourceFile: "t"}},
		},
		Nets: []*ir.Net{
			{Name: "+3V3", Connections: []*ir.Connection{
				{ComponentRef: "U3", PinRef: "12"}, {ComponentRef: "U7", PinRef: "12"}},
				Prov: &ir.Provenance{SourceFile: "t"}},
			{Name: "U3_12_U7_4_3V3", Connections: []*ir.Connection{
				{ComponentRef: "U3", PinRef: "4"}, {ComponentRef: "U7", PinRef: "4"}},
				Prov: &ir.Provenance{SourceFile: "t"}},
		},
	}
}

// The issue-194 case: a signal net whose name token-scans to a voltage must NOT appear in a relation
// documented as a rail nominal. Before the split it did, and the number was right while the relation
// carrying it was not.
func TestSignalNetLevelIsNotARailNominal(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(voltageNameDesign())))

	for _, f := range byRel[RelNetNominalVoltage] {
		if f.Subject == "U3_12_U7_4_3V3" {
			t.Errorf("a signal net must not project a rail nominal, got %+v", f)
		}
	}
	found := false
	for _, f := range byRel[RelNetSignalLevel] {
		if f.Subject == "U3_12_U7_4_3V3" {
			found = true
			if f.Num == nil || *f.Num != 3.3 {
				t.Errorf("net.signal_level num = %v, want 3.3", f.Num)
			}
		}
	}
	if !found {
		t.Errorf("the level must survive the split, not be dropped: %+v", byRel[RelNetSignalLevel])
	}
}

// The other half: gating must not cost the relation its actual subject. A named rail still projects
// a nominal, and does not leak into the signal relation.
func TestRailStillProjectsANominalAndNotALevel(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(voltageNameDesign())))

	found := false
	for _, f := range byRel[RelNetNominalVoltage] {
		if f.Subject == "+3V3" {
			found = true
			if f.Num == nil || *f.Num != 3.3 {
				t.Errorf("net.nominal_voltage(+3V3) num = %v, want 3.3", f.Num)
			}
		}
	}
	if !found {
		t.Errorf("a rail must still project its nominal: %+v", byRel[RelNetNominalVoltage])
	}
	for _, f := range byRel[RelNetSignalLevel] {
		if f.Subject == "+3V3" {
			t.Errorf("a rail must not also project a signal level, got %+v", f)
		}
	}
}

// The two relations partition the nets whose names parse: every such net lands in exactly one. This
// is what lets a consumer that wants both ask for both without deduping, and a consumer that wants
// rails be sure it cannot receive a level.
func TestNominalAndLevelArePartitioned(t *testing.T) {
	byRel := factsByRelation(Facts(check.NewModel(voltageNameDesign())))

	seen := map[string]int{}
	for _, f := range byRel[RelNetNominalVoltage] {
		seen[f.Subject]++
	}
	for _, f := range byRel[RelNetSignalLevel] {
		seen[f.Subject]++
	}
	for _, n := range voltageNameDesign().Nets {
		if got := seen[n.Name]; got != 1 {
			t.Errorf("net %q appears in %d of the two relations, want exactly 1", n.Name, got)
		}
	}
}
