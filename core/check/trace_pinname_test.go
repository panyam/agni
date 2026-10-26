package check

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// pinNameModel is traceFixture with U7 given more pins: two it names GND, and one named
// GMAC0_MII_RGMII_TXD2 that a spelling of GMAC0_TXD2 matches only by dropping tokens.
func pinNameModel() Model {
	d := traceFixture()
	d.Libraries[0].Parts[0].Pins = append(d.Libraries[0].Parts[0].Pins,
		&ir.Pin{Designator: "20", Name: "GND"}, &ir.Pin{Designator: "21", Name: "GND"},
		&ir.Pin{Designator: "22", Name: "GMAC0_MII_RGMII_TXD2"})
	d.Nets = append(d.Nets, tnet("GND", "U7.20", "U7.21"), tnet("TX", "U7.22"))
	return NewModel(d)
}

// A pin spelled by its functional name is the pin it names, so `U7.SDA` traces exactly as `U7.3`
// does (agni issue 1193), and the answer names the pin by its designator.
func TestTraceResolvesAPinByItsName(t *testing.T) {
	m := pinNameModel()
	byNum := TracePins(m, Endpoint{"U7", "3"}, Endpoint{"U12", "4"}, DefaultTraceHops)
	for _, spelled := range []string{"SDA", "sda"} {
		byName := TracePins(m, Endpoint{"U7", spelled}, Endpoint{"U12", "SDIO"}, DefaultTraceHops)
		if byName.Outcome != TraceRouted || byName.From.Pin != "3" || byName.To.Pin != "4" {
			t.Fatalf("%s: outcome %s (%q), from %v, to %v", spelled, byName.Outcome, byName.Reason, byName.From, byName.To)
		}
		if len(byName.Crossings) != len(byNum.Crossings) || byName.Crossings[0].RefDes != byNum.Crossings[0].RefDes {
			t.Errorf("%s: route differs from the designator's: %v vs %v", spelled, byName.Crossings, byNum.Crossings)
		}
	}
	byName := TracePins(m, Endpoint{"U7", "SDA"}, Endpoint{"U12", "SDIO"}, DefaultTraceHops)
	for _, stub := range byName.Nets[0].Stubs {
		if stub.RefDes == "U7" {
			t.Errorf("the start pin, named by its name, is listed as a stub on its own net: %v", stub)
		}
	}
}

// A name several pins carry, and a name a pin matches only loosely, are refused with the candidates,
// since a route from a guessed pin is an answer about the wrong node.
func TestTraceRefusesAnAmbiguousOrLoosePinName(t *testing.T) {
	m := pinNameModel()
	for _, tc := range []struct{ pin, want string }{
		{"GND", "U7.GND names 2 pins of U7 (20, 21)"},
		{"GMAC0_TXD2", "matches U7.22 only loosely"},
		{"NOPE", "U7 declares no pin NOPE"},
	} {
		tr := TracePins(m, Endpoint{"U7", tc.pin}, Endpoint{"U12", "4"}, DefaultTraceHops)
		if tr.Outcome != TraceUnresolved || !strings.Contains(tr.Reason, tc.want) {
			t.Errorf("%s: outcome %s, reason %q; want unresolved naming %q", tc.pin, tr.Outcome, tr.Reason, tc.want)
		}
	}
}
