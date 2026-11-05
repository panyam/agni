package main

import (
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

// TestTheSampleBoardReadsAndChecks runs the walkthrough's steps on the BeagleBone AI. The board comes
// from the samples corpus, so its absence fails the test rather than skipping it (CONSTRAINTS C10).
func TestTheSampleBoardReadsAndChecks(t *testing.T) {
	if _, err := os.Stat(sampleBoard); err != nil {
		t.Fatalf("no sample board at %s; run `make samples-oracle` from the repo root", sampleBoard)
	}
	d, err := common.ReadDesign(sampleBoard)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 716 || len(d.GetNets()) != 595 {
		t.Errorf("read %d components and %d nets, want the BeagleBone AI's 716 and 595", len(d.GetComponents()), len(d.GetNets()))
	}
	g, err := common.ReadBoard(sampleBoard)
	if err != nil {
		t.Fatal(err)
	}
	lines := boardLines(g)
	for _, want := range []string{"F.Cu In1.Cu", "In10.Cu B.Cu", "263 on top, 453 on the bottom", "1789 vias"} {
		if !strings.Contains(lines, want) {
			t.Errorf("board summary lacks %q:\n%s", want, lines)
		}
	}
	tally := ruleTally(check.RunBackground(check.NewModel(d, check.WithBoard(g)), check.BuiltinRules()))
	if !strings.Contains(tally, "track-width") {
		t.Errorf("the checks never reached the copper, so track-width did not run:\n%s", tally)
	}
}
