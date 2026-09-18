package classify

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/model"
	configpb "github.com/panyam/agni/gen/go/agni/v1/config"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestDefaultClassVocab pins the built-in classification vocabulary: the historical tokens still map to
// their class, and matching stays whole-token (a token containing "esd" as a substring does not match).
func TestDefaultClassVocab(t *testing.T) {
	v := DefaultClassVocab()
	cases := map[string]ComponentClass{"tvs": ClassTVS, "esd": ClassTVS, "zener": ClassZener, "diode": ClassDiode,
		"ferrite": ClassFerrite, "bead": ClassFerrite, "connector": ClassConnector}
	for tok, want := range cases {
		if h := v.HintsFor([]string{tok}); !h[want] {
			t.Errorf("hintsFor(%q) missing %s", tok, want)
		}
	}
	// whole-token: "pesd2eth1gt" must NOT match the "esd" pattern (no substring matching).
	if h := v.HintsFor([]string{"pesd2eth1gt"}); h[ClassTVS] {
		t.Error("default vocab must be whole-token: pesd2eth1gt is not esd")
	}
}

// TestClassVocabConfigExtendsClassification: a project pattern added to the tvs class lets an ESD-array
// MPN family classify as TVS, where the default (no tvs/esd word) leaves it a plain diode.
func TestClassVocabConfigExtendsClassification(t *testing.T) {
	defer SetActiveClassVocab(nil)
	part := &ir.Component{RefDes: "D5", Attributes: map[string]string{"Part Name": "PESD2ETH1GT"}}
	if got := Classify(part, &ir.PartType{}); got != ClassDiode {
		t.Fatalf("default: a PESD part with no tvs/esd word stays diode, got %s", got)
	}
	cv, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassTVS: {Patterns: []string{"^pesd"}}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	SetActiveClassVocab(cv)
	if got := Classify(part, &ir.PartType{}); got != ClassTVS {
		t.Errorf("with tvs:[^pesd], the PESD part should classify TVS, got %s", got)
	}
	if got := Classify(&ir.Component{RefDes: "R1"}, &ir.PartType{}); got != ClassResistor {
		t.Errorf("built-in classification kept alongside the override, got %s", got)
	}
}

// TestBuildClassVocabReplaceAndErrors: Replace drops the built-ins for that class; a bad regex is a
// returned error.
func TestBuildClassVocabReplaceAndErrors(t *testing.T) {
	repl, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassTVS: {Patterns: []string{"^myTvs$"}, Replace: true}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if repl.HintsFor([]string{"tvs"})[ClassTVS] {
		t.Error("replace should drop the built-in tvs token")
	}
	if !repl.HintsFor([]string{"mytvs"})[ClassTVS] {
		t.Error("replace should honor the project pattern")
	}
	if _, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{ClassTVS: {Patterns: []string{"(bad"}}}); err == nil {
		t.Error("a malformed regex must be a returned error")
	}
}

func TestParseComponentClass(t *testing.T) {
	for _, cl := range model.ComponentClasses() {
		if _, ok := ParseComponentClass(string(cl)); !ok {
			t.Errorf("ParseComponentClass(%q) should be known", cl)
		}
	}
	if _, ok := ParseComponentClass("bogus"); ok {
		t.Error("an unknown class name must not parse")
	}
}

// TestBuildClassVocabWithNoOverrideIsTheBuiltIns: a project that declares no class block reads with
// exactly the built-in prefix table, asserted entry by entry rather than assumed (agni 677).
func TestBuildClassVocabWithNoOverrideIsTheBuiltIns(t *testing.T) {
	v, err := BuildClassVocab(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.prefixes) != len(prefixClasses) {
		t.Fatalf("%d prefixes with no override, the built-in table has %d", len(v.prefixes), len(prefixClasses))
	}
	for p, cl := range prefixClasses {
		if got := v.ClassForPrefix(p); got != cl {
			t.Errorf("prefix %s: %s, built-in %s", p, got, cl)
		}
	}
	if got := v.ClassForPrefix("TH"); got != ClassUnknown {
		t.Errorf("TH is no built-in prefix, got %s", got)
	}
}

// TestClassPrefixOverrides covers what a declared prefix does to classification: it applies through a
// part's declared designator prefix as well as its ref-des, it re-points a built-in prefix, replace
// leaves prefixes alone, and a declared vocabulary never leaks into the defaults.
func TestClassPrefixOverrides(t *testing.T) {
	v, err := BuildClassVocab(map[ComponentClass]*configpb.ClassVocab{
		ClassThermistor: {Prefixes: []string{"TH"}, Replace: true},
		ClassFerrite:    {Prefixes: []string{"F"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	lex := &Lexicon{Class: v}
	cases := []struct {
		c    *ir.Component
		pt   *ir.PartType
		want ComponentClass
	}{
		{&ir.Component{RefDes: "TH1"}, &ir.PartType{}, ClassThermistor},
		{&ir.Component{RefDes: "1"}, &ir.PartType{DesignatorPrefix: "TH?"}, ClassThermistor},
		{&ir.Component{RefDes: "RT2"}, &ir.PartType{}, ClassThermistor}, // replace dropped patterns, not prefixes
		{&ir.Component{RefDes: "F4"}, &ir.PartType{}, ClassFerrite},     // a house re-pointing F from fuse
		{&ir.Component{RefDes: "FU4"}, &ir.PartType{}, ClassFuse},
	}
	for _, tc := range cases {
		if got := lex.Classify(tc.c, tc.pt); got != tc.want {
			t.Errorf("%s (prefix %q): %s, want %s", tc.c.GetRefDes(), tc.pt.GetDesignatorPrefix(), got, tc.want)
		}
	}
	if got := DefaultLexicon().Classify(&ir.Component{RefDes: "F4"}, &ir.PartType{}); got != ClassFuse {
		t.Errorf("the defaults must not carry a project's prefixes: F4 read %s", got)
	}
}

// TestClassPrefixLoadErrors: a prefix that can never match, and one claimed by two classes, fail at
// load and name what is wrong, rather than reading as a convention that matched nothing.
func TestClassPrefixLoadErrors(t *testing.T) {
	cases := map[string]struct {
		o    map[ComponentClass]*configpb.ClassVocab
		want []string
	}{
		"digit": {map[ComponentClass]*configpb.ClassVocab{ClassThermistor: {Prefixes: []string{"T1"}}}, []string{`"T1"`, "never match"}},
		"empty": {map[ComponentClass]*configpb.ClassVocab{ClassThermistor: {Prefixes: []string{""}}}, []string{`prefix ""`}},
		"two classes": {map[ComponentClass]*configpb.ClassVocab{
			ClassThermistor: {Prefixes: []string{"TH"}},
			ClassZener:      {Prefixes: []string{"th"}},
		}, []string{`"th"`, `"thermistor"`, `"zener"`}},
	}
	for name, tc := range cases {
		_, err := BuildClassVocab(tc.o)
		if err == nil {
			t.Errorf("%s: loaded without error", name)
			continue
		}
		for _, w := range tc.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: error %q does not name %s", name, err, w)
			}
		}
	}
}
