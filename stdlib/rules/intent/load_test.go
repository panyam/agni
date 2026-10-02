package intent

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	d, err := Parse([]byte(`
name: Test sample intent
intent:
  modules:
  - {name: SoC, class: soc}
  - {name: eMMC, mpn: MTFC4GACAJCN}
  - {name: CAN xcvr, class: can_transceiver, mpn: TCAN1042}
  nets:
    3V3: {nominal: 3.3, domain: io_3v3}
    VDD_IO: {nominal: 3.3, domain: io_3v3}
    VDD_CORE: {nominal: 0.8, domain: core}
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
		"no name":             "intent:\n  modules:\n  - {name: X, class: c}\n",
		"no intent section":   "name: Empty\n",
		"empty declaration":   "name: Empty\nintent: {}\n",
		"module no criterion": "name: N\nintent:\n  modules:\n  - {name: X}\n",
		"module no name":      "name: N\nintent:\n  modules:\n  - {class: soc}\n",
		// A block with nets is checked as one block, so a count on it would compile to nothing.
		"count on a block with nets": "name: N\nintent:\n  modules:\n  - {name: X, class: soc, count: 2, nets: [A]}\n",
		"net declares nothing":       "name: N\nintent:\n  nets:\n    3V3: {}\n",
		"nominal not positive":       "name: N\nintent:\n  nets:\n    3V3: {nominal: 0}\n",
		"domain with no nominal":     "name: N\nintent:\n  nets:\n    3V3: {domain: io}\n",
		"one domain, two voltages":   "name: N\nintent:\n  nets:\n    3V3: {nominal: 3.3, domain: io}\n    1V8: {nominal: 1.8, domain: io}\n",
		// A strap's value IS the assertion, so an omitted or misspelled level has to be a load error.
		// Accepting it would compile a rule with nothing to contradict, which then reads pass forever.
		"strap bad value":    "name: N\nintent:\n  nets:\n    BOOT0: {strap: pullup}\n",
		"reset bad value":    "name: N\nintent:\n  nets:\n    NRST: {reset: active}\n",
		"misspelled fact":    "name: N\nintent:\n  nets:\n    BOOT0: {strap: high, max_ohm: 1000}\n",
		"band with no strap": "name: N\nintent:\n  nets:\n    BOOT0: {max_ohms: 1000, reset: low}\n",
		"unknown protection": "name: N\nintent:\n  nets:\n    3V3: {protect: [clamp]}\n",
		"protection twice":   "name: N\nintent:\n  nets:\n    3V3: {protect: [ovp, ovp]}\n",
		// A zero or negative peak is met by every supply, so it would be a declaration that can only
		// pass. Same reasoning as the strap value, so reject it at load rather than compile a rule that
		// never fires.
		"zero peak":     "name: N\nintent:\n  nets:\n    3V3: {peak: 0}\n",
		"negative peak": "name: N\nintent:\n  nets:\n    3V3: {peak: -1}\n",
		// A factor of 1 restates the capacity rule and below 1 asks for a supply SMALLER than the
		// budget. Both are author errors, not policies.
		"margin factor of one":    "name: N\nintent:\n  nets:\n    3V3: {peak: 0.8}\n  margin_factor: 1\n",
		"margin factor below one": "name: N\nintent:\n  nets:\n    3V3: {peak: 0.8}\n  margin_factor: 0.9\n",
		// A factor with nothing to apply it to is a declaration that reads as covered and checks nothing.
		"margin factor alone": "name: N\nintent:\n  modules:\n  - {name: X, class: soc}\n  margin_factor: 1.2\n",
	}
	for label, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
}

// A file written for the nine-form layout fails at load and says what to write instead, rather than
// reading as a design that declares nothing (agni issue 824).
func TestParseRefusesTheEarlierLayout(t *testing.T) {
	for label, c := range map[string]struct{ doc, want string }{
		"forms at the top level":    {"name: N\nmodules:\n  - {name: X, class: soc}\n", `sits under "intent:"`},
		"voltage_domains in intent": {"name: N\nintent:\n  voltage_domains:\n  - {name: io, nominal: 3.3, rails: [3V3]}\n", "nets: {RAIL: {nominal: VOLTS, domain: NAME}}"},
		"protections in intent":     {"name: N\nintent:\n  protections:\n  - {rail: 3V3, kind: ovp}\n", "protect: [ovp, discharge]"},
		"net_properties in intent":  {"name: N\nintent:\n  net_properties:\n  - {net: NRST, property: reset-polarity, value: low}\n", "reset: low|high"},
		"rail_budgets in intent":    {"name: N\nintent:\n  rail_budgets:\n  - {rail: 3V3, peak: 1}\n", "peak: AMPS"},
		"subsystems in intent":      {"name: N\nintent:\n  subsystems:\n  - {name: reset, nets: [NRST]}\n", "modules: [{name: NAME, nets:"},
		"intent naming a file":      {"name: N\nintent: intent.yaml\n", "declared inline"},
	} {
		_, err := Parse([]byte(c.doc))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to say %q", label, err, c.want)
		}
	}
}

// A rail with a nominal and no domain label is named by its voltage, and rails sharing a label and a
// voltage form one domain.
func TestNetsFormVoltageDomains(t *testing.T) {
	d, err := Parse([]byte("name: N\nintent:\n  nets:\n" +
		"    VDD_IO: {nominal: 3.3, domain: io}\n" +
		"    3V3_AUX: {nominal: 3.3, domain: io}\n" +
		"    1V8: {nominal: 1.8}\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, v := range d.VoltageDomains {
		got[fmt.Sprintf("%s@%g", v.Name, v.Nominal)] = v.Rails
	}
	want := map[string][]string{"1.8V@1.8": {"1V8"}, "io@3.3": {"3V3_AUX", "VDD_IO"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("domains = %v, want %v", got, want)
	}
}

// TestParseRailBudgets checks that the WS3-095 form round-trips and margin_factor is optional.
// Omitted it stays zero, which is what leaves the margin rule uncompiled (no house policy baked
// into a rule literal).
func TestParseRailBudgets(t *testing.T) {
	d, err := Parse([]byte("name: N\nintent:\n  nets:\n    +3V3: {peak: 0.8}\n    +1V8: {peak: 0.35}\n  margin_factor: 1.2\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(d.RailBudgets) != 2 || d.RailBudgets[1].Rail != "+3V3" || d.RailBudgets[1].Peak != 0.8 {
		t.Fatalf("rail budgets parsed wrong: %+v", d.RailBudgets)
	}
	if d.MarginFactor != 1.2 {
		t.Errorf("margin_factor = %g, want 1.2", d.MarginFactor)
	}
	bare, err := Parse([]byte("name: N\nintent:\n  nets:\n    +3V3: {peak: 0.8}\n"))
	if err != nil {
		t.Fatalf("Parse without a factor: %v", err)
	}
	if bare.MarginFactor != 0 {
		t.Errorf("an omitted margin_factor must stay 0 (no default), got %g", bare.MarginFactor)
	}
}

func TestParseErrorTeaches(t *testing.T) {
	_, err := Parse([]byte("name: N\nintent:\n  modules:\n  - {name: Widget}\n"))
	if err == nil || !strings.Contains(err.Error(), "Widget") || !strings.Contains(err.Error(), "class") {
		t.Errorf("error should name the offending module and the missing field, got %v", err)
	}
}

// TestLoadStrapBandValidation (WS3-119) covers a band that could never be satisfied, or one
// declared on a kind with no resistance to bound, is an AUTHORING error. It is rejected at load
// rather than compiling to a check that can never fire, which is the route-six false pass (a
// well-formed-looking declaration that means nothing). A runtime verdict is the wrong tool for a
// declaration that is wrong on every design.
func TestLoadStrapBandValidation(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{
			"band on a kind with no resistance",
			"name: t\nintent:\n  nets:\n    CLK: {ac_coupled: true, max_ohms: 1000}\n",
			`with no "strap"`,
		},
		{
			"inverted band",
			"name: t\nintent:\n  nets:\n    B0: {strap: high, min_ohms: 100000, max_ohms: 1000}\n",
			"a band nothing can satisfy",
		},
		{
			"negative bound",
			"name: t\nintent:\n  nets:\n    B0: {strap: high, min_ohms: -5}\n",
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

// TestLoadStrapBandAccepted checks that the forms that ARE valid, including one-sided bands,
// survive the loader with their numbers intact.
func TestLoadStrapBandAccepted(t *testing.T) {
	d, err := Parse([]byte("name: t\nintent:\n  nets:\n" +
		"    B0: {strap: high, min_ohms: 1000, max_ohms: 100000}\n" +
		"    B1: {strap: low, max_ohms: 47000}\n" +
		"    B2: {strap: low}\n"))
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

// TestLoadStrapGroupValidation (WS3-120) covers a group that could never be satisfied on ANY
// design. That is an authoring error, so it fails the load rather than compiling to a rule that
// reports a design finding.
func TestLoadStrapGroupValidation(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{
			"value wider than the declared bits",
			"name: t\nintent:\n  strap_groups:\n  - name: PHYAD\n    nets: [A1, A0]\n    value: 9\n",
			"which 2 net(s) cannot encode",
		},
		{
			"no nets",
			"name: t\nintent:\n  strap_groups:\n  - name: PHYAD\n    nets: []\n    value: 0\n",
			"lists no \"nets\"",
		},
		{
			"a net used as two bits",
			"name: t\nintent:\n  strap_groups:\n  - name: PHYAD\n    nets: [A0, A0]\n    value: 1\n",
			"twice",
		},
		{
			"unknown default level",
			"name: t\nintent:\n  strap_groups:\n  - name: PHYAD\n    nets: [A0]\n    value: 1\n    default: floating\n",
			"want \"low\", \"high\", or omitted",
		},
		{
			"two groups slugifying to one rule name",
			"name: t\nintent:\n  strap_groups:\n  - name: PHY AD\n    nets: [A0]\n    value: 1\n  - name: phy-ad\n    nets: [B0]\n    value: 0\n",
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

// TestLoadStrapGroupAccepted checks that a declaration carrying ONLY strap_groups is valid (the
// empty-declaration guard has to know about the new form), and the fields survive the loader
// intact.
func TestLoadStrapGroupAccepted(t *testing.T) {
	d, err := Parse([]byte("name: t\nintent:\n  strap_groups:\n" +
		"    - {name: PHYAD, device: U12, nets: [PHYAD2, PHYAD1, PHYAD0], value: 5, bus: MDIO, default: low}\n"))
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
