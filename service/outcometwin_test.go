package service

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/panyam/agni/core/check"
	checkspb "github.com/panyam/agni/gen/go/agni/v1/checks"
)

// TestOutcomeTwin holds OutcomeWord, and the check.Outcome values it names, to
// testdata/outcome_twin.json, which the viewer's outcomeWord is tested against too (agni issue 1045).
func TestOutcomeTwin(t *testing.T) {
	b, err := os.ReadFile("testdata/outcome_twin.json")
	if err != nil {
		t.Fatal(err)
	}
	var twin struct {
		Outcomes []struct {
			Wire int32  `json:"wire"`
			Word string `json:"word"`
		} `json:"outcomes"`
	}
	if err := json.Unmarshal(b, &twin); err != nil {
		t.Fatal(err)
	}
	if len(twin.Outcomes) < 6 {
		t.Fatalf("fixture has %d outcomes", len(twin.Outcomes))
	}
	words := map[string]bool{}
	for _, o := range []check.Outcome{check.Pass, check.Fail, check.NoLimit, check.NotConsidered, check.Inconclusive} {
		words[string(o)] = true
	}
	for _, o := range twin.Outcomes {
		if got := OutcomeWord(checkspb.Outcome(o.Wire)); got != o.Word {
			t.Errorf("OutcomeWord(%d) = %q, want %q", o.Wire, got, o.Word)
		}
		if o.Word != "unspecified" && !words[o.Word] {
			t.Errorf("%q is no check.Outcome value", o.Word)
		}
	}
}
