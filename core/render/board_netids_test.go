package render

import (
	"slices"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// sigID answers "sig-1" for the fixture's SIG net and nothing for any other, recording what each
// call saw.
func sigID(seen map[string][][2]string) NetIDFunc {
	return func(n BoardNet) string {
		if seen != nil {
			seen[n.Name] = n.Pins
		}
		if n.Name == "SIG" {
			return "sig-1"
		}
		return ""
	}
}

// TestBoardHighlightJoinsANetByItsID holds a board to a finding's highlight, which names its net
// by the schematic id alone (agni issue 1440). The board names its nets, so the id joins through
// the render's NetIDFunc, and without one the same spec lights nothing.
func TestBoardHighlightJoinsANetByItsID(t *testing.T) {
	b := boardFixtureGeometry(t)
	spec := []*geom.HighlightSpec{{NetIds: []string{"sig-1"}}}
	byName := HighlightBoardSVG(b, []*geom.HighlightSpec{{Nets: []string{"SIG"}}})
	byID := HighlightBoardSVG(b, spec, WithNetIDs(sigID(nil)))
	if strings.Count(byID, "<line") != 2 || byID != byName {
		t.Errorf("an id spec drew %d lines, want SIG's 2, the same overlay as naming SIG", strings.Count(byID, "<line"))
	}
	if n := strings.Count(HighlightBoardSVG(b, spec), "<line"); n != 0 {
		t.Errorf("with no NetIDFunc an id spec drew %d lines, want 0", n)
	}
	if n := strings.Count(HighlightBoardSVG(b, []*geom.HighlightSpec{{NetIds: []string{"other"}}}, WithNetIDs(sigID(nil))), "<line"); n != 0 {
		t.Errorf("an id the board's nets don't answer drew %d lines, want 0", n)
	}
}

// TestPackedBoardKeysCarryNetIDs checks that WebGL's resolveHighlights, which joins a key by its
// net_id, gets the id on every key of the net and on no other net's keys.
func TestPackedBoardKeysCarryNetIDs(t *testing.T) {
	b := boardFixtureGeometry(t)
	sig, other := 0, 0
	for _, k := range PackBoard(b, WithNetIDs(sigID(nil))).GetKeys() {
		switch {
		case k.GetNet() == "SIG" && k.GetNetId() == "sig-1":
			sig++
		case k.GetNetId() != "":
			t.Errorf("key on net %q carries id %q", k.GetNet(), k.GetNetId())
		case k.GetNet() == "SIG":
			t.Errorf("a SIG key (primitive %d) has no id", k.GetPrimitive())
		case k.GetNet() != "":
			other++
		}
	}
	if sig == 0 || other == 0 {
		t.Fatalf("SIG keys with id = %d, other net keys = %d, want both above 0", sig, other)
	}
	for _, k := range PackBoard(b).GetKeys() {
		if k.GetNetId() != "" {
			t.Fatalf("with no NetIDFunc key %d carries id %q", k.GetPrimitive(), k.GetNetId())
		}
	}
}

// TestBoardPickKeysCarryNetIDs checks that a click on SIG's copper names its id, as a click on a
// schematic wire does, so the two picks are one Selection.
func TestBoardPickKeysCarryNetIDs(t *testing.T) {
	b := boardFixtureGeometry(t)
	doc := BoardSVG(b, WithPickTargets(), WithNetIDs(sigID(nil)))
	if !strings.Contains(doc, `data-net="SIG" data-net-id="sig-1"`) {
		t.Error("SIG's copper keys no data-net-id")
	}
	if n := strings.Count(doc, `data-net-id=`); n != strings.Count(doc, `data-net-id="sig-1"`) {
		t.Errorf("%d elements key an id, and only SIG's should", n)
	}
	if strings.Contains(BoardSVG(b, WithPickTargets()), "data-net-id") {
		t.Error("with no NetIDFunc the board keys an id")
	}
}

// TestNetIDFuncSeesEachNetsPads checks that a resolver gets every board net once, with its pads
// sorted, so a resolver joining by connections can rely on the order.
func TestNetIDFuncSeesEachNetsPads(t *testing.T) {
	b := boardFixtureGeometry(t)
	seen := map[string][][2]string{}
	PackBoard(b, WithNetIDs(sigID(seen)))
	pads := seen["SIG"]
	if len(pads) < 2 {
		t.Fatalf("SIG's pads = %v, want at least 2", pads)
	}
	if !slices.IsSortedFunc(pads, func(a, b [2]string) int { return strings.Compare(a[0]+"\x00"+a[1], b[0]+"\x00"+b[1]) }) {
		t.Errorf("SIG's pads arrive unsorted: %v", pads)
	}
}
