package naming

import (
	"strings"
	"testing"

	"github.com/panyam/agni/check"
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
	for _, f := range rule.Eval(m) {
		got[f.Subject] = true
	}
	want := map[string]bool{"badname": true, "/amp1/lower": true}
	if len(got) != len(want) || !got["badname"] || !got["/amp1/lower"] {
		t.Errorf("fired on %v, want %v", got, want)
	}
}

// TestSourceRejectsBadConfig: operator input fails with errors, never panics.
func TestSourceRejectsBadConfig(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no source name": {Rules: []RuleConfig{{Name: "x", Allow: []string{"a"}}}},
		"no rules":       {Name: "acme"},
		"no allow":       {Name: "acme", Rules: []RuleConfig{{Name: "x"}}},
		"bad severity":   {Name: "acme", Rules: []RuleConfig{{Name: "x", Severity: "fatal", Allow: []string{"a"}}}},
		"bad regex":      {Name: "acme", Rules: []RuleConfig{{Name: "x", Allow: []string{"("}}}},
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
  rail:
    patterns: ["^HV_"]
  feedback:
    patterns: ["_ETH_FB$"]
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Lexicon == nil || len(cfg.Lexicon.Rail.Patterns) != 1 || cfg.Lexicon.Rail.Patterns[0] != "^HV_" {
		t.Fatalf("lexicon did not parse: %+v", cfg.Lexicon)
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
