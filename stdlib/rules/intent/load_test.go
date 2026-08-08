package intent

import (
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	d, err := Parse([]byte(`
name: Test ECU intent
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
	if d.Name != "Test ECU intent" || len(d.Modules) != 3 || len(d.VoltageDomains) != 2 {
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
	}
	for label, doc := range cases {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: expected a validation error, got nil", label)
		}
	}
}

func TestParseErrorTeaches(t *testing.T) {
	_, err := Parse([]byte("name: N\nmodules:\n  - {name: Widget}"))
	if err == nil || !strings.Contains(err.Error(), "Widget") || !strings.Contains(err.Error(), "class") {
		t.Errorf("error should name the offending module and the missing field, got %v", err)
	}
}
