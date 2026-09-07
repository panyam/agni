package agni_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The packages an embedder is documented to import. A path under internal/ cannot be named from
// another module, so this list is the embedding surface and its location IS the contract.
var embeddingSurface = []string{
	"github.com/panyam/agni",
	"github.com/panyam/agni/service",
	"github.com/panyam/agni/artifact",
}

// TestEmbeddingSurfaceIsImportable is C13's "importable" clause as a test rather than a command in a
// document.
//
// PR #543 moved the service tier out of internal/ and left the check behind as a `go list` line in
// CONSTRAINTS.md. C29's own Verify was written that way and went stale within three PRs, because
// nothing ran it, which is what #542 fixed by turning it into core/facts/deps_test.go. Repeating the
// mistake in the PR that cites it would be a poor showing.
//
// The failure this catches is quiet in a specific way: moving one of these back under internal/
// breaks no build in this repo, since every package here is allowed to import it, and every example
// module keeps compiling too because their module paths are nested under github.com/panyam/agni/.
// Only a real third-party consumer would notice, and there is none in CI to notice for us.
func TestEmbeddingSurfaceIsImportable(t *testing.T) {
	for _, pkg := range embeddingSurface {
		if strings.Contains(pkg, "/internal/") || strings.HasSuffix(pkg, "/internal") {
			t.Errorf("%s is under internal/, so no embedder can import it (C13)", pkg)
		}
		if !packageExists(t, pkg) {
			t.Errorf("%s does not resolve; the embedding surface names a package that is not there", pkg)
		}
	}
}

const (
	rulePrimitive = "github.com/panyam/agni/core/check"
	queryEngine   = "github.com/panyam/agni/core/query"
)

// authoringShapes are the ways a rule can be written. Each compiles to []*check.Rule and reaches a
// catalog as a check.RuleSource; none of them is the primitive.
var authoringShapes = []string{
	"github.com/panyam/agni/stdlib/profiles",     // an interface profile
	"github.com/panyam/agni/stdlib/rules/intent", // a design-intent declaration
	"github.com/panyam/agni/stdlib/rules/datalog",
	queryEngine,
}

// TestRulePrimitiveNamesNoAuthoringShape is the rule-side twin of C29 (C30).
//
// C29 says the fact layer is the primitive and no query engine owns it. The same holds one layer
// over: check.Rule is the rule primitive, and Go, datalog, an interface profile and an intent
// declaration are four ways of AUTHORING one. They meet the catalog at check.RuleSource, which is
// why adding an interface is a data value rather than new code.
//
// The direction is what matters, and only one way round is checkable. An authoring shape depending
// on core/check is correct and every one of them does. core/check depending on an authoring shape is
// the violation, and it would be a quiet one: it compiles, it passes, and it makes that shape's
// limits everyone's limits. Datalog cannot express a path question at all (#374, #518), so a catalog
// that assumed datalog would foreclose the rules that need one.
func TestRulePrimitiveNamesNoAuthoringShape(t *testing.T) {
	for _, dep := range deps(t, rulePrimitive) {
		for _, shape := range authoringShapes {
			if dep == shape {
				t.Errorf("%s depends on %s; the rule primitive must not name an authoring shape (C30)",
					rulePrimitive, shape)
			}
		}
	}
}

// TestAuthoringShapesReachTheCatalogAsRuleSources is the positive half. A shape that stopped
// compiling to rules would not fail the test above, since that one only watches the arrow's
// direction, and an authoring shape that reached the catalog some other way is how the seam erodes.
func TestAuthoringShapesReachTheCatalogAsRuleSources(t *testing.T) {
	for _, shape := range authoringShapes {
		if shape == queryEngine {
			continue // the engine compiles queries; it does not itself author rules
		}
		var found bool
		for _, dep := range deps(t, shape) {
			if dep == rulePrimitive {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not depend on %s, so it no longer compiles to rules", shape, rulePrimitive)
		}
	}
}

func deps(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("go list -deps %s: %v", pkg, err)
	}
	return strings.Fields(string(out))
}

func packageExists(t *testing.T, pkg string) bool {
	t.Helper()
	return exec.Command("go", "list", pkg).Run() == nil
}
