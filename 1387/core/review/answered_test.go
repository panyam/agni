package review

import "testing"

// TestTallyAnsweredTiers pins the tier of every outcome in the vocabulary. That assignment is the
// whole content of the Answered axis, and the outcome names do not determine it.
//
// NotApplicable and ComputedNA matter most. Both read as "this item does not apply", and they are
// opposite answers. ComputedNA means the DESIGN determined the item is irrelevant, as when no crystal
// is on the board. That is an answer. NotApplicable means the rule's inputs are absent, as when the
// datasheet corpus moved. That is the question going unasked. Covered() cannot tell them apart, and
// this axis exists to catch the second as a regression.
func TestTallyAnsweredTiers(t *testing.T) {
	for _, tc := range []struct {
		outcome  Outcome
		answered bool
	}{
		{Pass, true},
		{Fail, true},
		{Provisional, true},
		{ComputedNA, true},
		{NotApplicable, false},
		{NotAutomated, false},
		{NeedsData, false},
		{NeedsDesignIntent, false},
		{Inconclusive, false},
	} {
		var got Tally
		got.add(tc.outcome)
		want := 0
		if tc.answered {
			want = 1
		}
		if got.Answered() != want {
			t.Errorf("Answered() for %s = %d, want %d", tc.outcome, got.Answered(), want)
		}
	}
}

// TestAnsweredDivergesFromCovered is the acceptance case for the axis. A checklist's answered count
// drops, and its covered and fail counts do not change.
//
// `check --fail-on` structurally cannot see this shape. Nothing failed, nothing left the catalog, and
// one fewer question got answered.
func TestAnsweredDivergesFromCovered(t *testing.T) {
	report := func(second Outcome) Report {
		return Report{Manifest: "M", Design: "d", Areas: []AreaResult{{
			Area: Area{Name: "A"},
			Items: []ItemResult{
				{Item: Item{ID: "1"}, Outcome: Pass},
				{Item: Item{ID: "2"}, Outcome: second},
				{Item: Item{ID: "3"}, Outcome: NotAutomated},
			},
		}}}
	}
	// Before: the datasheet corpus is seeded, so item 2 evaluates.
	before := report(Pass).Tally()
	// After: the corpus moved. Item 2's rule is still in the catalog and still selected; it simply has
	// no facts to read, so check.Available gates it to not-applicable.
	after := report(NotApplicable).Tally()

	if before.Covered() != after.Covered() {
		t.Fatalf("Covered() moved (%d -> %d); the premise of this test is that it does not",
			before.Covered(), after.Covered())
	}
	if before.Fail != 0 || after.Fail != 0 {
		t.Fatalf("failures must stay at zero, got %d -> %d", before.Fail, after.Fail)
	}
	if after.Answered() >= before.Answered() {
		t.Errorf("Answered() must drop, got %d -> %d", before.Answered(), after.Answered())
	}
}
