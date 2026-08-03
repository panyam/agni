package builtin

import (
	"testing"

	"github.com/panyam/agni/core/check"
)

// Every registered built-in rule must declare the facts it Reads (Available derives from them) and
// set the well-known category tag the catalog groups by. A rule added without these silently gates
// as always-available and falls out of the default group-by, so guard the zero value rather than
// trusting authors to remember.
func TestEveryRuleHasReadsAndCategory(t *testing.T) {
	for _, r := range rules {
		if len(r.Reads) == 0 {
			t.Errorf("rule %q has empty Reads", r.Name)
		}
		if r.Tags[check.KeyCategory] == "" {
			t.Errorf("rule %q has no %q tag", r.Name, check.KeyCategory)
		}
	}
}

func TestFilter(t *testing.T) {
	names := func(rs []*check.Rule) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.Name
		}
		return out
	}

	if got := len(check.Filter(rules, check.Facets{})); got != len(rules) {
		t.Errorf("empty Facets returned %d rules, want all %d", got, len(rules))
	}

	byCat := check.Filter(rules, check.Facets{Tags: map[string][]string{check.KeyCategory: {check.CategoryNaming}}})
	for _, r := range byCat {
		if r.Tags[check.KeyCategory] != check.CategoryNaming {
			t.Errorf("category filter leaked rule %q in category %q", r.Name, r.Tags[check.KeyCategory])
		}
	}
	if len(byCat) == 0 {
		t.Errorf("category filter for %q returned nothing", check.CategoryNaming)
	}

	byName := check.Filter(rules, check.Facets{Names: []string{"single-pin-net"}})
	if got := names(byName); len(got) != 1 || got[0] != "single-pin-net" {
		t.Errorf("name filter returned %v, want [single-pin-net]", got)
	}

	// Name and tag axes intersect: a real name in a non-matching category yields nothing.
	crossed := check.Filter(rules, check.Facets{
		Names: []string{"single-pin-net"},
		Tags:  map[string][]string{check.KeyCategory: {check.CategoryNaming}},
	})
	if len(crossed) != 0 {
		t.Errorf("intersecting name+category returned %v, want none", names(crossed))
	}

	if got := check.Filter(rules, check.Facets{Names: []string{"no-such-rule"}}); len(got) != 0 {
		t.Errorf("unknown name returned %d rules, want 0", len(got))
	}

	// An unknown tag key constrains to nothing (no rule has that key), so it selects none.
	if got := check.Filter(rules, check.Facets{Tags: map[string][]string{"no-such-key": {"x"}}}); len(got) != 0 {
		t.Errorf("unknown tag key returned %d rules, want 0", len(got))
	}
}

// testRule builds a minimal named rule for source-composition tests.
func testRule(name string) *check.Rule {
	return &check.Rule{
		Name: name, Severity: "info", Summary: "t", Reads: []string{"net.names"},
		Tags: map[string]string{check.KeyCategory: check.CategoryNaming},
		Eval: func(check.Model) []check.Finding { return nil },
	}
}

func TestCatalogComposition(t *testing.T) {
	suite := []*check.Rule{testRule("ctrs-naming"), testRule("single-pin-net")} // second collides with a built-in bare name
	c, err := check.NewCatalog(check.Builtins, check.NewSource("tesla", suite))
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	if got, want := len(c.Rules()), len(rules)+2; got != want {
		t.Fatalf("composed %d rules, want %d", got, want)
	}
	// Built-ins pass through bare and untouched: same pointers, no source tag.
	if c.Rules()[0] != rules[0] || c.Rules()[0].Tags[check.KeySource] != "" {
		t.Errorf("built-in rule mutated by composition: %+v", c.Rules()[0])
	}
	// The suite's rules are prefixed copies with the source stamped; the ORIGINALS are not mutated.
	r := c.Lookup("tesla/ctrs-naming")
	if r == nil || r.Tags[check.KeySource] != "tesla" {
		t.Fatalf("tesla/ctrs-naming = %+v", r)
	}
	if suite[0].Name != "ctrs-naming" || suite[0].Tags[check.KeySource] != "" {
		t.Errorf("source's own rule was mutated: %+v", suite[0])
	}
	// Prefixing makes the built-in collision a non-collision.
	if c.Lookup("tesla/single-pin-net") == nil || c.Lookup("single-pin-net") == nil {
		t.Error("prefixed and bare single-pin-net should coexist")
	}
	// The source tag is an ordinary facet.
	if got := c.Filter(check.Facets{Tags: map[string][]string{check.KeySource: {"tesla"}}}); len(got) != 2 {
		t.Errorf("source facet selected %d rules, want 2", len(got))
	}
}
