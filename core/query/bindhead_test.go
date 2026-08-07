package query

import (
	"strings"
	"testing"
)

// TestGeneratorFirstRules covers the WS3-114 lint. The first two cases are the ACTUAL rule bodies from
// before and after the fix, so the test proves the guard catches the real regression rather than a
// shape invented to match the guard.
func TestGeneratorFirstRules(t *testing.T) {
	for _, c := range []struct {
		name string
		text string
		want []string
	}{{
		name: "the regression: esd_ok opened with an unbound reaches",
		text: `esd_ok(?n) :- reaches(?n, ?rn, ?h), component-on-net(?t, ?rn), component.class(?t, "tvs"); esd_ok(?n) => ?n`,
		want: []string{"esd_ok"},
	}, {
		name: "the fix: the guard binds the start net before the walk",
		text: `esd_ok(?n) :- needs_esd(?n), reaches(?n, ?rn, ?h), component-on-net(?t, ?rn); esd_ok(?n) => ?n`,
		want: nil,
	}, {
		name: "a constant start is a single walk, not a scan of the board",
		text: `near(?n) :- reaches("VBUS", ?n, ?h); near(?n) => ?n`,
		want: nil,
	}, {
		name: "the 2-arity reaches has the same hazard",
		text: `far(?n) :- reaches(?a, ?n), rail(?n); far(?n) => ?n`,
		want: []string{"far"},
	}, {
		name: "a filter cannot enumerate, so leading with one is not this bug",
		text: `named(?n) :- rail(?n), prefix(?n, "V"); named(?n) => ?n`,
		want: nil,
	}} {
		t.Run(c.name, func(t *testing.T) {
			q, err := Parse(c.text)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := GeneratorFirstRules(q); strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("GeneratorFirstRules = %v, want %v", got, c.want)
			}
		})
	}
}

// TestGeneratorFirstRulesMissesJoinOrder pins what this lint does NOT do, so nobody reads a clean run
// as "the join orders are fine". `pulled` opened with component-on-net with both variables unbound, a
// full EDB scan re-entered per survivor. That was 21s on a real board, not forever, and no syntactic
// check distinguishes it from a legitimate small-relation lead — that needs a cost-based planner
// (WS3-031). Recording it here so the gap is documented where someone would look for it.
func TestGeneratorFirstRulesMissesJoinOrder(t *testing.T) {
	q, err := Parse(`pulled(?n) :- component-on-net(?pu, ?n), component.class(?pu, "resistor"), component-on-net(?pu, ?rail); pulled(?n) => ?n`)
	if err != nil {
		t.Fatal(err)
	}
	if got := GeneratorFirstRules(q); len(got) != 0 {
		t.Errorf("GeneratorFirstRules = %v, want empty: this lint deliberately does not judge join order", got)
	}
}
