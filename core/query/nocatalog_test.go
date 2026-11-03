package query

import (
	"context"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// TestNoCatalogSaysSo is the red-check for the silent-empty-fact-base trap. A host that omits the
// relation-catalog import still builds and still runs. Its fact base is simply empty, so every
// relation is unknown. Before this, the error blamed a typo and offered the nearest catalog name.
// That catalog is empty too, so the reader got a bare "unknown relation" and no way to reach the
// real cause. A datalog-authored rule swallows that error into zero findings. Zero findings read as
// a clean pass on a design nobody checked. So this message is the only place the omission shows.
func TestNoCatalogSaysSo(t *testing.T) {
	// A bare vocabulary with no built-in catalog, so every relation is unknown. It is composed
	// rather than stripped from the process default. A host that omits the catalog import is
	// actually in this state.
	bare, err := facts.NewRegistry()
	if err != nil {
		t.Fatalf("compose empty registry: %v", err)
	}

	q := MustParse(`component.net(?r,?n) => ?r`)
	_, err = Naive{}.Eval(context.Background(), q, NewBaseFrom(bare, check.NewModel(chainDesign())))
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

// TestUnknownRelationStillSuggests pins the other side. With a catalog installed, a mistyped name
// still gets its typo hint rather than the missing-catalog message.
func TestUnknownRelationStillSuggests(t *testing.T) {
	q := MustParse(`component_on_net(?r,?n) => ?r`)
	_, err := Naive{}.Eval(context.Background(), q, NewBase(check.NewModel(chainDesign())))
	if err == nil {
		t.Fatal("a mistyped relation must error")
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("error %q lost the typo hint", err)
	}
}
