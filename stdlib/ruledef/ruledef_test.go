package ruledef

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
	"github.com/panyam/agni/stdlib/profiles"
	_ "github.com/panyam/agni/stdlib/relations" // registers the fact relations a query definition names
)

func deck(defs ...*checkspb.RuleDef) *checkspb.RuleDeck {
	return &checkspb.RuleDeck{Name: "t", Rules: defs}
}

func meta(name string) check.Rule {
	return check.Rule{Name: name, Severity: "warning", Summary: "s"}
}

// TestCompileRejectsWhatItCannotRun is the load-time gate, and the reason it exists is worth stating:
// every one of these definitions would otherwise compile to a rule that never fires, and a rule that
// never fires is indistinguishable from a design with nothing wrong with it. So each is an error when
// the definition is READ, not a surprise when someone later trusts a clean report.
func TestCompileRejectsWhatItCannotRun(t *testing.T) {
	cases := map[string]struct {
		def  *checkspb.RuleDef
		want string
	}{
		"unknown entity set": {
			def:  SpecDef(meta("r"), check.Spec{Over: "widgets", Message: "m"}),
			want: "unknown entity set",
		},
		"unknown fact": {
			def: SpecDef(meta("r"), check.Spec{
				Over:    "nets",
				Where:   check.Cmp{L: check.Fact{Name: "net.vibes"}, Op: "==", R: check.Lit{V: 1}},
				Message: "m",
			}),
			want: "unknown fact",
		},
		"unregistered function": {
			def: SpecDef(meta("r"), check.Spec{
				Over:    "nets",
				Where:   check.IsTrue{T: check.Call{Fn: "no_such_helper"}},
				Message: "m",
			}),
			want: "unregistered func",
		},
		"unbound var": {
			def: SpecDef(meta("r"), check.Spec{
				Over:    "nets",
				Where:   check.Cmp{L: check.Var{Name: "n"}, Op: "==", R: check.Lit{V: "x"}},
				Message: "m",
			}),
			want: "unbound var",
		},
		"unknown relation": {
			def: QueryDef(query.FindingQuery{
				Rule:       meta("r"),
				Query:      query.MustParse(`component.mnp(?r, ?m) => ?r`),
				Kind:       check.KindComponent,
				SubjectVar: "r",
				Message:    "m",
			}),
			want: "unknown relation",
		},
		"unknown requirement type": {
			def: ProfileDef(profiles.Profile{
				Name:         "X",
				Signals:      []profiles.Signal{{Name: "A", Suffix: "_A", Anchor: true}, {Name: "B", Suffix: "_B"}},
				Requirements: []profiles.Requirement{{Type: "signal-teleported"}},
			}),
			want: "unknown requirement type",
		},
		"requirement with incomplete params": {
			def: ProfileDef(profiles.Profile{
				Name:         "X",
				Signals:      []profiles.Signal{{Name: "A", Suffix: "_A", Anchor: true}, {Name: "B", Suffix: "_B"}},
				Requirements: []profiles.Requirement{{Type: "termination", Params: map[string]string{"high": "_A"}}},
			}),
			want: "termination",
		},
		"completeness requirement with no anchor": {
			def: ProfileDef(profiles.Profile{
				Name:         "X",
				Signals:      []profiles.Signal{{Name: "A", Suffix: "_A"}, {Name: "B", Suffix: "_B"}},
				Requirements: []profiles.Requirement{{Type: "signal-missing"}},
			}),
			want: "anchor",
		},
		"over-broad signal matcher": {
			def: ProfileDef(profiles.Profile{
				Name:         "X",
				Signals:      []profiles.Signal{{Name: "A", Regex: ".*", Anchor: true}, {Name: "B", Suffix: "_B"}},
				Requirements: []profiles.Requirement{{Type: "signal-missing"}},
			}),
			want: "",
		},
		"empty definition": {
			def:  &checkspb.RuleDef{},
			want: "no body set",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := CompileDeck(deck(tc.def))
			if err == nil {
				t.Fatal("compiling should fail; a rule that cannot run must not join a catalog silently")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestUnknownRelationSuggestsTheRealOne pins that the load-time rejection teaches rather than only
// refuses. A half-remembered relation name is the most common way a definition names nothing.
func TestUnknownRelationSuggestsTheRealOne(t *testing.T) {
	_, err := CompileDeck(deck(QueryDef(query.FindingQuery{
		Rule:       meta("r"),
		Query:      query.MustParse(`compnent-on-net(?r, ?n) => ?r`),
		Kind:       check.KindComponent,
		SubjectVar: "r",
		Message:    "m",
	})))
	if err == nil || !strings.Contains(err.Error(), "component-on-net") {
		t.Errorf("error = %v, want a suggestion naming component-on-net", err)
	}
}

// TestCompileDeckStopsAtTheFirstBadDefinition pins that a deck does not load partially. A catalog
// missing one rule looks exactly like a catalog that ran it and found nothing.
func TestCompileDeckStopsAtTheFirstBadDefinition(t *testing.T) {
	good := SpecDef(meta("good"), check.Spec{Over: "nets", Message: "m"})
	bad := SpecDef(meta("bad"), check.Spec{Over: "widgets", Message: "m"})
	if _, err := CompileDeck(deck(good, bad)); err == nil {
		t.Fatal("a deck with one bad definition should fail to load")
	}
	if _, err := CompileDeck(deck(bad, good)); err == nil {
		t.Fatal("order should not matter")
	}
	rules, err := CompileDeck(deck(good))
	if err != nil || len(rules) != 1 {
		t.Fatalf("a good deck should load: %v, %d rules", err, len(rules))
	}
}

// TestSourceJoinsACatalog pins the data seam: definitions read from a document become a catalog
// source exactly the way a Go-registered suite does, which is what makes a rule source a document
// rather than code linked into the binary.
func TestSourceJoinsACatalog(t *testing.T) {
	d := deck(SpecDef(meta("stub-net"), check.Spec{
		Over:    "nets",
		Where:   check.Cmp{L: check.Fact{Name: "net.pin_count"}, Op: "<", R: check.Lit{V: 2}},
		Message: "net has {net.pin_count} connection(s)",
	}))
	d.Name = "acme"
	src, err := Source(d)
	if err != nil {
		t.Fatal(err)
	}
	cat := check.CatalogWith(src)
	found := false
	for _, r := range cat.Rules() {
		if r.Name == "acme/stub-net" {
			found = true
		}
	}
	if !found {
		var names []string
		for _, r := range cat.Rules() {
			names = append(names, r.Name)
		}
		t.Errorf("acme/stub-net is not in the composed catalog; got %v", names)
	}
}

// TestMarshalParseRoundTripsADeck pins the encoding itself, separately from compiling: a deck must
// survive bytes even when it holds a definition this build could not run.
func TestMarshalParseRoundTripsADeck(t *testing.T) {
	d := deck(SpecDef(meta("r"), check.Spec{Over: "nets", Message: "m"}))
	d.Source = "somewhere.json"
	b, err := Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetName() != d.GetName() || got.GetSource() != d.GetSource() || len(got.GetRules()) != 1 {
		t.Errorf("deck changed: %+v", got)
	}
}
