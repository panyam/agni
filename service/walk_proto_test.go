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
	census(check.Walk{}, "From", "Kind", "Radius", "Reason", "Nets", "Boundary", "Elided")
	census(check.WalkNet{}, "Hop", "Net", "Entered", "AlsoEntered", "Ended")
	census(check.WalkBoundary{}, "RefDes", "Class", "Hop", "Net", "Reason")
	census(check.WalkElided{}, "Hop", "Nets")
}
