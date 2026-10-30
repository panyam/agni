package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// TestAPullUpThroughAZeroOhmLinkToAnInferredRailPasses is MNT's INA bus in miniature (agni 1242,
// 1243). The controller's SCL reaches INA_SCL through a 0 Ohm link, and INA_SCL is pulled up by R69 to
// LPC_VCC, a rail only because it feeds the MCU's VDD. Both I2C nets pass once the rail is inferred.
func TestAPullUpThroughAZeroOhmLinkToAnInferredRailPasses(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"U18": {{"VDD", "1", pwrIn}, {"SCL", "2", ir.PinDirection_PIN_DIRECTION_INOUT}},
		"U24": {{"I2CT_SCL", "9", pass}},
		"R69": twoPin(), "R83": twoPin(),
	}, map[string][]string{
		"LPC_VCC":            {"U18.1", "R69.1"},
		"INA_SCL":            {"U18.2", "R69.2", "R83.1"},
		"Net-(U24-I2CT_SCL)": {"R83.2", "U24.9"},
	})
	l := classify.DefaultLexicon()
	l.Stamp(d)
	l.StampNetRoles(d)
	for _, net := range []string{"INA_SCL", "Net-(U24-I2CT_SCL)"} {
		if got := verdictOn(t, i2cPullUp, d, net); got != string(check.Pass) {
			t.Errorf("i2c-pull-up on %s = %q, want pass through R69 to LPC_VCC", net, got)
		}
	}
}

// TestBulkCapJudgesRailsNotGlobalLabels holds bulk-cap's subjects (agni 1252). A signal drawn with a
// global label gets no verdict, while a net carrying a power symbol, and a global net that reads as a
// rail, are still judged.
func TestBulkCapJudgesRailsNotGlobalLabels(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"U1": {{"DA0", "1", ir.PinDirection_PIN_DIRECTION_OUTPUT}, {"VDD", "2", pwrIn}, {"VDDIO", "3", pwrIn}},
		"U2": {{"D0", "1", ir.PinDirection_PIN_DIRECTION_INPUT}},
	}, map[string][]string{
		"DA0":     {"U1.1", "U2.1"},
		"+3V3":    {"U1.2"},
		"VCC_AUX": {"U1.3"},
	})
	global := map[string]string{netgraph.AttrGlobal: "true"}
	for _, n := range d.Nets {
		switch n.Name {
		case "DA0", "VCC_AUX":
			n.Attributes = global
		case "+3V3":
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: "#PWR01", PinRef: "1",
				Attributes: map[string]string{"direction": "power_in"}})
		}
	}
	l := classify.DefaultLexicon()
	l.Stamp(d)
	l.StampNetRoles(d)
	if got := verdictOn(t, bulkCap, d, "DA0"); got != "" {
		t.Errorf("bulk-cap on a global-label signal = %q, want no verdict", got)
	}
	for _, net := range []string{"+3V3", "VCC_AUX"} {
		if got := verdictOn(t, bulkCap, d, net); got != string(check.Fail) {
			t.Errorf("bulk-cap on %s = %q, want fail (a rail with no capacitor)", net, got)
		}
	}
}
