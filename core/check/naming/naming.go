// Package naming compiles an operator-supplied net-naming convention into a
// check.RuleSource (WS3-015). It is the catalog's first CONFIG-CARRYING source, and it is
// deliberately built on the check package's exported surface alone — the Spec AST and
// NewSource — so it doubles as proof that an out-of-tree rule suite needs no second
// registry: config in, ordinary namespaced rules out (WS3-006).
//
// A convention is data, not code: the pattern set belongs to a project or customer and
// must not be baked into the shareable engine. The config stays small on purpose (names,
// severities, allow/exempt regex lists); anything needing more than patterns over net
// names is a real rule and belongs in Go/Spec (the DSL-deferred posture, docs/19).
package naming

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"github.com/panyam/agni/core/check"
	"gopkg.in/yaml.v3"
)

// Config is one convention file: a source name (the catalog namespace the rules appear
// under, e.g. "acme" -> "acme/signal-net-naming"), its convention rules, and an optional
// naming LEXICON that extends the engine's built-in rail/ground/feedback role vocabularies.
type Config struct {
	Name    string       `yaml:"name"`
	Lexicon *Lexicon     `yaml:"lexicon"`
	Rules   []RuleConfig `yaml:"rules"`
}

// Lexicon overrides the engine's built-in role-name vocabularies (WS3-069): the regex sets that decide
// whether a net name is a power rail, a ground, or a regulator feedback node. A project declares its
// house naming here instead of being stuck with the built-in literals.
type Lexicon struct {
	Rail      VocabConfig            `yaml:"rail"`
	Ground    VocabConfig            `yaml:"ground"`
	Feedback  VocabConfig            `yaml:"feedback"`
	SupplyPin VocabConfig            `yaml:"supply_pin"` // a component's power-supply INPUT pin names (WS3-072)
	Class     map[string]VocabConfig `yaml:"class"`      // component-class name (e.g. "tvs") -> patterns
}

// VocabConfig is one vocabulary override: Patterns are RE2 (case-insensitive, matched on the hierarchy
// leaf), merged onto the built-in set unless Replace is set, in which case they become the whole set.
type VocabConfig struct {
	Patterns []string `yaml:"patterns"`
	Replace  bool     `yaml:"replace"`
}

// RuleConfig is one convention rule. A net name FIRES when it matches none of the Allow
// patterns; names matching any Exempt pattern are skipped entirely. Patterns are RE2 and
// UNANCHORED (write ^...$ for whole-name matches). Reader-synthesized stub names (N$,
// unconnected-(...), Net-(...)) and empty names are always exempt. Patterns match the
// LEAF of a hierarchy-qualified name ("/amp1/SIG" -> "SIG") unless MatchFull is set —
// qualification is the reader's scoping, not the author's spelling.
type RuleConfig struct {
	Name      string   `yaml:"name"`
	Severity  string   `yaml:"severity"` // error | warning | info; default warning
	Why       string   `yaml:"why"`      // one line of intent, shown in the rule prose
	Allow     []string `yaml:"allow"`
	Exempt    []string `yaml:"exempt"`
	MatchFull bool     `yaml:"match_full"`
}

// Load reads and parses a convention YAML file.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	return Parse(b)
}

// Parse decodes a convention config; strict decoding, so a typo'd key fails loudly.
func Parse(b []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("naming config: %w", err)
	}
	return cfg, nil
}

// alwaysExempt matches names no convention governs: tool-synthesized stubs — netgraph's
// N$<n>, the no-connect marker vocabulary, KiCad's Net-() pad stubs, and the $-prefixed
// auto-names EDIF exporters (Mentor/OrCAD) give unlabeled nets. Conventions govern names
// an author chose.
const alwaysExempt = `^(N\$|unconnected-\(|Net-\(|\$)`

// ApplyLexicon installs the config's naming-lexicon overrides as the process-level role vocabulary
// (WS3-069). A nil Lexicon is a no-op (defaults stay). Each regex is validated here; a bad pattern is a
// returned error, not a panic, because config is operator input. Call it once at startup, before any
// rule runs. It is idempotent for a given config.
func ApplyLexicon(cfg Config) error {
	if cfg.Lexicon == nil {
		return nil
	}
	vp := func(vc VocabConfig) check.VocabPatterns {
		return check.VocabPatterns{Patterns: vc.Patterns, Replace: vc.Replace}
	}
	v, err := check.BuildRoleVocab(vp(cfg.Lexicon.Rail), vp(cfg.Lexicon.Ground), vp(cfg.Lexicon.Feedback), vp(cfg.Lexicon.SupplyPin))
	if err != nil {
		return fmt.Errorf("naming config %q lexicon: %w", cfg.Name, err)
	}
	check.SetActiveRoleVocab(v)

	if len(cfg.Lexicon.Class) > 0 {
		overrides := map[check.ComponentClass]check.VocabPatterns{}
		for name, vc := range cfg.Lexicon.Class {
			cl, ok := check.ParseComponentClass(name)
			if !ok {
				return fmt.Errorf("naming config %q lexicon: unknown component class %q", cfg.Name, name)
			}
			overrides[cl] = vp(vc)
		}
		cv, err := check.BuildClassVocab(overrides)
		if err != nil {
			return fmt.Errorf("naming config %q lexicon class: %w", cfg.Name, err)
		}
		check.SetActiveClassVocab(cv)
	}
	return nil
}

// Source compiles the config into a named RuleSource. Every regex is validated here (an
// error, not the bind-time panic Spec.Rule reserves for programmer mistakes) because
// config is operator input.
func Source(cfg Config) (check.RuleSource, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("naming config: name is required (it is the catalog namespace)")
	}
	if len(cfg.Rules) == 0 {
		return nil, fmt.Errorf("naming config %q: no rules", cfg.Name)
	}
	var rules []*check.Rule
	for _, rc := range cfg.Rules {
		r, err := compile(rc)
		if err != nil {
			return nil, fmt.Errorf("naming config %q, rule %q: %w", cfg.Name, rc.Name, err)
		}
		rules = append(rules, r)
	}
	return check.NewSource(cfg.Name, rules), nil
}

func compile(rc RuleConfig) (*check.Rule, error) {
	if rc.Name == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	severity := rc.Severity
	if severity == "" {
		severity = "warning"
	}
	switch severity {
	case "error", "warning", "info":
	default:
		return nil, fmt.Errorf("severity %q (want error, warning, or info)", severity)
	}
	if len(rc.Allow) == 0 {
		return nil, fmt.Errorf("at least one allow pattern is required")
	}
	for _, p := range append(append([]string{}, rc.Allow...), rc.Exempt...) {
		if _, err := regexp.Compile(p); err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
	}

	fact := "net.name_leaf"
	if rc.MatchFull {
		fact = "net.names"
	}
	matchAny := func(patterns []string) check.Expr {
		xs := make([]check.Expr, len(patterns))
		for i, p := range patterns {
			xs[i] = check.Match{T: check.Fact{Name: fact}, Pattern: p}
		}
		return check.Or{Xs: xs}
	}
	exempt := []check.Expr{
		check.Cmp{L: check.Fact{Name: "net.names"}, Op: "==", R: check.Lit{V: ""}},
		check.Match{T: check.Fact{Name: "net.names"}, Pattern: alwaysExempt},
	}
	if len(rc.Exempt) > 0 {
		exempt = append(exempt, matchAny(rc.Exempt))
	}
	spec := &check.Spec{
		Over: "nets",
		Where: check.And{Xs: []check.Expr{
			check.Not{X: check.Or{Xs: exempt}},
			check.Not{X: matchAny(rc.Allow)},
		}},
		Message: "net name matches no allowed naming pattern",
	}
	why := rc.Why
	if why == "" {
		why = "operator-supplied naming convention"
	}
	return spec.Rule(check.Rule{
		Name:     rc.Name,
		Severity: severity,
		Summary:  "Net name violates the project naming convention.",
		Impact:   why,
		Tags: map[string]string{
			"category":     "naming",
			"tier":         "R",
			"distribution": "config",
		},
		Detail: detail(rc, why),
	}), nil
}

func detail(rc RuleConfig, why string) string {
	s := "## " + rc.Name + "\n\n**What it means.** " + why + "\n\n**Allowed patterns** (a conforming name matches at least one):\n"
	for _, p := range rc.Allow {
		s += "\n    " + p
	}
	if len(rc.Exempt) > 0 {
		s += "\n\n**Exempt patterns** (never checked):\n"
		for _, p := range rc.Exempt {
			s += "\n    " + p
		}
	}
	s += "\n\nTool-synthesized stub names (N$..., unconnected-(...), Net-(...), $-prefixed EDIF auto-names) and empty names are always exempt. Patterns are unanchored RE2 and match the "
	if rc.MatchFull {
		s += "FULL net name."
	} else {
		s += "LEAF of a hierarchy-qualified name (\"/amp1/SIG\" checks \"SIG\")."
	}
	s += "\n\nThis rule is compiled from an operator convention config (check/naming), not built in.\n\nReads: net.names. Tier R."
	return s
}
