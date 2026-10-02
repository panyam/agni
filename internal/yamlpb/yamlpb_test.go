package yamlpb

import (
	"strings"
	"testing"

	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	"gopkg.in/yaml.v3"
)

func decode(t *testing.T, src string) (*configpb.DesignIntent, error) {
	t.Helper()
	var n yaml.Node
	if err := yaml.Unmarshal([]byte(src), &n); err != nil {
		t.Fatal(err)
	}
	msg := &configpb.DesignIntent{}
	return msg, Decode(&n, msg)
}

// A scalar takes the type of the FIELD it fills, so an unquoted pin number is the pin named "9" and a
// strap value written as a number is a number.
func TestDecodeTypesScalarsByTheirField(t *testing.T) {
	m, err := decode(t, `
io_map:
  - {net: SDA, device: U3, pin: 9, to: {device: U1, pin: 05}}
nets:
  3V3: {nominal: 3.3, peak: 1, ac_coupled: true}
strap_groups:
  - {name: A, device: U1, nets: [X, Y], value: 2}
margin_factor: 1.25
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.GetIoMap()[0].GetPin(); got != "9" {
		t.Errorf("pin = %q, want \"9\"", got)
	}
	// The string is the text as written, so a zero-padded designator survives for core/ident to judge.
	if got := m.GetIoMap()[0].GetTo().GetPin(); got != "05" {
		t.Errorf("far-end pin = %q, want \"05\" as written", got)
	}
	n := m.GetNets()["3V3"]
	if n.GetNominal() != 3.3 || n.GetPeak() != 1 || !n.GetAcCoupled() || n.Nominal == nil {
		t.Errorf("net facts = %+v", n)
	}
	if m.GetStrapGroups()[0].GetValue() != 2 || m.GetMarginFactor() != 1.25 {
		t.Errorf("numbers = %d, %g", m.GetStrapGroups()[0].GetValue(), m.GetMarginFactor())
	}
}

// Presence survives the conversion: an absent nominal is nil, and a stated zero is a zero someone wrote.
func TestDecodeKeepsPresence(t *testing.T) {
	m, err := decode(t, "nets:\n  A: {peak: 1}\n  B: {nominal: 0}\n")
	if err != nil {
		t.Fatal(err)
	}
	if m.GetNets()["A"].Nominal != nil {
		t.Error("an absent nominal must stay absent")
	}
	if b := m.GetNets()["B"]; b.Nominal == nil || *b.Nominal != 0 {
		t.Error("a stated zero nominal must be present, so the loader can refuse it")
	}
}

func TestDecodeRefuses(t *testing.T) {
	for label, c := range map[string]struct{ src, want string }{
		"unknown key, with its line": {"modules:\n  - {name: X, class: soc}\nnetz: {}\n", `line 3: unknown key "netz"`},
		"nested unknown key":         {"nets:\n  A:\n    strapp: high\n", `line 3: unknown key "strapp" in "nets.A"`},
		"lowerCamel spelling":        {"nets:\n  A: {acCoupled: true}\n", `unknown key "acCoupled"`},
		"text for a number":          {"nets:\n  A: {peak: lots}\n", `"nets.A.peak" must be a number`},
		"fraction for a whole":       {"strap_groups:\n  - {name: A, nets: [X], value: 1.5}\n", "must be a whole number"},
		"word for a bool":            {"nets:\n  A: {ac_coupled: yes}\n", "must be true or false"},
		"list for a scalar":          {"nets:\n  A: {strap: [high]}\n", "must be a single value"},
		"scalar for a list":          {"nets:\n  A: {protect: ovp}\n", "must be a list"},
		"null map key":               {"nets:\n  ~: {peak: 1}\n", "non-empty keys"},
		"empty list entry":           {"modules:\n  -\n", "has an empty entry"},
		"mapping expected":           {"nets: [A, B]\n", `"nets" must be a mapping`},
	} {
		_, err := decode(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error = %v, want it to contain %q", label, err, c.want)
		}
	}
}

func TestDecodeFollowsAliases(t *testing.T) {
	m, err := decode(t, "nets:\n  A: &rail {nominal: 3.3, domain: io}\n  B: *rail\n")
	if err != nil {
		t.Fatal(err)
	}
	if m.GetNets()["B"].GetDomain() != "io" {
		t.Errorf("an aliased entry must decode as its anchor, got %+v", m.GetNets()["B"])
	}
}

// An empty map entry still exists, so the loader can refuse a net that declares nothing rather than
// never seeing it.
func TestDecodeKeepsAnEmptyMapEntry(t *testing.T) {
	m, err := decode(t, "nets:\n  A: {}\n  B:\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.GetNets()["A"]; !ok {
		t.Error("A: {} must decode as a present, empty entry")
	}
	if _, ok := m.GetNets()["B"]; !ok {
		t.Error("B with no value must decode as a present, empty entry")
	}
}
