package main

import (
	"os"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/examples/common"
)

// TestTheSampleBoardReadsAndChecks runs the walkthrough's steps on the foxBMS master board. The board
// comes from the samples corpus, so its absence fails the test rather than skipping it
// (CONSTRAINTS C10).
func TestTheSampleBoardReadsAndChecks(t *testing.T) {
	if _, err := os.Stat(sampleBoard); err != nil {
		t.Fatalf("no sample board at %s; run `make samples-oracle` from the repo root", sampleBoard)
	}
	d, err := common.ReadDesign(sampleBoard)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.GetComponents()) != 318 || len(d.GetNets()) != 331 {
		t.Errorf("read %d components and %d nets, want the master board's 318 and 331", len(d.GetComponents()), len(d.GetNets()))
	}
	g, err := common.ReadBoard(sampleBoard)
	if err != nil {
		t.Fatal(err)
	}
	lines := common.BoardLines(g)
	if !strings.Contains(lines, "copper layers: F.Cu In1.Cu In2.Cu B.Cu") {
		t.Errorf("board summary lacks the four copper layers:\n%s", lines)
	}
	tally := common.RuleTally(check.RunBackground(check.NewModel(d, check.WithBoard(g)), check.BuiltinRules()))
	if !strings.Contains(tally, "esd-protection") {
		t.Errorf("the checks never judged the USB connector's nets:\n%s", tally)
	}
}
