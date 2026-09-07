package agni

import (
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// This test binary deliberately imports NEITHER stdlib/relations NOR stdlib/rules/builtin, which is
// what makes the refusals below testable at all: the package under test is in exactly the state an
// embedder's binary is in when they forget a blank import. internal/composetest is the other half,
// composing everything and asserting the green path.

func TestNewRefusesWhenNoRelationsAreInstalled(t *testing.T) {
	_, err := New()
	if !errors.Is(err, MissingRelationsError) {
		t.Fatalf("New with no relation catalog = %v, want MissingRelationsError", err)
	}
	if !strings.Contains(err.Error(), "stdlib/relations") {
		t.Errorf("error does not name the import that fixes it: %v", err)
	}
}

func TestNewRefusesWhenBuiltinRulesAreNotInstalled(t *testing.T) {
	// Get past the relations check, so the builtins check is what we are actually reading. Without
	// this the first refusal masks the second and the test would pass for the wrong reason.
	facts.RegisterRelation("composetest_probe", []facts.Field{facts.FieldSubject}, func(check.Model) []facts.Row { return nil })
	_, err := New()
	if !errors.Is(err, MissingBuiltinsError) {
		t.Fatalf("New with no built-in rules = %v, want MissingBuiltinsError", err)
	}
	if !strings.Contains(err.Error(), "stdlib/rules/builtin") {
		t.Errorf("error does not name the import that fixes it: %v", err)
	}
}

// A size check on the composed catalog would NOT catch a missing built-in source, because
// stdlib/profiles registers its own rules from an init that this package's imports trigger. Pinning
// it keeps anyone from "simplifying" checkSeams into the check that silently passes.
func TestComposedCatalogIsNonEmptyWithoutTheBuiltins(t *testing.T) {
	if len(check.BuiltinRules()) != 0 {
		t.Skip("built-in rules are installed in this binary; the premise does not hold")
	}
	if got := len(check.DefaultCatalog().Rules()); got == 0 {
		t.Fatalf("composed catalog is empty, so a size check would have caught the missing built-ins "+
			"and checkSeams could use one; got %d rules", got)
	}
}
