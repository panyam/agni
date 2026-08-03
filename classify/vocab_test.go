package classify

import (
	"testing"

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
	cv, err := BuildClassVocab(map[ComponentClass]VocabPatterns{ClassTVS: {Patterns: []string{"^pesd"}}})
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
	repl, err := BuildClassVocab(map[ComponentClass]VocabPatterns{ClassTVS: {Patterns: []string{"^myTvs$"}, Replace: true}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if repl.HintsFor([]string{"tvs"})[ClassTVS] {
		t.Error("replace should drop the built-in tvs token")
	}
	if !repl.HintsFor([]string{"mytvs"})[ClassTVS] {
		t.Error("replace should honor the project pattern")
	}
	if _, err := BuildClassVocab(map[ComponentClass]VocabPatterns{ClassTVS: {Patterns: []string{"(bad"}}}); err == nil {
		t.Error("a malformed regex must be a returned error")
	}
}

func TestParseComponentClass(t *testing.T) {
	for _, name := range []string{"tvs", "diode", "test_point", "resistor"} {
		if _, ok := ParseComponentClass(name); !ok {
			t.Errorf("ParseComponentClass(%q) should be known", name)
		}
	}
	if _, ok := ParseComponentClass("bogus"); ok {
		t.Error("an unknown class name must not parse")
	}
}
