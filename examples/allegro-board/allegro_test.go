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
	lines := common.BoardLines(g)
	for _, want := range []string{"F.Cu In1.Cu", "In10.Cu B.Cu", "263 on top, 453 on the bottom", "1789 vias"} {
		if !strings.Contains(lines, want) {
			t.Errorf("board summary lacks %q:\n%s", want, lines)
		}
	}
	m := check.NewModel(d, check.WithBoard(g))
	tally := common.RuleTally(check.RunBackground(m, check.BuiltinRules()))
	if !strings.Contains(tally, "copper-clearance") {
		t.Errorf("the checks never reached the copper, so copper-clearance did not run:\n%s", tally)
	}
	// The board's narrowest constraint set is its track floor (agni issue 1432), so track-width judges
	// every routed net and fails none of them.
	judged, failed := 0, 0
	for _, v := range check.RunVerdictsBackground(m, check.BuiltinRules()) {
		if v.Rule != "track-width" {
			continue
		}
		switch v.Outcome {
		case check.Pass:
			judged++
		case check.Fail:
			judged++
			failed++
		}
	}
	if judged < 500 || failed != 0 {
		t.Errorf("track-width judged %d nets and failed %d, want the board's routed nets judged and none failed", judged, failed)
	}
}
