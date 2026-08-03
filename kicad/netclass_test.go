package kicad

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestParseAndAnnotateNetClasses covers WS1-037: net_class is populated from the .kicad_pro
// net_settings (patterns, in file order, plus explicit assignments), the only place KiCad
// records net-class membership. Wildcards span '/' (net names embed sheet paths), and a net
// matching no rule is left untouched.
func TestParseAndAnnotateNetClasses(t *testing.T) {
	const pro = `{
	  "net_settings": {
	    "netclass_assignments": { "/analog/REF": "Precision" },
	    "netclass_patterns": [
	      { "netclass": "HighSpeed", "pattern": "/usb/USB_*" },
	      { "netclass": "Power", "pattern": "+*" }
	    ]
	  }
	}`
	rules := ParseNetClasses(strings.NewReader(pro))

	d := &ir.Design{Nets: []*ir.Net{
		{Name: "/usb/USB_D+"},
		{Name: "+3V3"},
		{Name: "/analog/REF"},
		{Name: "SIG"},
	}}
	AnnotateNetClasses(d, rules)

	want := map[string]string{
		"/usb/USB_D+": "HighSpeed", // pattern spans '/'
		"+3V3":        "Power",     // wildcard tail
		"/analog/REF": "Precision", // explicit assignment beats patterns
		"SIG":         "",          // no rule -> untouched
	}
	for _, n := range d.Nets {
		if got := n.NetClass; got != want[n.Name] {
			t.Errorf("net %q class = %q, want %q", n.Name, got, want[n.Name])
		}
	}
}

func TestParseNetClassesEmptyAndMalformed(t *testing.T) {
	// A netclass-free or malformed project yields empty rules, never a panic or error, so a
	// project read never fails on advisory net-class metadata.
	for _, in := range []string{`{}`, `{"net_settings":{"netclass_patterns":[],"netclass_assignments":null}}`, `not json`} {
		if c := ParseNetClasses(strings.NewReader(in)).ClassOf("ANY"); c != "" {
			t.Errorf("ClassOf on %q = %q, want empty", in, c)
		}
	}
	// A nil receiver / nil design is safe.
	AnnotateNetClasses(nil, ParseNetClasses(strings.NewReader(`{}`)))
	if (*NetClassRules)(nil).ClassOf("X") != "" {
		t.Error("nil rules ClassOf should be empty")
	}
}

func TestWildcardMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"USB_*", "USB_D+", true},
		{"USB_*", "USB_", true},
		{"USB_*", "USBD", false},
		{"/pwr/*", "/pwr/GND", true},
		{"/pwr/*", "/pwr/rail/5V", true}, // * spans '/'
		{"?GND", "AGND", true},
		{"?GND", "GND", false},
		{"GND", "GND", true},
		{"GND", "GNDA", false},
		{"*", "anything", true},
	}
	for _, c := range cases {
		if got := wildcardMatch(c.pattern, c.s); got != c.want {
			t.Errorf("wildcardMatch(%q, %q) = %v, want %v", c.pattern, c.s, got, c.want)
		}
	}
}
