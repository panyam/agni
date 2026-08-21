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
//
// The config TYPE is the generated agni.v1.config.NamingConvention, not a struct declared here.
// There used to be both: this package held a yaml-tagged twin and internal/service converted it to
// the wire message on every path. The two drifted, the wire form never grew the transistor terminal
// vocabularies, and a project declaring them had them dropped in silence. One schema is what stops
// that recurring, and agni.v1.config exists (rather than the message living in webapi) so the engine
// can depend on it without importing the web request tier (C17).
//
// YAML remains the authoring syntax and carries no schema of its own: Parse converts it to JSON and
// lets protojson bind it to the message.
package naming

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"google.golang.org/protobuf/encoding/protojson"
	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/check"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
)

// Load reads and parses a convention YAML file.
func Load(path string) (*configpb.NamingConvention, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse decodes a convention config.
//
// YAML has no protobuf binding, so it is converted to JSON and bound by protojson. That keeps the
// generated message as the single schema while an operator still writes (and comments) YAML. Decoding
// stays STRICT, because protojson rejects an unknown field by default the same way the old
// yaml.KnownFields(true) did: a typo'd key fails loudly instead of silently configuring nothing.
func Parse(b []byte) (*configpb.NamingConvention, error) {
	var tree any
	if err := yaml.Unmarshal(b, &tree); err != nil {
		return nil, fmt.Errorf("naming config: %w", err)
	}
	if tree == nil {
		return &configpb.NamingConvention{}, nil
	}
	j, err := json.Marshal(tree)
	if err != nil {
		return nil, fmt.Errorf("naming config: %w", err)
	}
	var cfg configpb.NamingConvention
	if err := protojson.Unmarshal(j, &cfg); err != nil {
		return nil, fmt.Errorf("naming config: %w", err)
	}
	return &cfg, nil
}

const alwaysExempt = `^(N\$|unconnected-\(|Net-\(|\$)`

// BuildLexicon compiles the config's naming-lexicon block into a lexicon VALUE (WS3-106). A nil
// Lexicon block yields nil, meaning "this config states no vocabulary", which the caller reads as the
// engine defaults. Each regex is validated here; a bad pattern is a returned error, not a panic,
// because config is operator input.
//
// This is the form a per-request caller wants: the value travels with the read it configures, so two
// designs can be read with different project conventions in one process. ApplyLexicon is the same
// build followed by a process-wide install, kept for the startup-config callers.
func BuildLexicon(cfg *configpb.NamingConvention) (*check.Lexicon, error) {
	lx := cfg.GetLexicon()
	if lx == nil {
		return nil, nil
	}
	vp := func(v *configpb.VocabPatterns) check.VocabPatterns {
		return check.VocabPatterns{Patterns: v.GetPatterns(), Replace: v.GetReplace()}
	}
	net, pin := lx.GetNet(), lx.GetPin()
	v, err := check.BuildRoleVocab(check.RoleVocabConfig{
		Rail:      vp(net.GetRail()),
		Ground:    vp(net.GetGround()),
		Feedback:  vp(net.GetFeedback()),
		SupplyPin: vp(pin.GetSupply()),
		Gate:      vp(pin.GetGate()),
		Source:    vp(pin.GetSource()),
		Drain:     vp(pin.GetDrain()),
	})
	if err != nil {
		return nil, fmt.Errorf("naming config %q lexicon: %w", cfg.GetName(), err)
	}
	lex := &check.Lexicon{Role: v}
	if cls := lx.GetClass(); len(cls) > 0 {
		overrides := map[check.ComponentClass]check.VocabPatterns{}
		for name, v := range cls {
			cl, ok := check.ParseComponentClass(name)
			if !ok {
				return nil, fmt.Errorf("naming config %q lexicon: unknown component class %q", cfg.GetName(), name)
			}
			overrides[cl] = vp(v)
		}
		cv, err := check.BuildClassVocab(overrides)
		if err != nil {
			return nil, fmt.Errorf("naming config %q lexicon class: %w", cfg.GetName(), err)
		}
		lex.Class = cv
	}
	return lex, nil
}

// ApplyLexicon installs the config's naming-lexicon overrides as the PROCESS-level vocabulary
// (WS3-069). A nil Lexicon is a no-op (defaults stay). Call it once at startup, before any design is
// read. It is idempotent for a given config.
//
// Prefer BuildLexicon where the vocabulary can travel with the read: a process-wide install cannot be
// scoped to one request, so it is startup config only.
func ApplyLexicon(cfg *configpb.NamingConvention) error {
	lex, err := BuildLexicon(cfg)
	if err != nil || lex == nil {
		return err
	}
	check.SetActiveRoleVocab(lex.Role)
	if lex.Class != nil {
		check.SetActiveClassVocab(lex.Class)
	}
	return nil
}

// Source compiles the config into a named RuleSource. Every regex is validated here (an
// error, not the bind-time panic Spec.Rule reserves for programmer mistakes) because
// config is operator input.
func Source(cfg *configpb.NamingConvention) (check.RuleSource, error) {
	if cfg.GetName() == "" {
		return nil, fmt.Errorf("naming config: name is required (it is the catalog namespace)")
	}
	if len(cfg.GetRules()) == 0 {
		return nil, fmt.Errorf("naming config %q: no rules", cfg.GetName())
	}
	var rules []*check.Rule
	for _, rc := range cfg.GetRules() {
		r, err := compile(rc)
		if err != nil {
			return nil, fmt.Errorf("naming config %q, rule %q: %w", cfg.GetName(), rc.GetName(), err)
		}
		rules = append(rules, r)
	}
	return check.NewSource(cfg.GetName(), rules), nil
}

func compile(rc *configpb.NamingRule) (*check.Rule, error) {
	if rc.GetName() == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	severity := rc.GetSeverity()
	if severity == "" {
		severity = "warning"
	}
	switch severity {
	case "error", "warning", "info":
	default:
		return nil, fmt.Errorf("severity %q (want error, warning, or info)", severity)
	}
	if len(rc.GetAllow()) == 0 {
		return nil, fmt.Errorf("at least one allow pattern is required")
	}
	for _, p := range append(append([]string{}, rc.GetAllow()...), rc.GetExempt()...) {
		if _, err := regexp.Compile(p); err != nil {
			return nil, fmt.Errorf("pattern %q: %w", p, err)
		}
	}

	fact := "net.name_leaf"
	if rc.GetMatchFull() {
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
	if len(rc.GetExempt()) > 0 {
		exempt = append(exempt, matchAny(rc.GetExempt()))
	}
	spec := &check.Spec{
		Over: "nets",
		Where: check.And{Xs: []check.Expr{
			check.Not{X: check.Or{Xs: exempt}},
			check.Not{X: matchAny(rc.GetAllow())},
		}},
		Message: "net name matches no allowed naming pattern",
	}
	why := rc.GetWhy()
	if why == "" {
		why = "operator-supplied naming convention"
	}
	return spec.Rule(check.Rule{
		Name:     rc.GetName(),
		Severity: severity,
		Summary:  "Net name violates the project naming convention.",
		Impact:   why,
		Remedy:   "Rename the net to match one of the convention's allowed patterns, or amend the convention if the name is right and the pattern list has not kept up.",
		Tags: map[string]string{
			"category":     "naming",
			"tier":         "R",
			"distribution": "config",
		},
		Detail: detail(rc, why),
	}), nil
}

func detail(rc *configpb.NamingRule, why string) string {
	s := "## " + rc.GetName() + "\n\n**What it means.** " + why + "\n\n**Allowed patterns** (a conforming name matches at least one):\n"
	for _, p := range rc.GetAllow() {
		s += "\n    " + p
	}
	if len(rc.GetExempt()) > 0 {
		s += "\n\n**Exempt patterns** (never checked):\n"
		for _, p := range rc.GetExempt() {
			s += "\n    " + p
		}
	}
	s += "\n\nTool-synthesized stub names (N$..., unconnected-(...), Net-(...), $-prefixed EDIF auto-names) and empty names are always exempt. Patterns are unanchored RE2 and match the "
	if rc.GetMatchFull() {
		s += "FULL net name."
	} else {
		s += "LEAF of a hierarchy-qualified name (\"/amp1/SIG\" checks \"SIG\")."
	}
	s += "\n\nThis rule is compiled from an operator convention config (check/naming), not built in.\n\nReads: net.names. Tier R."
	return s
}
