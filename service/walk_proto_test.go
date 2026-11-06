package service

import (
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
)

func fullWalk() check.Walk {
	return check.Walk{
		From:   check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U2", Pin: "4"}, PinName: "SDA", Net: "SDA"},
		Kind:   "small-parts",
		Radius: 3,
		Reason: "a reason",
		Nets: []check.WalkNet{{
			Hop:         2,
			Net:         check.TraceNet{Name: "SDA_TP", Stubs: []check.TraceStub{{RefDes: "TP1", Pin: "1", Class: "test_point"}}, StubsElided: 1, BusLike: true},
			Entered:     check.TraceCross{RefDes: "R2", Class: "resistor", EnterPin: "1", ExitPin: "2", FromNet: "SDA", ToNet: "SDA_TP"},
			AlsoEntered: []check.TraceCross{{RefDes: "C2", Class: "capacitor", EnterPin: "1", ExitPin: "2", FromNet: "X", ToNet: "SDA_TP", Paired: true}},
			Ended:       "rail",
		}},
		Boundary: []check.WalkBoundary{{RefDes: "U1", Class: "ic", Hop: 1, Net: "SDA", Reason: "5 connected pins"}},
		Elided:   []check.WalkElided{{Hop: 2, Nets: 5}},
		Goal: &check.WalkGoalAnswer{
			Kind: "pin", At: "SDA", To: check.TraceEnd{Endpoint: check.Endpoint{RefDes: "J1", Pin: "2"}, PinName: "SDA", Net: "SDA_J"},
			Outcome: check.TraceRouted, Reason: "a reason", Radius: 6,
			Routes: []check.WalkRoute{{
				Crossings: []check.TraceCross{{RefDes: "R7", Class: "resistor", EnterPin: "1", ExitPin: "2", FromNet: "SDA", ToNet: "SDA_J", Paired: true}},
				Nets:      []check.TraceNet{{Name: "SDA", Stubs: []check.TraceStub{{RefDes: "U2", Pin: "4", Class: "ic"}}, StubsElided: 2, BusLike: true}, {Name: "SDA_J"}},
				EndsAt:    "SDA_J",
				Note:      "a note",
			}},
		},
	}
}

func TestWalkProtoRoundTrip(t *testing.T) {
	want := fullWalk()
	if got := WalkFromProto(WalkProto(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field\n got: %+v\nwant: %+v", got, want)
	}
}

// A new field on a walk type fails here until someone places it on the wire (C26).
func TestWalkFieldCensus(t *testing.T) {
	census := func(v any, want ...string) {
		t.Helper()
		var got []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			got = append(got, rt.Field(i).Name)
		}
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s has fields %v; WalkProto knows %v. Place the new one on the wire, then list it here", rt.Name(), got, want)
		}
	}
	census(check.Walk{}, "From", "Kind", "Radius", "Reason", "Nets", "Boundary", "Elided", "Goal")
	census(check.WalkGoalAnswer{}, "Kind", "At", "To", "Outcome", "Reason", "Radius", "Routes")
	census(check.WalkRoute{}, "Crossings", "Nets", "EndsAt", "Note")
	census(check.WalkNet{}, "Hop", "Net", "Entered", "AlsoEntered", "Ended")
	census(check.WalkBoundary{}, "RefDes", "Class", "Hop", "Net", "Reason")
	census(check.WalkElided{}, "Hop", "Nets")
}

// The walk_goal table writes one row per net of each route, with the crossing into each net after
// the first, and one row stating the outcome when there is no route (agni issue 1340).
func TestWalkGoalTable(t *testing.T) {
	goalTable := func(w check.Walk) [][]string {
		t.Helper()
		var rows [][]string
		for _, tb := range WalkTables(WalkProto(w)) {
			if tb.GetName() != "walk_goal" {
				continue
			}
			for _, r := range tb.GetRows() {
				rows = append(rows, r.GetCells())
			}
			return rows
		}
		t.Fatal("a walk has no walk_goal table")
		return nil
	}
	w := fullWalk()
	got := goalTable(w)
	want := [][]string{
		{"routed", "a reason", "1", "0", "SDA", "", "", "", "U2.4", "a note"},
		{"routed", "a reason", "1", "1", "SDA_J", "R7", "1", "2", "", "a note"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("routed rows\n got: %q\nwant: %q", got, want)
	}
	w.Goal = &check.WalkGoalAnswer{Kind: "rail", At: "SCL", Outcome: check.TraceNoRoute, Reason: "no rail is reachable from SCL"}
	if got := goalTable(w); len(got) != 1 || got[0][0] != "no-route" || got[0][1] != "no rail is reachable from SCL" {
		t.Errorf("no-route rows %q, want one row stating the outcome", got)
	}
	w.Goal = nil
	if got := goalTable(w); len(got) != 0 {
		t.Errorf("a walk with no goal wrote %q", got)
	}
}
