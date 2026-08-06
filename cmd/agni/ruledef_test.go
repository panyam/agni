package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/stdlib/profiles"
	"github.com/panyam/agni/stdlib/ruledef"
	"github.com/panyam/agni/stdlib/rules/datalog"
)

// deckOf wraps definitions in a one-off deck, the unit Marshal/Parse operate on.
func deckOf(defs ...*checkspb.RuleDef) *checkspb.RuleDeck {
	return &checkspb.RuleDeck{Name: "roundtrip", Source: "test", Rules: defs}
}

// queryRule compiles a declaration the way the shipping package does, giving the comparison baseline.
func queryRule(fq query.FindingQuery) *check.Rule { return query.RuleFromQuery(fq) }

// conformanceModels loads every conformance fixture into a Model, so a rule can be run against the
// whole fixture corpus rather than one hand-picked design. It lives at the CLI edge for the same
// reason TestConformance does: the reader is then in the loop, and a rule is judged on what it does to
// real parsed designs.
func conformanceModels(t *testing.T) map[string]check.Model {
	t.Helper()
	sidecars, err := filepath.Glob(filepath.Join("testdata", "conformance", "*.expect.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sidecars) == 0 {
		t.Fatal("no conformance fixtures found")
	}
	out := map[string]check.Model{}
	for _, sc := range sidecars {
		fixture := strings.TrimSuffix(sc, ".expect.yaml")
		m, err := readModelWithParams(fixture, filepath.Join("testdata", "conformance", "params"))
		if err != nil {
			t.Fatalf("readModel %s: %v", fixture, err)
		}
		out[filepath.Base(fixture)] = m
	}
	return out
}

// assertSameFindings runs two rules over every fixture and requires identical findings.
func assertSameFindings(t *testing.T, name string, want, got *check.Rule, models map[string]check.Model) {
	t.Helper()
	for fixture, m := range models {
		w := check.Run(m, []*check.Rule{want})
		g := check.Run(m, []*check.Rule{got})
		if !reflect.DeepEqual(w, g) {
			t.Errorf("%s on %s: findings differ after a round trip\n original: %+v\n decoded:  %+v", name, fixture, w, g)
		}
	}
}

// TestSpecRulesRoundTripThroughTheDefinitionContract is the acceptance test for the rule-definition
// half of the checks contract (WS3-103), over the spec-authored rules.
//
// Every shipped Spec is encoded, marshalled to bytes, read back, compiled, and run against every
// conformance fixture; the findings must match exactly. Comparing FINDINGS rather than the decoded AST
// is the point: two Specs can differ structurally and behave identically, and, far more importantly,
// they can look identical and behave differently — a literal decoded as the wrong numeric type
// silently makes every ordering comparison false, which no structural equality check would notice.
//
// The baseline is the spec bound through Spec.Rule, NOT the shipped built-in of the same name. Most
// built-ins are Go-authored with hand-written Reads and Primitives, and their Spec is a declarative
// TWIN whose metadata is DERIVED from its body (and therefore sorted). TestSpecParity already holds
// the two forms to the same findings; conflating them here would make this test fail over a
// difference that is deliberate and has nothing to do with a round trip.
func TestSpecRulesRoundTripThroughTheDefinitionContract(t *testing.T) {
	models := conformanceModels(t)
	specs := check.BuiltinSpecs()
	if len(specs) == 0 {
		t.Fatal("no built-in specs; the twin registry is empty")
	}
	byName := map[string]*check.Rule{}
	for _, r := range check.BuiltinRules() {
		byName[r.Name] = r
	}
	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			meta := byName[name]
			if meta == nil {
				t.Fatalf("spec %q has no rule in the built-in catalog", name)
			}
			original := spec.Rule(*meta)
			b, err := ruledef.Marshal(deckOf(ruledef.SpecDef(*meta, *spec)))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			read, err := ruledef.Parse(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			rules, err := ruledef.CompileDeck(read)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if len(rules) != 1 {
				t.Fatalf("a spec definition compiled to %d rules, want 1", len(rules))
			}
			assertMetaSurvives(t, original, rules[0])
			assertSameFindings(t, name, original, rules[0], models)
		})
	}
}

// assertMetaSurvives pins the fields a compiled rule needs to behave the same in a catalog: identity,
// severity, the declared gates, and the derived reads. Reads and Primitives are NOT serialized — they
// are derived by the compiler from the body — so their surviving the trip is evidence that the body
// itself came through intact.
func assertMetaSurvives(t *testing.T, want, got *check.Rule) {
	t.Helper()
	if got.Name != want.Name || got.Severity != want.Severity || got.Summary != want.Summary {
		t.Errorf("identity changed: got %q/%q, want %q/%q", got.Name, got.Severity, want.Name, want.Severity)
	}
	if !reflect.DeepEqual(got.Reads, want.Reads) {
		t.Errorf("derived Reads changed: got %v, want %v", got.Reads, want.Reads)
	}
	if !reflect.DeepEqual(got.Primitives, want.Primitives) {
		t.Errorf("derived Primitives changed: got %v, want %v", got.Primitives, want.Primitives)
	}
	if !reflect.DeepEqual(got.OptionalReads, want.OptionalReads) {
		t.Errorf("OptionalReads changed: got %v, want %v", got.OptionalReads, want.OptionalReads)
	}
	if !reflect.DeepEqual(got.RequiresCapability, want.RequiresCapability) {
		t.Errorf("RequiresCapability changed: got %v, want %v", got.RequiresCapability, want.RequiresCapability)
	}
	if !reflect.DeepEqual(got.Tags, want.Tags) {
		t.Errorf("Tags changed: got %v, want %v", got.Tags, want.Tags)
	}
}

// TestDatalogRulesRoundTripThroughTheDefinitionContract is the same acceptance test over the
// datalog-authored rules. It carries the AST rather than the query text on purpose, so this also pins
// that the AST survives: a program whose negation, aggregate, or numeric constant were dropped would
// still parse and still run, and would simply report less.
func TestDatalogRulesRoundTripThroughTheDefinitionContract(t *testing.T) {
	models := conformanceModels(t)
	queries := datalog.Queries()
	if len(queries) == 0 {
		t.Fatal("no datalog rule declarations")
	}
	for _, fq := range queries {
		t.Run(fq.Rule.Name, func(t *testing.T) {
			original := queryRule(fq)
			b, err := ruledef.Marshal(deckOf(ruledef.QueryDef(fq)))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			read, err := ruledef.Parse(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			rules, err := ruledef.CompileDeck(read)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if len(rules) != 1 {
				t.Fatalf("a query definition compiled to %d rules, want 1", len(rules))
			}
			assertSameFindings(t, fq.Rule.Name, original, rules[0], models)
		})
	}
}

// TestProfilesRoundTripThroughTheDefinitionContract covers the third source. A profile is the one
// definition that compiles to MORE than one rule, so this asserts the rule SET matches by name before
// comparing each rule's findings — a trip that silently dropped a requirement would otherwise look
// clean, and a dropped requirement is a check that no longer exists.
func TestProfilesRoundTripThroughTheDefinitionContract(t *testing.T) {
	models := conformanceModels(t)
	if len(profiles.Profiles) == 0 {
		t.Fatal("no built-in profiles")
	}
	for _, p := range profiles.Profiles {
		t.Run(p.Name, func(t *testing.T) {
			original := profiles.Compile(p)
			b, err := ruledef.Marshal(deckOf(ruledef.ProfileDef(p)))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			read, err := ruledef.Parse(b)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, err := ruledef.CompileDeck(read)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if len(got) != len(original) {
				t.Fatalf("profile compiled to %d rules after the round trip, want %d", len(got), len(original))
			}
			for i := range original {
				if got[i].Name != original[i].Name {
					t.Fatalf("rule #%d is %q, want %q", i, got[i].Name, original[i].Name)
				}
				assertSameFindings(t, got[i].Name, original[i], got[i], models)
			}
		})
	}
}
