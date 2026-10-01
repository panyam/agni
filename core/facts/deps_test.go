package facts_test

import (
	"os/exec"
	"strings"
	"testing"
)

// queryEngine is agni's adapter over the Datalog engine, and datalogEngine the engine itself, which
// lives in its own module since agni issue 731. A core package importing either has picked an engine.
const (
	queryEngine   = "github.com/panyam/agni/core/query"
	datalogEngine = "github.com/panyam/jaala/datalog"
)

func isEngine(dep string) bool { return dep == queryEngine || dep == datalogEngine }

// TestCoreNamesNoQueryEngine is C29 as a test rather than a command in a document.
//
// The fact layer is the primitive and a query engine depends on it, never the reverse (#536), and no
// core package outside the engine itself names a query syntax (#537). Both are one import away from
// being untrue and neither failure is loud. Adding `core/query` to stdlib/relations or core/review
// compiles, passes, and silently re-couples the layer. C29 carried a `go list` command for this, which
// went stale within three PRs of being written because nothing ran it.
//
// The shape is core/model/deps_test.go's, for the same reason. A contract that a consumer must be able
// to depend on without dragging an implementation is only a contract while something checks.
func TestCoreNamesNoQueryEngine(t *testing.T) {
	for _, pkg := range corePackages(t) {
		if pkg == queryEngine {
			continue // the engine is allowed to be itself
		}
		for _, dep := range deps(t, pkg) {
			if isEngine(dep) {
				t.Errorf("%s depends on the query engine (%s); C29 keeps core free of one", pkg, dep)
			}
		}
	}
}

// TestQueryEngineReachesTheDatalogEngine is the positive control for the two sweeps here. They pass
// by finding nothing, so a stale engine path would make them pass forever. The adapter does depend
// on the engine, so if this fails the paths above no longer name it.
func TestQueryEngineReachesTheDatalogEngine(t *testing.T) {
	for _, dep := range deps(t, queryEngine) {
		if dep == datalogEngine {
			return
		}
	}
	t.Errorf("%s does not depend on %s; the engine paths these sweeps look for are stale", queryEngine, datalogEngine)
}

// TestRelationCatalogNamesNoQueryEngine is the other half. The shipped relation catalog is DATA
// derived from a Model, so authoring a relation must not require picking an engine. stdlib/relations
// imported core/query until #536 purely to declare its tuple type.
func TestRelationCatalogNamesNoQueryEngine(t *testing.T) {
	for _, dep := range deps(t, "github.com/panyam/agni/stdlib/relations") {
		if isEngine(dep) {
			t.Errorf("stdlib/relations depends on the query engine (%s); a relation is data, not a query", dep)
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
