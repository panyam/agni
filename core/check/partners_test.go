package check

import (
	"reflect"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func partnersByPin(t *testing.T, m Model, ref string) map[string]PinPartners {
	t.Helper()
	got := map[string]PinPartners{}
	for _, p := range PartnersOf(m, ref, 0) {
		got[p.Pin] = p
	}
	return got
}

// agni issue 1238. SDA crosses R5 to the EEPROM, and on the far side R6 pulls up to VCC, which
// carries U10's supply pin. The partner is the EEPROM's pin alone: a test point and a capacitor sit
// on the signal rather than end it, and a load on the rail is not on the signal at all.
func TestPartnersOfFollowsTheSignalAndStopsAtTheRail(t *testing.T) {
	got := partnersByPin(t, traceModel(), "U7")
	if len(got) != 2 {
		t.Fatalf("U7 has %d pins listed, want its two (3 and 9): %+v", len(got), got)
	}
	sda := got["3"]
	if sda.Net != "SDA_RAW" || sda.Name != "SDA" || sda.Reason != "" {
		t.Errorf("pin 3 = %+v, want SDA on SDA_RAW, walked", sda)
	}
	if want := []Endpoint{{RefDes: "U12", Pin: "4"}}; !reflect.DeepEqual(sda.Partners, want) {
		t.Errorf("pin 3 partners = %v, want %v", sda.Partners, want)
	}
	if spare := got["9"]; spare.Reason != PartnerReasonNoNet || len(spare.Partners) != 0 {
		t.Errorf("pin 9 = %+v, want on no net and not walked", spare)
	}
}

// A signal reaching two devices names both, since which one the map should declare is a choice the
// netlist cannot make. Ground and a rail are not walked, and say so.
func TestPartnersOfListsEveryDeviceAndSkipsSupplies(t *testing.T) {
	comps := []*ir.Component{}
	for _, ref := range []string{"U1", "U2", "J1", "R1", "C1"} {
		comps = append(comps, &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}})
	}
	gnd := tnet("GND", "U1.4", "U2.2", "C1.2")
	m := NewModel(&ir.Design{Components: comps, Nets: []*ir.Net{
		tnet("CLK", "U1.1", "R1.1"),
		tnet("CLK_OUT", "R1.2", "J1.3", "U2.1", "C1.1"),
		tnet("+3V3", "U1.2", "U2.3"),
		gnd,
	}})
	got := partnersByPin(t, m, "U1")
	if want := []Endpoint{{RefDes: "J1", Pin: "3"}, {RefDes: "U2", Pin: "1"}}; !reflect.DeepEqual(got["1"].Partners, want) {
		t.Errorf("pin 1 partners = %v, want %v", got["1"].Partners, want)
	}
	if got["2"].Reason != PartnerReasonRail || got["4"].Reason != PartnerReasonGround {
		t.Errorf("pin 2 = %+v and pin 4 = %+v, want a rail and ground, neither walked", got["2"], got["4"])
	}
	if pins := PartnersOf(m, "U1", 0); pins[0].Pin != "1" || pins[len(pins)-1].Pin != "4" {
		t.Errorf("pins out of order: %+v", pins)
	}
}
