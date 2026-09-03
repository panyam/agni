package facts_test

import (
	"os/exec"
	"strings"
	"testing"
)

const queryEngine = "github.com/panyam/agni/core/query"

// TestCoreNamesNoQueryEngine is C29 as a test rather than a command in a document.
//
// The fact layer is the primitive and a query engine depends on it, never the reverse (#536), and no
// core package outside the engine itself names a query syntax (#537). Both are one import away from
// being untrue and neither failure is loud: adding `core/query` to stdlib/relations or core/review
// compiles, passes, and silently re-couples the layer. C29 carried a `go list` command for this, which
// went stale within three PRs of being written because nothing ran it.
//
// The shape is core/model/deps_test.go's, for the same reason: a contract that a consumer must be able
// to depend on without dragging an implementation is only a contract while something checks.
func TestCoreNamesNoQueryEngine(t *testing.T) {
	for _, pkg := range corePackages(t) {
		if pkg == queryEngine {
			continue // the engine is allowed to be itself
		}
		for _, dep := range deps(t, pkg) {
			if dep == queryEngine {
				t.Errorf("%s depends on the query engine; C29 keeps core free of one", pkg)
			}
		}
	}
}

// TestRelationCatalogNamesNoQueryEngine is the other half: the shipped relation catalog is DATA
// derived from a Model, so authoring a relation must not require picking an engine. stdlib/relations
// imported core/query until #536 purely to declare its tuple type.
func TestRelationCatalogNamesNoQueryEngine(t *testing.T) {
	for _, dep := range deps(t, "github.com/panyam/agni/stdlib/relations") {
		if dep == queryEngine {
			t.Errorf("stdlib/relations depends on the query engine; a relation is data, not a query")
		}
	}
}

func corePackages(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "github.com/panyam/agni/core/...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	return strings.Fields(string(out))
}

func deps(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pkg).CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps %s: %v\n%s", pkg, err, out)
	}
	return strings.Fields(string(out))
}
