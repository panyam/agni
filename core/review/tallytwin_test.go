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
			Total             int `json:"total"`
			Covered           int `json:"covered"`
		} `json:"want"`
	} `json:"cases"`
}

// TestTallyTwinFixture checks Report.Tally() against the fixture the web tier's review.test.ts reads
// too (WS9-052). The tally is DERIVED on both surfaces rather than sent on the wire, which keeps a
// redundant field off the contract but means the two implementations can disagree.
//
// A disagreement would not be cosmetic. `covered` is what a team reads as "how much of our checklist
// is actually mechanised", and not-automated is the one outcome it excludes. A client that counted
// not-automated as covered would report a checklist as answered when nobody had answered it, which is
// the exact failure the outcome vocabulary exists to prevent.
//
// Checking both sides against ONE file is what makes that structural rather than a promise: the
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
// with it. Without this, adding an outcome would leave the twin silently checking a stale vocabulary:
// both sides would keep agreeing about the outcomes they already knew, and the new one could be
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
		Provisional, NeedsDesignIntent, ComputedNA, NeedsData, Inconclusive,
	} {
		if !seen[o] {
			t.Errorf("outcome %q appears in no twin case; add it to testdata/tally_twin.json so both surfaces are checked on it", o)
		}
	}
}
