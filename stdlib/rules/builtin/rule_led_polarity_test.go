package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// ledFixture wires three LEDs and a zener-shaped diode: LED1 reversed (anode on GND,
// cathode on a rail) fires; LED2 forward stays silent; LED3 with unnamed pins stays
// silent (RoleUnknown, never guess); D1 reversed is a diode, not an LED — silent.
func ledFixture() *ir.Design {
	led := &ir.PartType{Name: "LED", Pins: []*ir.Pin{
		{Designator: "1", Name: "A", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
		{Designator: "2", Name: "K", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
	}}
	bare := &ir.PartType{Name: "LEDBARE", Pins: []*ir.Pin{
		{Designator: "1", Name: "~", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
		{Designator: "2", Name: "~", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
	}}
	zener := &ir.PartType{Name: "ZENER", Pins: []*ir.Pin{ // no "led" token: D prefix stays diode
		{Designator: "1", Name: "A", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
		{Designator: "2", Name: "K", Direction: ir.PinDirection_PIN_DIRECTION_PASSIVE},
	}}
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"},
			Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	d := &ir.Design{
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{led, bare, zener}}},
		Components: []*ir.Component{
			comp("LED1", "LED"), comp("LED2", "LED"), comp("LED3", "LEDBARE"), comp("D1", "ZENER"),
		},
		Nets: []*ir.Net{
			tnet("GND", "LED1.1", "LED2.2", "LED3.1", "D1.1"),
			tnet("+3V3", "LED1.2", "LED2.1", "LED3.2", "D1.2"),
		},
	}
	return d
}

func TestLedPolarity(t *testing.T) {
	m := check.NewModel(ledFixture())
	fs := ledPolarity.Eval(m)
	if len(fs) != 1 || fs[0].Subject != "LED1" || fs[0].Kind != check.KindComponent {
		t.Fatalf("findings = %+v, want exactly LED1", fs)
	}
}

// TestPinNetConflict: the tripwire fires on a genuinely multi-claimed pin, and stays
// silent when the multi-claim is the mechanical symptom of a ref-des collision (that
// root cause belongs to duplicate-ref-des — learned from the sheetnav fixture).
func TestPinNetConflict(t *testing.T) {
	d := &ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "t"}}},
		Nets: []*ir.Net{
			tnet("NET_A", "U1.3"),
			tnet("NET_B", "U1.3", "U1.4"),
		},
	}
	fs := pinNetConflict.Eval(check.NewModel(d))
	if len(fs) != 1 || fs[0].Subject != "U1" || fs[0].Pin != "3" || fs[0].Kind != check.KindPin {
		t.Fatalf("findings = %+v, want one KindPin finding on U1 pin 3", fs)
	}
	if fs[0].Message != "pin appears in more than one net (NET_A, NET_B); a pin belongs to exactly one" {
		t.Errorf("message = %q", fs[0].Message)
	}

	d.InputDiagnostics = &ir.InputDiagnostics{RefDesCollisions: []*ir.RefDesCollision{{RefDes: "U1"}}}
	if fs := pinNetConflict.Eval(check.NewModel(d)); len(fs) != 0 {
		t.Errorf("collided ref-des still fired pin-net-conflict: %+v", fs)
	}
}

func TestPinRoleAndNetOnModel(t *testing.T) {
	m := check.NewModel(ledFixture())
	if r := m.PinRole("LED1", "1"); r != check.RoleAnode {
		t.Errorf("LED1.1 role = %s, want anode", r)
	}
	if r := m.PinRole("D1", "2"); r != check.RoleCathode {
		t.Errorf("D1.2 role = %s, want cathode (diode family)", r)
	}
	if n := m.PinNetName("LED1", "2"); n != "+3V3" {
		t.Errorf("LED1.2 net = %q, want +3V3", n)
	}
	if n := m.PinNetName("LED1", "9"); n != "" {
		t.Errorf("unknown pin net = %q, want empty", n)
	}
}
