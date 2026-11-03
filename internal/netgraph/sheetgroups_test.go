package netgraph

import (
	"reflect"
	"testing"
)

// strapped is a child sheet's two ports, A and B, each on its own wire. The parent wires them
// together through its two sheet pins, which are anchors carrying the child's names on the
// parent's sheet.
func strapped(childSheet, parentSheet string) ([]Wire, []Anchor, []Pin) {
	p := func(x int64) Point { return Point{X: x} }
	wires := []Wire{
		{A: p(0), B: p(1), Sheet: childSheet},    // port A's wire in the child
		{A: p(10), B: p(11), Sheet: childSheet},  // port B's wire in the child
		{A: p(20), B: p(21), Sheet: parentSheet}, // the parent's strap between its two pins
	}
	anchors := []Anchor{
		{At: p(0), Label: "/c/A", Rank: 3, Sheet: childSheet},
		{At: p(10), Label: "/c/B", Rank: 3, Sheet: childSheet},
		{At: p(20), Label: "/c/A", Rank: 4, Sheet: parentSheet},
		{At: p(21), Label: "/c/B", Rank: 4, Sheet: parentSheet},
	}
	pins := []Pin{{At: p(1), Comp: "R1", Pin: "1"}, {At: p(11), Comp: "R1", Pin: "2"}}
	return wires, anchors, pins
}

// Two ports a parent straps together meet on the parent, not on the child, and the groups say so.
func TestSheetGroupsSayWhereNamesMet(t *testing.T) {
	w, a, p := strapped("/c", "")
	nets, _, _ := Build(w, a, p, nil)
	if len(nets) != 1 {
		t.Fatalf("want one net, got %d: %+v", len(nets), nets)
	}
	want := []SheetGroup{
		{Sheet: "", Names: []string{"/c/A", "/c/B"}},
		{Sheet: "/c", Names: []string{"/c/A"}},
		{Sheet: "/c", Names: []string{"/c/B"}},
	}
	if got := nets[0].SheetGroups; !reflect.DeepEqual(got, want) {
		t.Errorf("SheetGroups\n got %+v\nwant %+v", got, want)
	}
	if got := ParseSheetGroups(IRNets(nets, "t")[0].GetAttributes()[AttrSheetGroups]); !reflect.DeepEqual(got, want) {
		t.Errorf("the IR attribute decodes to %+v, want %+v", got, want)
	}
}

// A caller that names no sheet gets no groups, so its nets read as they did before groups existed.
func TestSheetGroupsNeedASheet(t *testing.T) {
	w, a, p := strapped("", "")
	nets, _, _ := Build(w, a, p, nil)
	if len(nets) != 1 || nets[0].SheetGroups != nil {
		t.Errorf("want one net with no SheetGroups, got %+v", nets)
	}
	if _, ok := IRNets(nets, "t")[0].GetAttributes()[AttrSheetGroups]; ok {
		t.Error("an IR net with no groups carries the attribute")
	}
}
