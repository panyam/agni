package builtin

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/netgraph"
)

// linkedRailDesign is a named rail +12V feeding U1's supply pin, joined through R1 to the converter's
// output node 12V_OUT where C1 sits. The rail itself carries no capacitor. value is R1's Value text,
// and dnp marks R1 do-not-populate.
func linkedRailDesign(value string, dnp bool) *ir.Design {
	d := pinDesign(map[string][]supplyPin{
		"U1": {{"VDD", "1", pwrIn}, {"GND", "2", pwrIn}}, "R1": twoPin(), "C1": twoPin(),
	}, map[string][]string{
		"+12V":    {"U1.1", "R1.1"},
		"12V_OUT": {"R1.2", "C1.1"},
		"GND":     {"U1.2", "C1.2"},
	})
	for _, n := range d.Nets {
		if n.Name == "+12V" {
			n.Attributes = map[string]string{netgraph.AttrGlobal: "true"}
		}
	}
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			c.Attributes = map[string]string{"Value": value}
			if dnp {
				c.Attributes["dnp"] = "yes"
			}
		}
	}
	classify.Stamp(d)
	classify.StampValues(d)
	return d
}

// A 0 Ohm link joins two names for one rail, so the capacitor on the converter's side of it is on the
// rail (agni issue 1015). Jetson's +12V reaches its four 22 uF capacitors through R22, whose Value is
// the library name R_0R_22.4A_0603, which parses to no number at all.
func TestACapacitorAcrossAZeroOhmLinkIsOnTheRail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		dnp   bool
		want  check.Outcome
	}{
		{"a 0R link", "0R", false, check.Pass},
		{"a link named by its library", "R_0R_22.4A_0603", false, check.Pass},
		{"a bare zero", "0", false, check.Pass},
		{"a series resistor", "10k", false, check.Fail},
		{"a library name with no zero-ohm token", "R_10k_0402", false, check.Fail},
		{"an unfitted link", "0R", true, check.Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := linkedRailDesign(tc.value, tc.dnp)
			for _, r := range []*check.Rule{bulkCap, decouplingPresent} {
				if got := verdictOn(t, r, d, "+12V"); got != string(tc.want) {
					t.Errorf("%s on +12V = %q, want %q", r.Name, got, tc.want)
				}
			}
			specFires := len(bulkCapSpec.Eval(context.Background(), check.NewModel(d))) > 0
			if specFires != (tc.want == check.Fail) {
				t.Errorf("bulk-cap's spec twin fires = %v, want %v", specFires, tc.want == check.Fail)
			}
		})
	}
}

// The pass names the link, so a reviewer looking at +12V on the schematic is told the capacitor is
// across R1 rather than on the rail they are looking at.
func TestARailCapacitorAcrossALinkNamesTheLink(t *testing.T) {
	for _, v := range bulkCap.Eval(context.Background(), check.NewModel(linkedRailDesign("0R", false))) {
		if v.Subjects[0].Ref != "+12V" {
			continue
		}
		want := "capacitor C1 sits on 12V_OUT, which zero-ohm link R1 joins to the rail"
		if v.Witness == nil || v.Witness.Statement != want {
			t.Fatalf("witness = %+v, want %q", v.Witness, want)
		}
		if len(v.Context) != 2 || v.Context[1].Entity.Ref != "R1" {
			t.Errorf("context = %+v, want C1 and the link R1", v.Context)
		}
		return
	}
	t.Fatal("bulk-cap gave no verdict on +12V")
}

// A 0 Ohm link to ground ties a return to a chassis net and does not make the ground part of the
// rail, so a capacitor reached only through ground does not count.
func TestAZeroOhmLinkNeverJoinsAGround(t *testing.T) {
	d := pinDesign(map[string][]supplyPin{
		"U1": {{"VDD", "1", pwrIn}}, "R1": twoPin(), "C1": twoPin(),
	}, map[string][]string{
		"+5V": {"U1.1", "R1.1"},
		"GND": {"R1.2", "C1.1"},
		"X":   {"C1.2"},
	})
	for _, c := range d.Components {
		if c.RefDes == "R1" {
			c.Attributes = map[string]string{"Value": "0R"}
		}
	}
	classify.Stamp(d)
	classify.StampValues(d)
	for _, n := range d.Nets {
		if n.Name != "+5V" {
			continue
		}
		if got := check.JoinedRail(check.NewModel(d), n); len(got) != 1 {
			t.Errorf("JoinedRail(+5V) = %d nets, want only +5V itself", len(got))
		}
		return
	}
	t.Fatal("no +5V net")
}
