package main

import (
	"os"
	"testing"

	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/examples/common"
)

// TestMain clears the two path variables for the same reason examples/common does: they replace the
// bundled defaults these tests assert against, so a developer who exports either to drive the walk
// over their own board could not run the gate, and the failure would print their path into the log.
func TestMain(m *testing.M) {
	os.Unsetenv(common.DesignPathEnv)
	os.Unsetenv(common.ReviewPathEnv)
	os.Exit(m.Run())
}

// The walkthrough's whole argument is that a checklist reports what it could NOT answer, and the
// bundled checklist is sized so each of those states occurs. Prose cannot be checked by building, so
// this holds the claim: change the fixture or the checklist and a narration that no longer matches
// fails here rather than teaching the wrong lesson quietly.
func TestBundledChecklistShowsEveryOutcomeItNarrates(t *testing.T) {
	rep, err := run(
		common.AskPath("design", "../common/designs/i2c-sensor/i2c-sensor.edn"),
		common.AskReviewPath("checklist", "checklist.yaml"),
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	seen := map[review.Outcome]int{}
	for _, it := range items(rep) {
		seen[it.Outcome]++
	}
	for _, want := range []review.Outcome{
		review.Pass,
		review.Fail,
		review.NotApplicable,
		review.NeedsDesignIntent,
		review.NotAutomated,
	} {
		if seen[want] == 0 {
			t.Errorf("no item resolved %q, so the walkthrough narrates a state it does not show", want)
		}
	}
}

// The unbound item is the one the last step is about, and it is the one most likely to be "tidied
// up" by someone raising the coverage number. Removing it would leave every other assertion here
// passing, so it gets its own.
func TestOneItemIsDeliberatelyUnbound(t *testing.T) {
	rep, err := run(
		common.AskPath("design", "../common/designs/i2c-sensor/i2c-sensor.edn"),
		common.AskReviewPath("checklist", "checklist.yaml"),
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, it := range items(rep) {
		if it.Outcome != review.NotAutomated {
			continue
		}
		if it.Item.Rule != "" || it.Item.Query != nil || it.Item.Present != nil {
			t.Errorf("item %q reads not-automated but carries a binding", it.Item.ID)
		}
		return
	}
	t.Fatal("no unbound item: the checklist no longer demonstrates a question nothing answers")
}
