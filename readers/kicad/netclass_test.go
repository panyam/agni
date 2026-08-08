package kicad

import (
	"slices"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestParseAndAnnotateNetClasses covers WS1-037: net_classes is populated from the .kicad_pro
// net_settings (patterns plus explicit assignments), the only place KiCad records net-class
// membership. Wildcards span '/' (net names embed sheet paths), and a net matching no rule is
// left untouched.
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

	want := map[string][]string{
		"/usb/USB_D+": {"HighSpeed"}, // pattern spans '/'
		"+3V3":        {"Power"},     // wildcard tail
		"/analog/REF": {"Precision"}, // explicit assignment, matching no pattern
		"SIG":         nil,           // no rule -> untouched
	}
	for _, n := range d.Nets {
		if got := n.NetClasses; !slices.Equal(got, want[n.Name]) {
			t.Errorf("net %q classes = %v, want %v", n.Name, got, want[n.Name])
		}
	}
}

// TestNetClassMultipleMemberships is the WS1-050 acceptance: KiCad stores a net's membership as a
// SET (map<netname, set<netclass>>), unioning the explicit assignment with EVERY matching pattern,
// so all three of the reader's old collapse points must carry every class instead of one.
func TestNetClassMultipleMemberships(t *testing.T) {
	const pro = `{
	  "net_settings": {
	    "netclass_assignments": { "VBUS": ["Power", "HighCurrent"], "SDA": "I2C" },
	    "netclass_patterns": [
	      { "netclass": "HighSpeed", "pattern": "USB_*" },
	      { "netclass": "Differential", "pattern": "*_D?" },
	      { "netclass": "Slow", "pattern": "SDA" }
	    ]
	  }
	}`
	rules := ParseNetClasses(strings.NewReader(pro))

	for _, tc := range []struct {
		net  string
		want []string
		why  string
	}{
		{"VBUS", []string{"HighCurrent", "Power"}, "array-form assignment keeps every entry, not arr[0]"},
		{"USB_DP", []string{"Differential", "HighSpeed"}, "every matching pattern applies, not the first"},
		{"SDA", []string{"I2C", "Slow"}, "an explicit assignment does not short-circuit pattern matching"},
		{"NC", nil, "a net matching nothing stays unclassed"},
	} {
		if got := rules.ClassesOf(tc.net); !slices.Equal(got, tc.want) {
			t.Errorf("ClassesOf(%q) = %v, want %v (%s)", tc.net, got, tc.want, tc.why)
		}
	}
}

// TestNetClassesSortedAndDeduped pins the ordering contract the IR field documents: sorted for
// determinism (the assignment map is a Go map, so read order is random), and deduplicated so a net
// named by both an assignment and a pattern of the same class carries that class once.
func TestNetClassesSortedAndDeduped(t *testing.T) {
	const pro = `{
	  "net_settings": {
	    "netclass_assignments": { "N": ["zeta", "Power", "alpha"] },
	    "netclass_patterns": [ { "netclass": "Power", "pattern": "N" } ]
	  }
	}`
	rules := ParseNetClasses(strings.NewReader(pro))
	want := []string{"Power", "alpha", "zeta"}
	for range 20 {
		if got := rules.ClassesOf("N"); !slices.Equal(got, want) {
			t.Fatalf("ClassesOf(N) = %v, want %v (stable sorted, deduped)", got, want)
		}
	}
}

func TestParseNetClassesEmptyAndMalformed(t *testing.T) {
	// A netclass-free or malformed project yields empty rules, never a panic or error, so a
	// project read never fails on advisory net-class metadata.
	for _, in := range []string{`{}`, `{"net_settings":{"netclass_patterns":[],"netclass_assignments":null}}`, `not json`} {
		if c := ParseNetClasses(strings.NewReader(in)).ClassesOf("ANY"); len(c) != 0 {
			t.Errorf("ClassesOf on %q = %v, want empty", in, c)
		}
	}
	// A nil receiver / nil design is safe.
	AnnotateNetClasses(nil, ParseNetClasses(strings.NewReader(`{}`)))
	if len((*NetClassRules)(nil).ClassesOf("X")) != 0 {
		t.Error("nil rules ClassesOf should be empty")
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

// TestParseNetClassDefs covers WS3-111: the classes[] block carries each class's declared routing
// constraints. Absent and zero must stay distinguishable (a class that states no track width lets a
// lower-priority class fill it), and an unstated priority must cascade LAST rather than first.
func TestParseNetClassDefs(t *testing.T) {
	const pro = `{
	  "net_settings": {
	    "classes": [
	      {"name": "Default", "priority": 2147483647, "clearance": 0.2, "track_width": 0.25,
	       "via_diameter": 0.6, "via_drill": 0.3},
	      {"name": "HighSpeed", "priority": 1, "clearance": 0.15},
	      {"name": "Power", "priority": 5, "track_width": 0.8},
	      {"name": "NoPriority", "track_width": 0.4},
	      {"name": ""}
	    ]
	  }
	}`
	defs := ParseNetClassDefs(strings.NewReader(pro))

	var names []string
	for _, d := range defs {
		names = append(names, d.Name)
	}
	// Sorted by cascade rank: stated priorities first, then the unstated ones and Default.
	if want := []string{"HighSpeed", "Power", "Default", "NoPriority"}; !slices.Equal(names, want) {
		t.Errorf("cascade order = %v, want %v (unstated priority sorts last, not first)", names, want)
	}

	byName := map[string]NetClassDef{}
	for _, d := range defs {
		byName[d.Name] = d
	}
	// HighSpeed declares ONLY a clearance: its other fields must be absent, not 0.
	hs := byName["HighSpeed"]
	if hs.Clearance == nil || *hs.Clearance != 0.15 {
		t.Errorf("HighSpeed clearance = %v, want 0.15", hs.Clearance)
	}
	if hs.TrackWidth != nil {
		t.Errorf("HighSpeed track width = %v, want ABSENT (it declares none, so it cascades)", *hs.TrackWidth)
	}
	if d := byName["Default"]; d.ViaDrill == nil || *d.ViaDrill != 0.3 {
		t.Errorf("Default via drill = %v, want 0.3", d.ViaDrill)
	}
	if _, ok := byName[""]; ok {
		t.Error("an unnamed class must be skipped")
	}
}

// TestAnnotateNetClassDefs: the definitions land on ir.Design as Constraint nodes of kind
// "netclass" — the first reader to populate that provisional node. Only STATED scalars become
// params, so a consumer can still tell "declares none" from "declares zero".
func TestAnnotateNetClassDefs(t *testing.T) {
	d := &ir.Design{}
	AnnotateNetClassDefs(d, []NetClassDef{{Name: "HighSpeed", Priority: 1, Clearance: ptr(0.15)}})
	if len(d.Constraints) != 1 {
		t.Fatalf("constraints = %v, want 1", d.Constraints)
	}
	c := d.Constraints[0]
	if c.Kind != ConstraintKindNetClass || c.Name != "HighSpeed" {
		t.Errorf("constraint = {%q %q}, want {HighSpeed netclass}", c.Name, c.Kind)
	}
	if c.Params["clearance"] != "0.15" || c.Params["priority"] != "1" {
		t.Errorf("params = %v, want clearance 0.15 / priority 1", c.Params)
	}
	if _, ok := c.Params["track_width"]; ok {
		t.Errorf("params carry a track_width %q, want the key ABSENT (none was declared)", c.Params["track_width"])
	}
}

func ptr(f float64) *float64 { return &f }
