package main

import (
	"os"
	"testing"

	"github.com/panyam/agni/core/review"
	"github.com/panyam/agni/examples/common"
)

// TestMain clears the two path variables for the same reason examples/common does. They replace the
// bundled defaults, and these tests assert against those defaults. A developer who exports either
// variable to drive the walk over their own board could not run the gate, and the failure would
// print their path into the log.
func TestMain(m *testing.M) {
	os.Unsetenv(common.DesignPathEnv)
	os.Unsetenv(common.ReviewPathEnv)
	os.Exit(m.Run())
}

// The walkthrough argues that a checklist reports what it could NOT answer. The bundled checklist
// is sized so each of those states occurs. Prose cannot be checked by building, so this test holds
// the claim. Change the fixture or the checklist, and a narration that no longer matches fails here
// rather than teaching the wrong lesson quietly.
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

// The last step is about the unbound item. Someone raising the coverage number is likely to "tidy
// up" that item. Removing it would leave every other assertion here passing, so it gets its own
// test.
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
