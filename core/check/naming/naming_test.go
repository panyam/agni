package naming

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/classify"
	"github.com/panyam/agni/core/model"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

const cfgYAML = `
name: acme
rules:
  - name: signal-net-naming
    severity: info
    why: "signal nets are UPPER_SNAKE with a domain prefix"
    allow: ["^[A-Z]+_[A-Z0-9_]+$"]
    exempt: ["^TP[0-9]"]
`

func net(name string) *ir.Net {
	return &ir.Net{Name: name, Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}}
}

// TestSourceCompilesAndFires: the config round-trips into a namespaced catalog rule that
// fires on non-conforming names, honors exempt patterns and the always-exempt stubs, and
// checks the LEAF of hierarchy-qualified names.
func TestSourceCompilesAndFires(t *testing.T) {
	cfg, err := Parse([]byte(cfgYAML))
	if err != nil {
		t.Fatal(err)
	}
	src, err := Source(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := check.NewCatalog(check.Builtins, src)
	if err != nil {
		t.Fatal(err)
	}
	rule := cat.Lookup("acme/signal-net-naming")
	if rule == nil {
		t.Fatal("compiled rule missing from catalog under its namespace")
	}
	if rule.Severity != "info" || rule.Tags["source"] != "acme" {
		t.Errorf("severity=%q source tag=%q", rule.Severity, rule.Tags["source"])
	}

	m := check.NewModel(&ir.Design{Nets: []*ir.Net{
		net("CTRL_MAIN"),             // conforms -> silent
		net("badname"),               // no allow match -> fires
		net("/amp1/CTRL_SUB"),        // qualified, conforming leaf -> silent
		net("/amp1/lower"),           // qualified, non-conforming leaf -> fires
		net("TP1_RAW"),               // exempt pattern -> silent
		net("N$3"),                   // stub -> always silent
		net("unconnected-(U1-Pad2)"), // marker stub -> always silent
	}})
	got := map[string]bool{}
	for _, f := range rule.Findings(m) {
		got[check.EntityRef(f.Subject)] = true
	}
	want := map[string]bool{"badname": true, "/amp1/lower": true}
	if len(got) != len(want) || !got["badname"] || !got["/amp1/lower"] {
		t.Errorf("fired on %v, want %v", got, want)
	}
}

// TestSourceRejectsBadConfig: operator input fails with errors, never panics.
func TestSourceRejectsBadConfig(t *testing.T) {
	for name, cfg := range map[string]*configpb.NamingConvention{
		"no source name": {Rules: []*configpb.NamingRule{{Name: "x", Allow: []string{"a"}}}},
		"no rules":       {Name: "acme"},
		"no allow":       {Name: "acme", Rules: []*configpb.NamingRule{{Name: "x"}}},
		"bad severity":   {Name: "acme", Rules: []*configpb.NamingRule{{Name: "x", Severity: "fatal", Allow: []string{"a"}}}},
		"bad regex":      {Name: "acme", Rules: []*configpb.NamingRule{{Name: "x", Allow: []string{"("}}}},
	} {
		if _, err := Source(cfg); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// TestParseRejectsUnknownKeys: a typo'd config key fails loudly instead of silently
// dropping a pattern list.
func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse([]byte("name: acme\nrules:\n  - name: x\n    alow: [\"a\"]\n"))
	if err == nil || !strings.Contains(err.Error(), "alow") {
		t.Errorf("want unknown-field error naming the typo, got %v", err)
	}
}

// TestApplyLexicon: a lexicon-only config (no rules) parses and installs its rail/feedback overrides
// onto the process vocab, extending the built-ins; the defaults themselves are untouched.
func TestApplyLexicon(t *testing.T) {
	defer check.SetActiveRoleVocab(nil)
	cfg, err := Parse([]byte(`
name: acme
lexicon:
  net:
    rail:
      patterns: ["^HV_"]
    feedback:
      patterns: ["_ETH_FB$"]
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rail := cfg.GetLexicon().GetNet().GetRail().GetPatterns(); len(rail) != 1 || rail[0] != "^HV_" {
		t.Fatalf("lexicon did not parse: %+v", cfg.GetLexicon())
	}
	if err := ApplyLexicon(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	active := check.ActiveRoleVocab()
	if !active.IsRail("HV_BATT") || !active.IsRail("VCC") {
		t.Error("active rail vocab should match the project pattern AND the built-ins")
	}
	if !active.IsFeedback("VCC0.8_ETH_FB") {
		t.Error("active feedback vocab should match the project pattern")
	}
	if check.DefaultRoleVocab().IsRail("HV_BATT") {
		t.Error("the defaults must not carry the project pattern")
	}
}

// TestApplyLexiconClass: a lexicon class block installs a per-class pattern; an unknown class name is a
// teaching error.
func TestApplyLexiconClass(t *testing.T) {
	defer check.SetActiveClassVocab(nil)
	cfg, err := Parse([]byte(`
name: acme
lexicon:
  class:
    tvs:
      patterns: ["^pesd"]
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := ApplyLexicon(cfg); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !check.ActiveClassVocab().HintsFor([]string{"pesd2eth1gt"})[check.ClassTVS] {
		t.Error("active class vocab should carry the project tvs pattern")
	}
	// unknown class name teaches
	bad, _ := Parse([]byte("name: acme\nlexicon:\n  class:\n    bogus:\n      patterns: [\"^x\"]\n"))
	if err := ApplyLexicon(bad); err == nil {
		t.Error("an unknown component class in a lexicon must error")
	}
}

// TestEveryShippedClassLoadsFromYAML: a project may extend every class the engine ships, walked from
// the YAML a project writes. The names a config may use were a hand-kept list that lacked thermistor,
// zener and ideal_diode_controller, so conventions naming any of them failed to load (agni 677).
func TestEveryShippedClassLoadsFromYAML(t *testing.T) {
	for _, cl := range model.ComponentClasses() {
		cfg, err := Parse([]byte("name: acme\nlexicon:\n  class:\n    " + string(cl) + ":\n      patterns: [\"^zzz$\"]\n"))
		if err != nil {
			t.Fatalf("parse %s: %v", cl, err)
		}
		if _, err := BuildLexicon(cfg); err != nil {
			t.Errorf("class %q ships with the engine and must be extendable: %v", cl, err)
		}
	}
}

// TestClassPrefixesReachTheReadFromYAML: a prefix declared in YAML gives a part its class, and the
// family tag rides along, through the per-read lexicon rather than a process install.
func TestClassPrefixesReachTheReadFromYAML(t *testing.T) {
	cfg, err := Parse([]byte(`
name: acme
lexicon:
  class:
    thermistor: { prefixes: ["TH"] }
    zener:      { prefixes: ["z"] }
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	lex, err := BuildLexicon(cfg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	d := &ir.Design{Components: []*ir.Component{{RefDes: "TH3"}, {RefDes: "Z1"}, {RefDes: "R1"}}}
	lex.Stamp(d)
	want := map[string]string{"TH3": "thermistor,resistor", "Z1": "zener,diode", "R1": "resistor"}
	for _, c := range d.GetComponents() {
		if got := strings.Join(classify.ClassNames(c), ","); got != want[c.GetRefDes()] {
			t.Errorf("%s: classes %q, want %q", c.GetRefDes(), got, want[c.GetRefDes()])
		}
	}
	// Positive control: without the project's prefixes neither part classifies.
	classify.DefaultLexicon().Stamp(d)
	if got := d.GetComponents()[0].GetDeviceClasses(); len(got) != 0 {
		t.Errorf("TH3 classified %v under the built-in vocabulary, so the test above proves nothing", got)
	}
}

// TestEveryNetVocabularyReachesTheEngineFromYAML walks the WHOLE path a project's config takes:
// YAML text, protojson, the generated message, BuildRoleVocab, the compiled vocabulary. It covers
// every net role rather than a sample, because the defect this guards is one vocabulary being absent
// from the copy that fills the engine, and a test naming three of six would have passed throughout.
//
// That defect shipped twice. WS3-117 added gate/source/drain to the Go override struct and not to the
// wire form, so a project declaring them had them silently dropped. Agni 680 did the same to
// switching, control and gate_drive, and the test written alongside it called BuildRoleVocab directly
// and so proved nothing about the path a project actually uses.
func TestEveryNetVocabularyReachesTheEngineFromYAML(t *testing.T) {
	defer check.SetActiveRoleVocab(nil)
	cfg, err := Parse([]byte(`
name: acme
lexicon:
  net:
    rail:       { patterns: ["^HV_"] }
    ground:     { patterns: ["^CHASSIS_"] }
    feedback:   { patterns: ["_FBK$"] }
    switching:  { patterns: ["_HSD$"] }
    control:    { patterns: ["_SHDN$"] }
    gate_drive: { patterns: ["_VBOOST$"] }
  pin:
    supply: { patterns: ["^PWR_"] }
    gate:   { patterns: ["^DRV$"] }
    source: { patterns: ["^SRC_"] }
    drain:  { patterns: ["^DRN_"] }
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	lex, err := BuildLexicon(cfg)
	if err != nil {
		t.Fatalf("BuildLexicon: %v", err)
	}
	v := lex.RoleVocab()

	for _, c := range []struct {
		vocab string
		name  string
		got   func(string) bool
	}{
		{"rail", "HV_BUS", v.IsRail},
		{"ground", "CHASSIS_0", v.IsGround},
		{"feedback", "12V_FBK", v.IsFeedback},
		{"switching", "12V_HSD", v.IsSwitching},
		{"control", "12V_SHDN", v.IsControl},
		{"gate_drive", "12V_VBOOST", v.IsGateDrive},
		{"supply_pin", "PWR_IN", v.IsSupplyPin},
		{"gate", "DRV", v.IsGate},
		{"source", "SRC_A", v.IsSource},
		{"drain", "DRN_A", v.IsDrain},
	} {
		if !c.got(c.name) {
			t.Errorf("%s: %q declared in conventions.yaml never reached the engine", c.vocab, c.name)
		}
	}

	// The override EXTENDS rather than replaces, so the built-ins still answer. Without this the
	// assertions above would also pass for a build that threw the defaults away.
	if !v.IsSwitching("12V_SW") {
		t.Error("switching: an added pattern displaced the built-in _SW")
	}
	if !v.IsRail("+3V3") {
		t.Error("rail: an added pattern displaced the built-ins")
	}
}

// TestLexiconReadsBackItsPatterns: RoleVocab embeds the lexicon it compiled, so a caller can see the
// EFFECTIVE patterns rather than only ask yes/no. ActiveRoleVocab's doc has promised that since
// WS3-069 and could not deliver while the patterns were discarded at compile time.
func TestLexiconReadsBackItsPatterns(t *testing.T) {
	cfg, err := Parse([]byte("name: acme\nlexicon:\n  net:\n    switching: { patterns: [\"_HSD$\"] }\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	lex, err := BuildLexicon(cfg)
	if err != nil {
		t.Fatalf("BuildLexicon: %v", err)
	}
	got := lex.RoleVocab().GetNet().GetSwitching().GetPatterns()
	var found bool
	for _, p := range got {
		if p == "_HSD$" {
			found = true
		}
	}
	if !found {
		t.Errorf("the project's own pattern is not readable off the vocabulary: %v", got)
	}
	if len(got) < 2 {
		t.Errorf("the effective set should carry the built-ins too, got %v", got)
	}
}
