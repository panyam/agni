package main

import (
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

// TestTheSampleDesignReadsWholeAndChecks runs the walkthrough's steps on Clay R6. The design comes
// from the samples corpus, so its absence fails the test rather than skipping it (CONSTRAINTS C10).
func TestTheSampleDesignReadsWholeAndChecks(t *testing.T) {
	if _, err := os.Stat(sampleDesign); err != nil {
		t.Fatalf("no sample design at %s; run `make samples-oracle` from the repo root", sampleDesign)
	}
	d, err := common.ReadDesign(sampleDesign)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 149 || len(d.GetNets()) != 167 {
		t.Errorf("read %d components and %d nets, want Clay R6's 149 and 167", len(d.GetComponents()), len(d.GetNets()))
	}
	if u := d.GetInputDiagnostics().GetUnexpandedHierarchy(); len(u) != 0 {
		t.Errorf("%d blocks were left out: %v", len(u), u)
	}
	tally := common.RuleTally(check.RunBackground(check.NewModel(d), check.BuiltinRules()))
	if !strings.Contains(tally, "single-pin-net") {
		t.Errorf("the checks did not reach the nets:\n%s", tally)
	}
}
