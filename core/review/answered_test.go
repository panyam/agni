package review

import "testing"

// TestTallyAnsweredTiers pins which tier every outcome in the vocabulary lands in, because that
// assignment is the whole content of the Answered axis and it is not derivable from the outcome names.
//
// The pair that matters most is NotApplicable and ComputedNA. Both read as "this item does not apply",
// and they are opposite answers: ComputedNA is the DESIGN determining the item is irrelevant (no
// crystal on the board), which is an answer; NotApplicable is the rule's inputs being absent (the
// datasheet corpus moved), which is the question going unasked. Covered() cannot tell them apart, and
// the second is the regression this axis exists to catch.
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

// TestAnsweredDivergesFromCovered is the acceptance case for the axis: a checklist whose answered count
// drops while covered and fail both stay put.
//
// This is the shape `check --fail-on` structurally cannot see. Nothing failed, nothing left the
// catalog, and one fewer question got answered.
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
