package model_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The model package is the design read-surface CONTRACT (WS1-043): a consumer must be able to
// depend on it without pulling the check implementation (rules + irModel) or the param logic
// package — that is the whole point of the extraction. If this fails, the interface has grown a
// dependency that re-couples the contract to the implementation.
func TestModelDepsExcludeImplementation(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", "github.com/panyam/agni/model").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.HasPrefix(dep, "github.com/panyam/agni/check") || dep == "github.com/panyam/agni/datasheet/param" {
			t.Errorf("model must depend on the contract only, but pulls the implementation %q", dep)
		}
	}
}
