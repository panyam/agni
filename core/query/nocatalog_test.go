package query

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// TestNoCatalogSaysSo is the red-check for the silent-empty-fact-base trap. A host that omits the
// relation-catalog import still builds and still runs: the fact base is simply empty, so every
// relation is unknown. Before this, the error blamed a typo and offered the nearest catalog name —
// which is empty too, so the reader got a bare "unknown relation" and no way to reach the real
// cause. A datalog-authored rule swallows that error into zero findings, which is a clean pass on a
// design nobody checked, so this message is the one place the omission is visible.
func TestNoCatalogSaysSo(t *testing.T) {
	defer facts.Snapshot()()
	// Strip the catalog the test binary installed (relations_register_test.go), leaving the evaluator
	// with its computed predicates and nothing to look up.
	facts.Reset()

	q := MustParse(`component-on-net(?r,?n) => ?r`)
	_, err := Naive{}.Eval(q, NewBase(check.NewModel(chainDesign())))
	if err == nil {
		t.Fatal("evaluating against an uninstalled fact base returned no error; a rule reads that as a clean pass")
	}
	if !strings.Contains(err.Error(), "no fact relations are installed") {
		t.Errorf("error %q does not name the missing catalog", err)
	}
	if strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error %q blames a typo when the catalog is absent", err)
	}
}

// TestUnknownRelationStillSuggests pins the other side: with a catalog installed, a mistyped name
// still gets its typo hint rather than the missing-catalog message.
func TestUnknownRelationStillSuggests(t *testing.T) {
	q := MustParse(`component_on_net(?r,?n) => ?r`)
	_, err := Naive{}.Eval(q, NewBase(check.NewModel(chainDesign())))
	if err == nil {
		t.Fatal("a mistyped relation must error")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error %q lost the typo hint", err)
	}
}
