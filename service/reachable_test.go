package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// fullReachable sets every check.Reachable field, with more than one pin and rail, so a field the
// converters drop fails the round trip (C26).
func fullReachable() check.Reachable {
	end := func(ref, pin, name, net string) check.TraceEnd {
		return check.TraceEnd{Endpoint: check.Endpoint{RefDes: ref, Pin: pin}, PinName: name, Net: net}
	}
	return check.Reachable{
		From:   end("U7", "3", "SDA", "SDA_RAW"),
		Reason: "kept for the round trip",
		Radius: 4,
		Pins: []check.ReachablePin{
			{TraceEnd: end("TP1", "1", "", "SDA_RAW"), Depth: 0},
			{TraceEnd: end("U12", "4", "SDIO", "SDA_FILT"), Depth: 1},
		},
		Rails: []check.ReachedRail{{Net: "VCC", Depth: 2, Pins: 40}, {Net: "GND", Depth: 3, Pins: 412}},
	}
}

func TestReachableProtoRoundTrip(t *testing.T) {
	want := fullReachable()
	if got := ReachableFromProto(ReachableProto(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field\n got: %+v\nwant: %+v", got, want)
	}
}

// TestReachableFieldCensus fails when check.Reachable or its parts grow a field, until the field is
// on the wire and in fullReachable, or listed here as Go-only with a reason.
func TestReachableFieldCensus(t *testing.T) {
	fields := func(v any) []string {
		rt := reflect.TypeOf(v)
		var out []string
		for i := 0; i < rt.NumField(); i++ {
			out = append(out, rt.Field(i).Name)
		}
		return out
	}
	for _, c := range []struct {
		v    any
		want []string
	}{
		{check.Reachable{}, []string{"From", "Reason", "Radius", "Pins", "Rails"}},
		{check.ReachablePin{}, []string{"TraceEnd", "Depth"}},
		{check.ReachedRail{}, []string{"Net", "Depth", "Pins"}},
	} {
		if got := fields(c.v); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%T's fields are %v, want %v. Put a new field on the wire, in both converters and in fullReachable, then list it here.", c.v, got, c.want)
		}
	}
}

// The served answer is the walk's answer, so the CLI and the viewer cannot drift (agni issue 1541).
func TestReachablePinsServesTheWalksAnswer(t *testing.T) {
	d := traceServiceFixture()
	svc := NewDesignService(fakeLoader{design: d}, noNative{}, render.Style{}, nil)
	snap := snapOf(t, svc.Snapshots(), "mount://m/x.edn")
	resp, err := svc.ReachablePins(context.Background(), &webapi.ReachablePinsRequest{
		Snapshot: snap,
		From:     &webapi.TraceEndpoint{RefDes: "U1", Pin: "1"},
	})
	if err != nil {
		t.Fatalf("ReachablePins: %v", err)
	}
	got := ReachableFromProto(resp)
	want := check.ReachablePins(check.NewModel(d), check.Endpoint{RefDes: "U1", Pin: "1"}, 0)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the served answer differs from the walk's\n got: %+v\nwant: %+v", got, want)
	}
	if len(got.Pins) != 4 {
		t.Errorf("%d pins, want R1.1 and TP1.1 at 0 and R1.2 and U2.1 at 1", len(got.Pins))
	}
	bad, err := svc.ReachablePins(context.Background(), &webapi.ReachablePinsRequest{Snapshot: snap, From: &webapi.TraceEndpoint{RefDes: "U99", Pin: "1"}})
	if err != nil || bad.GetReason() == "" || len(bad.GetPins()) != 0 {
		t.Errorf("an unknown start: err %v, reason %q, %d pins; want a reason and no pins", err, bad.GetReason(), len(bad.GetPins()))
	}
}
