package review

import (
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
)

func wlReport(design string, items ...ItemResult) Report {
	return Report{Design: design, Areas: []AreaResult{{Area: Area{Name: "A"}, Items: items}}}
}

func blockedItem(id string, deps ...check.UnmetDependency) ItemResult {
	return ItemResult{Item: Item{ID: id}, Outcome: NeedsData, Unmet: deps}
}

// The prioritisation signal a per-item view structurally cannot show: an item lists the facts IT
// needs, and only a rollup knows which fact the most items are waiting on.
func TestWorkListRanksByHowMuchAFactUnblocks(t *testing.T) {
	r := wlReport("d",
		blockedItem("1", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT"}),
		blockedItem("2", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT"}),
		blockedItem("3", check.UnmetDependency{MPN: "ACME-2", Symbol: "VIN"}),
	)
	got := WorkList(r)
	if len(got) != 2 {
		t.Fatalf("one fact blocking two items is one job: %+v", got)
	}
	if got[0].GetDependency().GetMpn() != "ACME-1" || len(got[0].GetBlocked()) != 2 {
		t.Errorf("want the most-blocking fact first, got %+v", got[0])
	}
	if strings.Join(got[0].GetBlocked(), ",") != "1,2" {
		t.Errorf("want the blocked items named and ordered, got %v", got[0].GetBlocked())
	}
}

// One part seeded once unblocks every design that places it; asking a person to find the same fact
// per design is how a work list gets ignored.
func TestWorkListMergesAcrossDesigns(t *testing.T) {
	a := wlReport("board-a", blockedItem("1", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT"}))
	b := wlReport("board-b", blockedItem("9", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT"}))
	got := WorkListAcross([]Report{a, b})
	if len(got) != 1 {
		t.Fatalf("the same fact across two designs is one job: %+v", got)
	}
	if len(got[0].GetBlocked()) != 2 {
		t.Errorf("want both designs' items credited, got %v", got[0].GetBlocked())
	}
}

// One design seeing a spec does not make another's absence go away, so the list keeps the harder
// state rather than understating the work.
func TestWorkListKeepsTheStrongerClaim(t *testing.T) {
	a := wlReport("a", blockedItem("1", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT", Manufacturer: "MakerCo"}))
	b := wlReport("b", blockedItem("2", check.UnmetDependency{MPN: "ACME-1", Symbol: "IOUT", SpecAbsent: true}))
	got := WorkListAcross([]Report{a, b})
	if len(got) != 1 || !got[0].GetDependency().GetSpecAbsent() {
		t.Fatalf("want one entry marked spec-absent, got %+v", got)
	}
	if got[0].GetDependency().GetManufacturer() != "MakerCo" {
		t.Errorf("a manufacturer learned from either side should survive, got %q", got[0].GetDependency().GetManufacturer())
	}
}

// A fully seeded design needs nothing, and that must read as a result rather than as an error or an
// empty table.
func TestWorkListEmptyIsAResult(t *testing.T) {
	clean := wlReport("d", ItemResult{Item: Item{ID: "1"}, Outcome: Pass})
	if got := WorkList(clean); len(got) != 0 {
		t.Fatalf("nothing blocked, got %+v", got)
	}
	out := RenderWorkListMarkdown(nil)
	if !strings.Contains(out, "needs nothing") && !strings.Contains(out, "No unmet") {
		t.Errorf("the empty case must say so in words, got %q", out)
	}
	if strings.Contains(out, "|") {
		t.Error("an empty table reads as a bug; say it in a sentence")
	}
}

func TestRenderNamesTheNextStep(t *testing.T) {
	out := RenderWorkListMarkdown(WorkList(wlReport("d",
		blockedItem("1", check.UnmetDependency{MPN: "ACME-2", Symbol: "VIN", SpecAbsent: true}))))
	if !strings.Contains(out, "no spec") {
		t.Errorf("a part with no document at all needs saying, got %q", out)
	}
	if !strings.Contains(out, "ACME-2") || !strings.Contains(out, "VIN") {
		t.Errorf("want the part and symbol rendered, got %q", out)
	}
}

// Stability: the same report yields the same list, so a work list can be diffed between runs to see
// what seeding actually cleared.
func TestWorkListIsStable(t *testing.T) {
	r := wlReport("d",
		blockedItem("1", check.UnmetDependency{MPN: "B", Symbol: "X"}),
		blockedItem("2", check.UnmetDependency{MPN: "A", Symbol: "X"}),
	)
	first, second := WorkList(r), WorkList(r)
	for i := range first {
		if first[i].GetDependency().GetMpn() != second[i].GetDependency().GetMpn() {
			t.Fatalf("unstable at %d: %q vs %q", i, first[i].GetDependency().GetMpn(), second[i].GetDependency().GetMpn())
		}
	}
	if first[0].GetDependency().GetMpn() != "A" {
		t.Errorf("equal blockage ties break by part, got %q", first[0].GetDependency().GetMpn())
	}
}
