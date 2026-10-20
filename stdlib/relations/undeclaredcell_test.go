package relations

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/readers/formats"
)

// undeclaredCellDesign places R1 and R2 from a cell RES no library declares, and U1 from a part type
// that declares pin 1 alone while the netlist also connects its pin 2, so the two cases a pin read
// off a connection could cover sit side by side.
func undeclaredCellDesign() *ir.Design {
	return &ir.Design{
		SourceFormat: "edif",
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			{Name: "BUF", Pins: []*ir.Pin{{Designator: "1", Direction: ir.PinDirection_PIN_DIRECTION_OUTPUT}}},
		}}},
		Components: []*ir.Component{
			{RefDes: "R1", Prov: &ir.Provenance{SourceFile: "t"}, Sections: []*ir.ComponentSection{{PartRef: "RES", LibraryRef: "lib"}}},
			{RefDes: "R2", Prov: &ir.Provenance{SourceFile: "t"}, Sections: []*ir.ComponentSection{{PartRef: "RES", LibraryRef: "lib"}}},
			{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}, Sections: []*ir.ComponentSection{{PartRef: "BUF", LibraryRef: "lib"}}},
		},
		Nets: []*ir.Net{tnet("SIGA", "R1.1", "R2.1", "U1.1"), tnet("SIGB", "R1.2", "R2.2", "U1.2")},
	}
}

// agni issue 975: a part from a cell the file never declares has the pins its connections name, so
// pin.net answers the same connections net.pin_count counts. Its pins carry no direction, since no
// library gave one, so pin.type says nothing rather than "unspecified".
func TestAPartFromAnUndeclaredCellHasThePinsItsConnectionsName(t *testing.T) {
	m := check.NewModel(undeclaredCellDesign())
	byRel := factsByRelation(Facts(m))
	on := map[string]string{}
	for _, r := range byRel[RelPinNet] {
		on[r.Subject+"."+r.Object] = r.Value
	}
	for pin, net := range map[string]string{"R1.1": "SIGA", "R2.1": "SIGA", "R1.2": "SIGB", "R2.2": "SIGB", "U1.1": "SIGA"} {
		if on[pin] != net {
			t.Errorf("pin.net %s = %q, want %q", pin, on[pin], net)
		}
	}
	for _, r := range byRel[RelPinType] {
		if r.Subject == "R1" || r.Subject == "R2" {
			t.Errorf("pin.type %s.%s = %q, want no row for a pin no library declares", r.Subject, r.Object, r.Value)
		}
	}
	if m.PinDeclared("R1", "1") {
		t.Error("R1.1 reads as declared, so a rule needing its direction would take UNSPECIFIED as the library's word")
	}
	// A declared part type that omits a connected pin is a different defect, which the cross-view rules
	// report, so it gains no pin here.
	if net, ok := on["U1.2"]; ok {
		t.Errorf("pin.net U1.2 = %q, want no row: BUF declares pin 1 alone", net)
	}
}

// The fixture the issue names, read through the Loader as the viewer reads it: pin.net and
// net.pin_count agree on every net.
func TestPinNetAgreesWithNetPinCountOnTheCompanionDemo(t *testing.T) {
	d, err := (&formats.Loader{}).ReadDesign("../../cmd/agni/testdata/review/companion-demo.edn")
	if err != nil {
		t.Fatal(err)
	}
	byRel := factsByRelation(Facts(check.NewModel(d)))
	pins := map[string]int{}
	for _, r := range byRel[RelPinNet] {
		pins[r.Value]++
	}
	counts := byRel[RelNetPinCount]
	if len(counts) == 0 {
		t.Fatal("the fixture projects no net.pin_count, so agreement would prove nothing")
	}
	for _, r := range counts {
		if r.Num == nil || int(*r.Num) != pins[r.Subject] {
			t.Errorf("net %s: net.pin_count %v, pin.net rows %d", r.Subject, r.Num, pins[r.Subject])
		}
	}
}
