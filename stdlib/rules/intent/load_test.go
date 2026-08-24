package intent

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	d, err := Parse([]byte(`
name: Test sample intent
modules:
  - {name: SoC, class: soc}
  - {name: eMMC, mpn: MTFC4GACAJCN}
  - {name: CAN xcvr, class: can_transceiver, mpn: TCAN1042}
voltage_domains:
  - {name: io_3v3, nominal: 3.3, rails: [3V3, VDD_IO]}
  - {name: core, nominal: 0.8, rails: [VDD_CORE]}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Name != "Test sample intent" || len(d.Modules) != 3 || len(d.VoltageDomains) != 2 {
		t.Fatalf("parsed shape wrong: %+v", d)
	}
	if d.Modules[1].MPN != "MTFC4GACAJCN" || d.VoltageDomains[0].Rails[1] != "VDD_IO" {
		t.Errorf("field mapping wrong: %+v", d)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"no name":             `modules: [{name: X, class: c}]`,
		"empty declaration":   `name: Empty`,
		"module no criterion": "name: N\nmodules:\n  - {name: X}",
		"module no name":      "name: N\nmodules:\n  - {class: soc}",
		"domain no nominal":   "name: N\nvoltage_domains:\n  - {name: d, rails: [3V3]}",
		"domain no rails":     "name: N\nvoltage_domains:\n  - {name: d, nominal: 3.3}",
		"domain no name":      "name: N\nvoltage_domains:\n  - {nominal: 3.3, rails: [3V3]}",
		// A strap's value IS the assertion, so an omitted or misspelled level has to be a load error.
		// Accepting it would compile a rule with nothing to contradict, which then reads pass forever.
		"strap no value":   "name: N\nnet_properties:\n  - {net: BOOT0, property: strap}",
		"strap bad value":  "name: N\nnet_properties:\n  - {net: BOOT0, property: strap, value: pullup}",
		"strap no net":     "name: N\nnet_properties:\n  - {property: strap, value: high}",
		"unknown property": "name: N\nnet_properties:\n  - {net: BOOT0, property: strapp, value: high}",
		// A zero or negative peak is met by every supply, so it would be a declaration that can only
		// pass. Same reasoning as the strap value: reject it at load rather than compile a rule that
		// never fires.
		"budget no peak":       "name: N\nrail_budgets:\n  - {rail: 3V3}",
		"budget negative peak": "name: N\nrail_budgets:\n  - {rail: 3V3, peak: -1}",
		"budget no rail":       "name: N\nrail_budgets:\n  - {peak: 0.8}",
		"budget duplicate rail": "name: N\nrail_budgets:\n  - {rail: 3V3, peak: 0.8}\n" +
			"  - {rail: 3V3, peak: 1.2}",
		// A factor of 1 restates the capacity rule and below 1 asks for a supply SMALLER than the
		// budget. Both are author errors, not policies.
		"margin factor of one": "name: N\nrail_budgets:\n  - {rail: 3V3, peak: 0.8}\nmargin_factor: 1",
		"margin factor below one": "name: N\nrail_budgets:\n  - {rail: 3V3, peak: 0.8}\n" +
			"margin_factor: 0.9",
		// A factor with nothing to apply it to is a declaration that reads as covered and checks nothing.
		"margin factor alone": "name: N\nmodules:\n  - {name: X, class: soc}\nmargin_factor: 1.2",
	}
	for label, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
}

// TestParseRailBudgets: the WS3-095 form round-trips, and margin_factor is optional. Omitted it stays
// zero, which is what leaves the margin rule uncompiled (no house policy baked into a rule literal).
func TestParseRailBudgets(t *testing.T) {
	d, err := Parse([]byte("name: N\nrail_budgets:\n  - {rail: +3V3, peak: 0.8}\n  - {rail: +1V8, peak: 0.35}\nmargin_factor: 1.2\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(d.RailBudgets) != 2 || d.RailBudgets[0].Rail != "+3V3" || d.RailBudgets[0].Peak != 0.8 {
		t.Fatalf("rail budgets parsed wrong: %+v", d.RailBudgets)
	}
	if d.MarginFactor != 1.2 {
		t.Errorf("margin_factor = %g, want 1.2", d.MarginFactor)
	}
	bare, err := Parse([]byte("name: N\nrail_budgets:\n  - {rail: +3V3, peak: 0.8}\n"))
	if err != nil {
		t.Fatalf("Parse without a factor: %v", err)
	}
	if bare.MarginFactor != 0 {
		t.Errorf("an omitted margin_factor must stay 0 (no default), got %g", bare.MarginFactor)
	}
}

func TestParseErrorTeaches(t *testing.T) {
	_, err := Parse([]byte("name: N\nmodules:\n  - {name: Widget}"))
	if err == nil || !strings.Contains(err.Error(), "Widget") || !strings.Contains(err.Error(), "class") {
		t.Errorf("error should name the offending module and the missing field, got %v", err)
	}
}

// TestLoadStrapBandValidation (WS3-119): a band that could never be satisfied, or one declared on a
// kind with no resistance to bound, is an AUTHORING error. It is rejected at load rather than
// compiling to a check that can never fire, which is the route-six false pass (a well-formed-looking
// declaration that means nothing) — a runtime verdict is the wrong tool for a declaration that is
// wrong on every design.
func TestLoadStrapBandValidation(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{
			"band on a kind with no resistance",
			"name: t\nnet_properties:\n  - {net: CLK, property: ac-coupled, min_ohms: 1000}\n",
			"takes no min_ohms/max_ohms",
		},
		{
			"inverted band",
			"name: t\nnet_properties:\n  - {net: B0, property: strap, value: high, min_ohms: 100000, max_ohms: 1000}\n",
			"a band nothing can satisfy",
		},
		{
			"negative bound",
			"name: t\nnet_properties:\n  - {net: B0, property: strap, value: high, min_ohms: -5}\n",
			"negative resistance bound",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("want a load error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should say why: got %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestLoadStrapBandAccepted: the forms that ARE valid, including one-sided bands, survive the loader
// with their numbers intact.
func TestLoadStrapBandAccepted(t *testing.T) {
	d, err := Parse([]byte("name: t\nnet_properties:\n" +
		"  - {net: B0, property: strap, value: high, min_ohms: 1000, max_ohms: 100000}\n" +
		"  - {net: B1, property: strap, value: low, max_ohms: 47000}\n" +
		"  - {net: B2, property: strap, value: low}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []NetProperty{
		{Net: "B0", Property: PropStrap, Value: "high", MinOhms: 1000, MaxOhms: 100000},
		{Net: "B1", Property: PropStrap, Value: "low", MaxOhms: 47000},
		{Net: "B2", Property: PropStrap, Value: "low"},
	}
	if len(d.NetProperties) != len(want) {
		t.Fatalf("got %d properties, want %d", len(d.NetProperties), len(want))
	}
	for i, w := range want {
		if d.NetProperties[i] != w {
			t.Errorf("property %d = %+v, want %+v", i, d.NetProperties[i], w)
		}
	}
}

// TestLoadStrapGroupValidation (WS3-120): a group that could never be satisfied on ANY design is an
// authoring error, so it fails the load rather than compiling to a rule that reports a design finding.
func TestLoadStrapGroupValidation(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{
			"value wider than the declared bits",
			"name: t\nstrap_groups:\n  - {name: PHYAD, nets: [A1, A0], value: 9}\n",
			"which 2 net(s) cannot encode",
		},
		{
			"no nets",
			"name: t\nstrap_groups:\n  - {name: PHYAD, nets: [], value: 0}\n",
			"lists no \"nets\"",
		},
		{
			"a net used as two bits",
			"name: t\nstrap_groups:\n  - {name: PHYAD, nets: [A0, A0], value: 1}\n",
			"twice",
		},
		{
			"unknown default level",
			"name: t\nstrap_groups:\n  - {name: PHYAD, nets: [A0], value: 1, default: floating}\n",
			"want \"low\", \"high\", or omitted",
		},
		{
			"two groups slugifying to one rule name",
			"name: t\nstrap_groups:\n  - {name: PHY AD, nets: [A0], value: 1}\n  - {name: 'phy-ad', nets: [B0], value: 0}\n",
			"slugify to the same rule name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Fatalf("want a load error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error should say why: got %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestLoadStrapGroupAccepted: a declaration carrying ONLY strap_groups is valid (the empty-declaration
// guard has to know about the new form), and the fields survive the loader intact.
func TestLoadStrapGroupAccepted(t *testing.T) {
	d, err := Parse([]byte("name: t\nstrap_groups:\n" +
		"  - {name: PHYAD, device: U12, nets: [PHYAD2, PHYAD1, PHYAD0], value: 5, bus: MDIO, default: low}\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := StrapGroup{Name: "PHYAD", Device: "U12", Nets: []string{"PHYAD2", "PHYAD1", "PHYAD0"}, Value: 5, Bus: "MDIO", Default: "low"}
	if len(d.StrapGroups) != 1 {
		t.Fatalf("got %d groups, want 1", len(d.StrapGroups))
	}
	g := d.StrapGroups[0]
	if g.Name != want.Name || g.Device != want.Device || g.Value != want.Value || g.Bus != want.Bus || g.Default != want.Default {
		t.Errorf("group = %+v, want %+v", g, want)
	}
	if strings.Join(g.Nets, ",") != strings.Join(want.Nets, ",") {
		t.Errorf("nets = %v, want %v (MSB-first order must survive)", g.Nets, want.Nets)
	}
}
