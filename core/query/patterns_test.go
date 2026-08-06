package query

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// patternDesign carries net names whose ROLE suffix is shared across buses (_H) and whose identity is
// the prefix — the naming affix matching cannot discriminate (WS3-057).
func patternDesign() check.Model {
	return check.NewModel(&ir.Design{
		Components: []*ir.Component{{RefDes: "U1", Prov: &ir.Provenance{SourceFile: "d"}}},
		Nets: []*ir.Net{
			{Name: "ETH_SW1_P1_A_H", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "1"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "ETH_SW2_P4_A_H", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "2"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "CAN_00_H", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "3"}}, Prov: &ir.Provenance{SourceFile: "d"}},
			{Name: "/amp1/DATA0", Connections: []*ir.Connection{{ComponentRef: "U1", PinRef: "4"}}, Prov: &ir.Provenance{SourceFile: "d"}},
		},
	})
}

func matched(t *testing.T, m check.Model, text string) []string {
	t.Helper()
	var out []string
	for _, r := range runQuery(t, m, text) {
		out = append(out, r.Bind["n"].S)
	}
	return out
}

// TestGlobPredicate: glob matches the WHOLE name with * as any run and ? as one character, which is
// what discriminates an ETH_ bus from a CAN_ bus that shares the bare _H suffix.
func TestGlobPredicate(t *testing.T) {
	m := patternDesign()
	cases := map[string][]string{
		`net.pin_count(?n,?c), glob(?n,"ETH_SW*_H") => ?n`:      {"ETH_SW1_P1_A_H", "ETH_SW2_P4_A_H"},
		`net.pin_count(?n,?c), glob(?n,"CAN_*_H") => ?n`:        {"CAN_00_H"},
		`net.pin_count(?n,?c), glob(?n,"ETH_SW?_P1_A_H") => ?n`: {"ETH_SW1_P1_A_H"},
		// Whole-string, so a bare role fragment matches nothing without wildcards on both sides.
		`net.pin_count(?n,?c), glob(?n,"_H") => ?n`: nil,
	}
	for text, want := range cases {
		got := matched(t, m, text)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s => %v, want %v", text, got, want)
		}
	}
}

// TestGlobStarCrossesSlash pins the reason glob is translated to a regexp rather than deferring to
// path.Match: path.Match's * stops at "/", and a hierarchical net name (a sub-sheet local) contains
// one, so path.Match would silently miss exactly the names a multi-instance bus is named with.
func TestGlobStarCrossesSlash(t *testing.T) {
	if got := matched(t, patternDesign(), `net.pin_count(?n,?c), glob(?n,"*DATA0") => ?n`); len(got) != 1 || got[0] != "/amp1/DATA0" {
		t.Errorf(`glob("*DATA0") => %v, want the hierarchical net /amp1/DATA0`, got)
	}
}

// TestMatchPredicate: match is an UNANCHORED RE2 search, so a caller who means the whole name anchors
// it. Both readings are exercised so the semantics are pinned, not incidental. Datalog string
// literals are verbatim (the parser processes no escapes), so a regex backslash is written singly.
func TestMatchPredicate(t *testing.T) {
	m := patternDesign()
	if got := matched(t, m, `net.pin_count(?n,?c), match(?n,"^ETH_SW\d+_P\d+_._H$") => ?n`); len(got) != 2 {
		t.Errorf("anchored regex => %v, want both ETH ports", got)
	}
	if got := matched(t, m, `net.pin_count(?n,?c), match(?n,"_H$") => ?n`); len(got) != 3 {
		t.Errorf("unanchored regex => %v, want every _H net including CAN's", got)
	}
	if got := matched(t, m, `net.pin_count(?n,?c), not match(?n,"^ETH_") => ?n`); len(got) != 2 {
		t.Errorf("negated regex => %v, want the two non-ETH nets", got)
	}
}

// TestPatternPredicateErrors: a malformed regex is an EVAL ERROR, not a silent non-match — a bad
// pattern that quietly matched nothing would read as "the design is clean" on a completeness check.
// Unbound and wrong-arity forms fail like every other filter.
func TestPatternPredicateErrors(t *testing.T) {
	m := patternDesign()
	for _, text := range []string{
		`net.pin_count(?n,?c), match(?n,"^ETH_(SW") => ?n`, // will not compile
		`match(?x,"^ETH_") => ?x`,                          // unbound: a filter cannot enumerate
		`net.pin_count(?n,?c), glob(?n) => ?n`,             // wrong arity
	} {
		if _, err := (Naive{}).Eval(mustParse(t, text), NewBase(m)); err == nil {
			t.Errorf("%s: want an error, got nil", text)
		}
	}
}

// TestCompileGlobTranslation pins the glob grammar itself: * and ? are the only metacharacters and
// every other character is literal, so a regex metacharacter in a net name cannot leak through as one.
func TestCompileGlobTranslation(t *testing.T) {
	re, err := CompileGlob("A+B*C?D")
	if err != nil {
		t.Fatalf("CompileGlob: %v", err)
	}
	if !re.MatchString("A+BxyzCqD") {
		t.Error(`"A+B*C?D" should match "A+BxyzCqD" (+ literal, * any run, ? one char)`)
	}
	if re.MatchString("AABxyzCqD") {
		t.Error("the + must be a literal, not a regex quantifier")
	}
}
