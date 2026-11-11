package review

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

const drc = "kicad-cli pcb drc"

func externalItem(id, rule string) Item {
	return Item{ID: id, Title: id, Binding: Binding{External: &ExternalBinding{Producer: drc, Rule: rule}}}
}

func runExternal(items []Item, reports []ExternalReport) []ItemResult {
	man := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: items}}}
	return runBackground(RunParams{Model: check.NewModel(oneDesign()), Catalog: check.DefaultCatalog(), Manifest: man, Design: "d", External: reports}).Areas[0].Items
}

// An item handed to another tool reads from that tool's report (agni issue 152). A matching
// violation fails it. No matching violation reads covered-externally and never pass. No report from
// that tool reads needs-data.
func TestExternalBindingOutcomes(t *testing.T) {
	report := ExternalReport{Producer: drc, Version: "10.0.6", Findings: []check.Finding{
		{Rule: "kicad-drc/clearance", Severity: "error", Message: "clearance violation"},
		{Rule: "kicad-drc/clearance", Severity: "error", Message: "another"},
		{Rule: "kicad-drc/silk_overlap", Severity: "warning", Message: "silk"},
	}}
	other := ExternalReport{Producer: "kicad-cli sch erc", Findings: []check.Finding{{Rule: "kicad-drc/hole_clearance"}}}
	items := []Item{
		externalItem("clearance", "kicad-drc/clearance"),
		externalItem("holes", "kicad-drc/hole_clearance"),
		externalItem("whole-report", ""),
	}

	got := runExternal(items, []ExternalReport{report, other})
	if len(got) != 3 {
		t.Fatalf("want 3 item results, got %d", len(got))
	}
	if got[0].Outcome != Fail || len(got[0].Findings) != 2 {
		t.Errorf("a matching violation: want fail with 2 findings, got %s with %d", got[0].Outcome, len(got[0].Findings))
	}
	// The ERC report holds a hole_clearance finding, and the item names the DRC, so it must not count.
	if got[1].Outcome != CoveredExternally {
		t.Errorf("no matching violation: want covered-externally, got %s", got[1].Outcome)
	}
	if want := "no kicad-drc/hole_clearance violation in the report from kicad-cli pcb drc 10.0.6"; got[1].Note != want {
		t.Errorf("covered-externally note:\n got %q\nwant %q", got[1].Note, want)
	}
	if got[2].Outcome != Fail || len(got[2].Findings) != 3 {
		t.Errorf("an empty rule takes every violation in the report: want fail with 3, got %s with %d", got[2].Outcome, len(got[2].Findings))
	}

	none := runExternal(items[:1], nil)[0]
	if none.Outcome != NeedsData || !strings.Contains(none.Note, "no report from kicad-cli pcb drc") {
		t.Errorf("no report from the producer: want needs-data naming it, got %s %q", none.Outcome, none.Note)
	}

	clean := runExternal(items[2:], []ExternalReport{{Producer: drc}})[0]
	if clean.Outcome != CoveredExternally || clean.Note != "no violation in the report from kicad-cli pcb drc" {
		t.Errorf("a clean report: want covered-externally, got %s %q", clean.Outcome, clean.Note)
	}
}

// A clean vendor report is a weaker claim than a native pass, so the tally must never count it as
// one. It is covered, since a mechanism stands behind it, and it is not answered, since agni checked
// nothing.
func TestCoveredExternallyIsNeverAPass(t *testing.T) {
	got := runExternal([]Item{externalItem("clearance", "kicad-drc/clearance")}, []ExternalReport{{Producer: drc}})
	rep := Report{Areas: []AreaResult{{Items: got}}}
	tl := rep.Tally()
	if tl.Pass != 0 || tl.Answered() != 0 || tl.Covered() != 1 || tl.CoveredExternally != 1 {
		t.Errorf("want pass 0, answered 0, covered 1, covered-externally 1; got %+v answered=%d covered=%d", tl, tl.Answered(), tl.Covered())
	}
	md := RenderCoverageMarkdown(rep)
	if !strings.Contains(md, "1 covered-externally") || !strings.Contains(md, "Covered-externally |") {
		t.Errorf("coverage rollup does not show the covered-externally count:\n%s", md)
	}
}

// The coverage rollup adds its covered-externally column only to a run that has one, so every
// review without an imported report renders as it did before.
func TestCoverageRollupOmitsExternalColumnWithoutOne(t *testing.T) {
	rep := Report{Areas: []AreaResult{{Area: Area{Name: "A"}, Items: []ItemResult{{Outcome: Pass}}}}}
	if md := RenderCoverageMarkdown(rep); strings.Contains(md, "xternally") {
		t.Errorf("a run with no external item mentions it:\n%s", md)
	}
}

func TestExternalBindingValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    Binding
		want string
	}{
		{"no producer", Binding{External: &ExternalBinding{Rule: "kicad-drc/clearance"}}, `needs "producer"`},
		{"with a rule", Binding{Rule: "x", External: &ExternalBinding{Producer: drc}}, "more than one binding"},
	} {
		m := Manifest{Name: "t", Areas: []Area{{Name: "A", Items: []Item{{ID: "i", Binding: tc.b}}}}}
		err := ValidateStructure(m)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want an error containing %q, got %v", tc.name, tc.want, err)
		}
	}
	m, err := Load(strings.NewReader("name: t\nareas:\n- name: A\n  items:\n  - id: c\n    external:\n      producer: kicad-cli pcb drc\n      rule: kicad-drc/clearance\n"))
	if err != nil {
		t.Fatal(err)
	}
	if e := m.Areas[0].Items[0].External; e == nil || e.Producer != drc || e.Rule != "kicad-drc/clearance" {
		t.Errorf("YAML external binding read as %+v", e)
	}
}

// A rule the tool's own settings turned off shows no violation because it never ran. The item reads
// not-automated and names the setting, and a real violation still fails it.
func TestExternalSkippedRuleIsNotCovered(t *testing.T) {
	skipping := ExternalReport{Producer: drc, Version: "10.0.6",
		Skipped: map[string]string{"kicad-drc/lib_footprint_mismatch": "ignored by the design's rule settings (Footprint doesn't match copy in library)"}}
	got := runExternal([]Item{externalItem("lib", "kicad-drc/lib_footprint_mismatch"), externalItem("all", "")}, []ExternalReport{skipping})
	if got[0].Outcome != NotAutomated {
		t.Errorf("a skipped rule: want not-automated, got %s", got[0].Outcome)
	}
	if want := "kicad-cli pcb drc 10.0.6 did not run kicad-drc/lib_footprint_mismatch: ignored by the design's rule settings (Footprint doesn't match copy in library)"; got[0].Note != want {
		t.Errorf("skipped note:\n got %q\nwant %q", got[0].Note, want)
	}
	if got[1].Outcome != CoveredExternally {
		t.Errorf("a whole-report item over a report with one skipped rule: want covered-externally, got %s", got[1].Outcome)
	}
	skipping.Findings = []check.Finding{{Rule: "kicad-drc/lib_footprint_mismatch"}}
	if got := runExternal([]Item{externalItem("lib", "kicad-drc/lib_footprint_mismatch")}, []ExternalReport{skipping})[0]; got.Outcome != Fail {
		t.Errorf("a violation of a rule also listed as skipped: want fail, got %s", got.Outcome)
	}
}
