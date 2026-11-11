package review

import (
	"encoding/json"
	"os"
	"testing"
)

// tallyTwin mirrors testdata/tally_twin.json. The JSON keys are lowerCamel because the web tier reads
// the same file, and its own tally is spelled that way.
type tallyTwin struct {
	Cases []struct {
		Name     string    `json:"name"`
		Outcomes []Outcome `json:"outcomes"`
		Want     struct {
			Pass              int `json:"pass"`
			Fail              int `json:"fail"`
			NotApplicable     int `json:"notApplicable"`
			NotAutomated      int `json:"notAutomated"`
			Provisional       int `json:"provisional"`
			NeedsDesignIntent int `json:"needsDesignIntent"`
			ComputedNA        int `json:"computedNA"`
			NeedsData         int `json:"needsData"`
			Inconclusive      int `json:"inconclusive"`
			CoveredExternally int `json:"coveredExternally"`
			Total             int `json:"total"`
			Covered           int `json:"covered"`
		} `json:"want"`
	} `json:"cases"`
}

// TestTallyTwinFixture checks Report.Tally() against a fixture, and the web tier's review.test.ts
// reads the same fixture (WS9-052). Both surfaces DERIVE the tally rather than send it on the wire.
// That keeps a redundant field off the contract, but the two implementations can then disagree.
//
// A disagreement would not be cosmetic. `covered` is what a team reads as "how much of our checklist
// is actually mechanised", and not-automated is the one outcome it excludes. A client that counted
// not-automated as covered would report a checklist as answered when nobody had answered it. The
// outcome vocabulary exists to prevent that failure.
//
// Checking both sides against ONE file makes that structural rather than a promise. The
// numbers here were authored once, so neither implementation can be "corrected" to match itself.
func TestTallyTwinFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/tally_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin tallyTwin
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Cases) == 0 {
		t.Fatal("fixture has no cases")
	}
	for _, tc := range twin.Cases {
		rep := Report{Areas: []AreaResult{{}}}
		for _, o := range tc.Outcomes {
			rep.Areas[0].Items = append(rep.Areas[0].Items, ItemResult{Outcome: o})
		}
		got := rep.Tally()
		w := tc.Want
		for _, chk := range []struct {
			field     string
			got, want int
		}{
			{"pass", got.Pass, w.Pass},
			{"fail", got.Fail, w.Fail},
			{"notApplicable", got.NotApplicable, w.NotApplicable},
			{"notAutomated", got.NotAutomated, w.NotAutomated},
			{"provisional", got.Provisional, w.Provisional},
			{"needsDesignIntent", got.NeedsDesignIntent, w.NeedsDesignIntent},
			{"computedNA", got.ComputedNA, w.ComputedNA},
			{"needsData", got.NeedsData, w.NeedsData},
			{"inconclusive", got.Inconclusive, w.Inconclusive},
			{"coveredExternally", got.CoveredExternally, w.CoveredExternally},
			{"total", got.Total, w.Total},
			{"covered", got.Covered(), w.Covered},
		} {
			if chk.got != chk.want {
				t.Errorf("%s: %s = %d, want %d", tc.Name, chk.field, chk.got, chk.want)
			}
		}
	}
}

// TestTallyTwinFixtureCoversEveryOutcome fails when the vocabulary grows without the fixture growing
// with it. Without this, adding an outcome would leave the twin silently checking a stale vocabulary.
// Both sides would keep agreeing about the outcomes they already knew, and the new one could be
// bucketed differently on each surface with nothing to notice.
func TestTallyTwinFixtureCoversEveryOutcome(t *testing.T) {
	b, err := os.ReadFile("testdata/tally_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin tallyTwin
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	seen := map[Outcome]bool{}
	for _, tc := range twin.Cases {
		for _, o := range tc.Outcomes {
			seen[o] = true
		}
	}
	for _, o := range []Outcome{
		Pass, Fail, NotApplicable, NotAutomated,
		Provisional, NeedsDesignIntent, ComputedNA, NeedsData, Inconclusive, CoveredExternally,
	} {
		if !seen[o] {
			t.Errorf("outcome %q appears in no twin case; add it to testdata/tally_twin.json so both surfaces are checked on it", o)
		}
	}
}
