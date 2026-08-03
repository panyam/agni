package check

import (
	"testing"

	"github.com/panyam/agni/param"
)

// Every registered rule must declare the facts it Reads (Available derives from them) and set the
// well-known category tag Agni's catalog groups by. A rule added without these silently gates as
// always-available and falls out of the default group-by, so guard the zero value rather than
// trusting authors to remember.
func TestEveryRuleHasReadsAndCategory(t *testing.T) {
	for _, r := range Rules {
		if len(r.Reads) == 0 {
			t.Errorf("rule %q has empty Reads", r.Name)
		}
		if r.Tags[KeyCategory] == "" {
			t.Errorf("rule %q has no %q tag", r.Name, KeyCategory)
		}
	}
}

func TestAvailableFromReads(t *testing.T) {
	if ok, reason := Available(&Rule{Reads: []string{"net.pin_count", "on_net"}}, nil); !ok || reason != "" {
		t.Errorf("topology-only rule: got (%v, %q), want (true, \"\")", ok, reason)
	}
	if ok, reason := Available(&Rule{Reads: []string{"net.names", "param(mpn, max_voltage)"}}, nil); ok || reason == "" {
		t.Errorf("datasheet-reading rule: got (%v, %q), want (false, non-empty)", ok, reason)
	}
	// A datasheet rule IS applicable once a params tier is attached: the earlier gate returned
	// not-applicable unconditionally, so a seeded ask could never pass/fail in a review even with
	// --params. m == nil (catalog listing) and a params-less model still gate.
	pr := &Rule{Reads: []string{"param.supply_abs_max"}}
	seeded := NewModelWithParams(supplyDesign("+5V", false, "ACME-33"), nil, param.ParamSet{})
	if ok, _ := Available(pr, seeded); !ok {
		t.Error("datasheet rule with a params tier attached: want available")
	}
	if ok, _ := Available(pr, NewModel(supplyDesign("+5V", false, "ACME-33"))); ok {
		t.Error("datasheet rule on a params-less model: want not-available")
	}
}

func TestFilter(t *testing.T) {
	names := func(rs []*Rule) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Name
		}
		return out
	}

	if got := len(Filter(Rules, Facets{})); got != len(Rules) {
		t.Errorf("empty Facets returned %d rules, want all %d", got, len(Rules))
	}

	byCat := Filter(Rules, Facets{Tags: map[string][]string{KeyCategory: {CategoryNaming}}})
	for _, r := range byCat {
		if r.Tags[KeyCategory] != CategoryNaming {
			t.Errorf("category filter leaked rule %q in category %q", r.Name, r.Tags[KeyCategory])
		}
	}
	if len(byCat) == 0 {
		t.Errorf("category filter for %q returned nothing", CategoryNaming)
	}

	byName := Filter(Rules, Facets{Names: []string{"single-pin-net"}})
	if got := names(byName); len(got) != 1 || got[0] != "single-pin-net" {
		t.Errorf("name filter returned %v, want [single-pin-net]", got)
	}

	// Name and tag axes intersect: a real name in a non-matching category yields nothing.
	crossed := Filter(Rules, Facets{
		Names: []string{"single-pin-net"},
		Tags:  map[string][]string{KeyCategory: {CategoryNaming}},
	})
	if len(crossed) != 0 {
		t.Errorf("intersecting name+category returned %v, want none", names(crossed))
	}

	if got := Filter(Rules, Facets{Names: []string{"no-such-rule"}}); len(got) != 0 {
		t.Errorf("unknown name returned %d rules, want 0", len(got))
	}

	// An unknown tag key constrains to nothing (no rule has that key), so it selects none.
	if got := Filter(Rules, Facets{Tags: map[string][]string{"no-such-key": {"x"}}}); len(got) != 0 {
		t.Errorf("unknown tag key returned %d rules, want 0", len(got))
	}
}

// testRule builds a minimal named rule for source-composition tests.
func testRule(name string) *Rule {
	return &Rule{
		Name: name, Severity: "info", Summary: "t", Reads: []string{"net.names"},
		Tags: map[string]string{KeyCategory: CategoryNaming},
		Eval: func(Model) []Finding { return nil },
	}
}

func TestCatalogComposition(t *testing.T) {
	suite := []*Rule{testRule("ctrs-naming"), testRule("single-pin-net")} // second collides with a built-in bare name
	c, err := NewCatalog(Builtins, NewSource("tesla", suite))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got, want := len(c.Rules()), len(Rules)+2; got != want {
		t.Fatalf("composed %d rules, want %d", got, want)
	}
	// Built-ins pass through bare and untouched: same pointers, no source tag.
	if c.Rules()[0] != Rules[0] || c.Rules()[0].Tags[KeySource] != "" {
		t.Errorf("built-in rule mutated by composition: %+v", c.Rules()[0])
	}
	// The suite's rules are prefixed copies with the source stamped; the ORIGINALS are not mutated.
	r := c.Lookup("tesla/ctrs-naming")
	if r == nil || r.Tags[KeySource] != "tesla" {
		t.Fatalf("tesla/ctrs-naming = %+v", r)
	}
	if suite[0].Name != "ctrs-naming" || suite[0].Tags[KeySource] != "" {
		t.Errorf("source's own rule was mutated: %+v", suite[0])
	}
	// Prefixing makes the built-in collision a non-collision.
	if c.Lookup("tesla/single-pin-net") == nil || c.Lookup("single-pin-net") == nil {
		t.Error("prefixed and bare single-pin-net should coexist")
	}
	// The source tag is an ordinary facet.
	if got := c.Filter(Facets{Tags: map[string][]string{KeySource: {"tesla"}}}); len(got) != 2 {
		t.Errorf("source facet selected %d rules, want 2", len(got))
	}
}

func TestCatalogRejections(t *testing.T) {
	cases := []struct {
		name    string
		sources []RuleSource
	}{
		{"duplicate source name", []RuleSource{NewSource("tesla", nil), NewSource("tesla", nil)}},
		{"second anonymous source", []RuleSource{Builtins, NewSource("", nil)}},
		{"bad source name", []RuleSource{NewSource("Tesla Rules", nil)}},
		{"separator in rule name", []RuleSource{NewSource("tesla", []*Rule{testRule("a/b")})}},
		{"duplicate rule in source", []RuleSource{NewSource("tesla", []*Rule{testRule("x"), testRule("x")})}},
	}
	for _, tc := range cases {
		if _, err := NewCatalog(tc.sources...); err == nil {
			t.Errorf("%s: NewCatalog accepted an invalid composition", tc.name)
		}
	}
}
