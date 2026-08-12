package review

import (
	"encoding/json"
	"strings"
	"testing"
)

// twoDesignAgg builds an Aggregate of the same one-area manifest run on two designs: item "1" is
// automated (pass on d1, fail on d2), item "2" is not-automated (design-independent).
func twoDesignAgg() Aggregate {
	rep := func(design string, o1 Outcome) Report {
		return Report{Manifest: "M", Design: design, Areas: []AreaResult{{
			Area: Area{Name: "Area A"},
			Items: []ItemResult{
				{Item: Item{ID: "1", Title: "first"}, Outcome: o1},
				{Item: Item{ID: "2", Title: "second"}, Outcome: NotAutomated},
			},
		}}}
	}
	return Aggregate{Manifest: "M", Reports: []Report{rep("d1", Pass), rep("d2", Fail)}}
}

// TestAggregateAutomationIsManifestLevel is the load-bearing invariant: automation (automated /
// not-automated) is stated ONCE for the manifest, never summed across designs — else "1 automated"
// would read as "2". Pass/fail is per design.
func TestAggregateAutomationIsManifestLevel(t *testing.T) {
	s := RenderAggregateMarkdown(twoDesignAgg())
	if !strings.Contains(s, "**1 of 2 items covered** (manifest-level), 1 not-automated") {
		t.Errorf("want manifest-level automation (1 of 2), got:\n%s", s)
	}
	if strings.Contains(s, "2 of 4") {
		t.Error("automation must not be summed across designs")
	}
	// per-design rows and the per-item matrix with per-design cells
	for _, want := range []string{
		// Answered is per design and leads the row: d1 answered item 1 with a pass, d2 with a fail, and
		// item 2 is not-automated on both, so each design answers 1 of 2.
		"| `d1` | 1/2 | 1 | 0 | 0 |",
		"| `d2` | 1/2 | 0 | 1 | 0 |",
		"### Area A",
		"| 1 | first | pass | fail |",
		"| 2 | second | not-automated | not-automated |",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, s)
		}
	}
}

// TestAggregateCoverageHasNoMatrix: the coverage renderer is the summary only.
func TestAggregateCoverageHasNoMatrix(t *testing.T) {
	s := RenderAggregateCoverageMarkdown(twoDesignAgg())
	if !strings.Contains(s, "## Per-design outcomes") {
		t.Errorf("coverage missing the per-design summary\n%s", s)
	}
	if strings.Contains(s, "Traceability matrix") {
		t.Error("coverage must not include the per-item matrix")
	}
}

// TestAggregateJSON: the JSON carries manifest-level automation, per-design summaries, and per-item
// outcome-by-design.
func TestAggregateJSON(t *testing.T) {
	s, err := RenderAggregateJSON(twoDesignAgg())
	if err != nil {
		t.Fatal(err)
	}
	var got jsonAggregate
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || got.Automated != 1 || got.NotAutomated != 1 {
		t.Errorf("automation = {total:%d automated:%d not_automated:%d}, want {2 1 1}", got.Total, got.Automated, got.NotAutomated)
	}
	if len(got.PerDesign) != 2 || got.PerDesign[0].Design != "d1" {
		t.Errorf("per_design = %+v, want two designs starting d1", got.PerDesign)
	}
	it := got.Areas[0].Items[0]
	if it.Outcomes["d1"] != "pass" || it.Outcomes["d2"] != "fail" {
		t.Errorf("item 1 outcomes = %v, want d1:pass d2:fail", it.Outcomes)
	}
}
