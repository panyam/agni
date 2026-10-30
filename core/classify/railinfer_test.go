package classify

import (
	"strings"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

// supplyFedDesign is MNT's LPC_VCC shape in miniature: an MCU's VDD and a capacitor on LPC_VCC, an
// enable signal EN_VCC on an MCU input, a net reaching only a connector's power_in pin, an unfitted
// part's VDD, and a second MCU's unwired VDD alone on its stub.
func supplyFedDesign() *ir.Design {
	pin := func(name, des string, dir ir.PinDirection) *ir.Pin {
		return &ir.Pin{Name: name, Designator: des, Direction: dir}
	}
	part := func(name string, pins ...*ir.Pin) *ir.PartType { return &ir.PartType{Name: name, Pins: pins} }
	comp := func(ref, part string) *ir.Component {
		return &ir.Component{RefDes: ref, Sections: []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}}
	}
	net := func(name string, conns ...string) *ir.Net {
		n := &ir.Net{Name: name}
		for _, c := range conns {
			ref, p, _ := strings.Cut(c, ".")
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: ref, PinRef: p})
		}
		return n
	}
	in, pwr := ir.PinDirection_PIN_DIRECTION_INPUT, ir.PinDirection_PIN_DIRECTION_POWER_IN
	dnp := comp("U3", "MCU")
	dnp.Attributes = map[string]string{"dnp": "yes"}
	return &ir.Design{
		SourceFormat: "kicad-sch",
		Libraries: []*ir.PartLibrary{{Name: "lib", Parts: []*ir.PartType{
			part("MCU", pin("VDD", "1", pwr), pin("EN", "2", in)),
			part("USB", pin("VBUS", "A4", pwr)),
			part("CAP", pin("1", "1", ir.PinDirection_PIN_DIRECTION_PASSIVE), pin("2", "2", ir.PinDirection_PIN_DIRECTION_PASSIVE)),
		}}},
		Components: []*ir.Component{comp("U1", "MCU"), comp("U4", "MCU"), comp("J1", "USB"), comp("C1", "CAP"), comp("C2", "CAP"), comp("C3", "CAP"), dnp},
		Nets: []*ir.Net{
			net("LPC_VCC", "U1.1", "C1.1"),
			net("EN_VCC", "U1.2", "C2.1"),
			net("CONN_VBUS", "J1.A4", "C3.1"),
			net("SPARE_VDD", "U3.1", "C3.2"),
			net("unconnected-(U4-VPP_VDD)", "U4.1"),
			net("VCC_IO", "U1.1"),
		},
	}
}

func roleOf(d *ir.Design, name string, role ir.Role) *ir.NetRole {
	for _, n := range d.GetNets() {
		if n.GetName() == name {
			for _, r := range n.GetRoles() {
				if r.GetRoleKind() == role {
					return r
				}
			}
		}
	}
	return nil
}

// TestARailIsInferredFromTheSupplyPinItFeeds holds both halves of the inference (agni 1243). LPC_VCC
// becomes a rail and names U1's VDD as its reason. EN_VCC feeds no supply pin, CONN_VBUS feeds only a
// connector, SPARE_VDD feeds only an unfitted part, and U4's unwired VDD is alone on its stub, so none
// of them is a rail.
func TestARailIsInferredFromTheSupplyPinItFeeds(t *testing.T) {
	d := supplyFedDesign()
	l := DefaultLexicon()
	l.Stamp(d)
	l.StampNetRoles(d)
	r := roleOf(d, "LPC_VCC", ir.Role_ROLE_RAIL)
	if r == nil {
		t.Fatal("LPC_VCC feeds U1's VDD and was not read as a rail")
	}
	if want := "the name's token VCC is a supply word and the net feeds U1 pin 1 (VDD), a supply input"; r.GetReason() != want {
		t.Errorf("LPC_VCC reason = %q, want %q", r.GetReason(), want)
	}
	for _, name := range []string{"EN_VCC", "CONN_VBUS", "SPARE_VDD", "unconnected-(U4-VPP_VDD)"} {
		if roleOf(d, name, ir.Role_ROLE_RAIL) != nil {
			t.Errorf("%s read as a rail", name)
		}
	}
	// A prefix match is the rail vocabulary's, and says which pattern it was.
	if r := roleOf(d, "VCC_IO", ir.Role_ROLE_RAIL); r == nil || !strings.HasPrefix(r.GetReason(), "the name matches the rail pattern ^(VCC|") {
		t.Errorf("VCC_IO rail role = %v, want one naming the rail pattern it matched", r)
	}
}

// TestRailInferenceCanBeTurnedOff checks that a convention setting infer_rail_from_supply_pins false
// leaves LPC_VCC without the rail role, and that leaving it unset keeps the built-in setting.
func TestRailInferenceCanBeTurnedOff(t *testing.T) {
	for _, c := range []struct {
		name string
		over *configpb.NamingLexicon
		want bool
	}{
		{"unset", &configpb.NamingLexicon{Net: &configpb.NetNameVocab{}}, true},
		{"off", &configpb.NamingLexicon{Net: &configpb.NetNameVocab{InferRailFromSupplyPins: proto.Bool(false)}}, false},
	} {
		v, err := BuildRoleVocab(c.over)
		if err != nil {
			t.Fatal(err)
		}
		d := supplyFedDesign()
		l := &Lexicon{Role: v}
		l.Stamp(d)
		l.StampNetRoles(d)
		if got := roleOf(d, "LPC_VCC", ir.Role_ROLE_RAIL) != nil; got != c.want {
			t.Errorf("%s: LPC_VCC rail = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAnExclusionTakesANameOutOfAVocabulary checks the exclude list and its merge: a project's
// exclusion joins the built-in one, and replace drops both the built-in patterns and exclusions.
func TestAnExclusionTakesANameOutOfAVocabulary(t *testing.T) {
	added, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{
		I2C: &configpb.VocabPatterns{Exclude: []string{`^DDC_`}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"SDA": true, "I2S0_RX_SDA": false, "DDC_SDA": false} {
		if got := added.IsI2C(name); got != want {
			t.Errorf("added exclusion: IsI2C(%q) = %v, want %v", name, got, want)
		}
	}
	replaced, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{
		I2C: &configpb.VocabPatterns{Patterns: []string{`_TWI_`}, Replace: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"SDA": false, "U1_TWI_D": true} {
		if got := replaced.IsI2C(name); got != want {
			t.Errorf("replaced: IsI2C(%q) = %v, want %v", name, got, want)
		}
	}
	if _, err := BuildRoleVocab(&configpb.NamingLexicon{Net: &configpb.NetNameVocab{
		I2C: &configpb.VocabPatterns{Exclude: []string{`(`}},
	}}); err == nil {
		t.Error("a bad exclusion pattern compiled; it must be a load error")
	}
}

// TestPWRIsARailOnlyAsAWholeToken checks the rail vocabulary's PWR prefix ends its token, so a
// configuration pin like PWRCTL_POL is not a power name (agni 1245).
func TestPWRIsARailOnlyAsAWholeToken(t *testing.T) {
	v := DefaultRoleVocab()
	for name, want := range map[string]bool{"PWR": true, "PWR_3V3": true, "PWR5V": true, "PWRCTL_POL": false, "PWRGOOD": false} {
		if got := v.IsRail(name); got != want {
			t.Errorf("IsRail(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestAPartNumberFamilyDecidesTheClass checks the built-in VBUS54 family (agni 1257) and a project's
// added family, each on a part whose ref-des reads as an IC.
func TestAPartNumberFamilyDecidesTheClass(t *testing.T) {
	ic := func(mpn string) (*ir.Component, *ir.PartType) {
		return &ir.Component{RefDes: "U15", Attributes: map[string]string{"MPN": mpn}}, &ir.PartType{Name: "X"}
	}
	c, pt := ic("VBUS54CV-HSF-G4-08")
	if got := DefaultLexicon().Classify(c, pt); got != ClassTVS {
		t.Errorf("VBUS54CV-HSF-G4-08 classified %q, want tvs", got)
	}
	cv, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassTVS: {Families: []string{`^acme`}}})
	if err != nil {
		t.Fatal(err)
	}
	c, pt = ic("ACME4E")
	if got := (&Lexicon{Class: cv}).Classify(c, pt); got != ClassTVS {
		t.Errorf("a project family: ACME4E classified %q, want tvs", got)
	}
	c, pt = ic("PESD5V0")
	if got := (&Lexicon{Class: cv}).Classify(c, pt); got != ClassTVS {
		t.Errorf("a project family must add to the built-ins: PESD5V0 classified %q", got)
	}
}
