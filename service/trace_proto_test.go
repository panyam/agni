package service

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/render"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	webapi "github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// fullTrace sets EVERY field to a distinguishable non-zero value, which is the load-bearing half of
// a round-trip guard: a field left at its zero value round-trips cleanly through a conversion that
// drops it, so a sparse fixture reports success while covering nothing.
//
// Every repeated field carries TWO elements for the same reason one level in. A converter that drops
// everything past the first element round-trips a one-element slice perfectly, over exactly the bug
// the guard exists to catch.
func fullTrace() check.Trace {
	return check.Trace{
		From:    check.TraceEnd{Endpoint: check.Endpoint{RefDes: "U1", Pin: "3"}, PinName: "SDA", Net: "I2C_SDA"},
		To:      check.TraceEnd{Endpoint: check.Endpoint{RefDes: "J1", Pin: "1"}, PinName: "VBUS", Net: "VCC"},
		Outcome: check.TraceRouted,
		Reason:  "a reason that should survive",
		Radius:  7,
		Crossings: []check.TraceCross{
			{RefDes: "R1", Class: "resistor", EnterPin: "2", ExitPin: "1", FromNet: "I2C_SDA", ToNet: "MID"},
			{RefDes: "FB2", Class: "ferrite", EnterPin: "1", ExitPin: "2", FromNet: "MID", ToNet: "VCC"},
		},
		Nets: []check.TraceNet{
			{
				Name:        "I2C_SDA",
				Stubs:       []check.TraceStub{{RefDes: "TP1", Pin: "1", Class: "test_point"}, {RefDes: "C7", Pin: "1", Class: "capacitor"}},
				StubsElided: 3,
				BusLike:     false,
			},
			{
				Name:        "VCC",
				Stubs:       []check.TraceStub{{RefDes: "C1", Pin: "1", Class: "capacitor"}, {RefDes: "U9", Pin: "4", Class: "ic"}},
				StubsElided: 11,
				BusLike:     true,
			},
		},
	}
}

func TestTraceProtoRoundTrip(t *testing.T) {
	want := fullTrace()
	if got := TraceFromProto(TraceProto(want)); !reflect.DeepEqual(got, want) {
		t.Errorf("round trip lost or changed a field\n got: %+v\nwant: %+v", got, want)
	}
}

// Every outcome must survive the enum crossing, and an unknown one must land on UNSPECIFIED rather
// than on ROUTED. A mapping bug then reads as "we cannot say" instead of as a connection the engine
// never found, which is the direction that cannot mislead.
func TestEveryTraceOutcomeRoundTrips(t *testing.T) {
	for _, o := range []check.TraceOutcome{check.TraceRouted, check.TraceNoRoute, check.TraceUnresolved} {
		tr := fullTrace()
		tr.Outcome = o
		if got := TraceFromProto(TraceProto(tr)).Outcome; got != o {
			t.Errorf("outcome %q came back as %q", o, got)
		}
	}
	tr := fullTrace()
	tr.Outcome = check.TraceOutcome("something nobody defined")
	if got := TraceProto(tr).GetOutcome(); got != webapi.TraceOutcome_TRACE_OUTCOME_UNSPECIFIED {
		t.Errorf("an unknown outcome mapped to %v, want UNSPECIFIED", got)
	}
}

// THE GUARD C26 ACTUALLY ASKS FOR. The round trip above can only test fields it knows about, so a
// new field on check.Trace that the converter never learned is absent from both sides of every
// assertion made on the proto. This fails when the struct grows, so the next person has to SAY
// whether the field belongs on the wire.
func TestTraceFieldCensus(t *testing.T) {
	census := func(v any, wire, goOnly []string, what string) {
		t.Helper()
		var got []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			got = append(got, rt.Field(i).Name)
		}
		sort.Strings(got)
		want := append(append([]string{}, wire...), goOnly...)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s's fields changed.\n got: %v\nwant: %v\n\n"+
				"A new field is not covered by TestTraceProtoRoundTrip until it is in fullTrace(). "+
				"Decide whether it belongs on the wire: if it does, add it to the proto, both "+
				"converters and the fixture, then list it in `wire` here. If it does not, list it in "+
				"`goOnly` and say why.", what, got, want)
		}
	}
	census(check.Trace{}, []string{"Crossings", "From", "Nets", "Outcome", "Radius", "Reason", "To"}, nil, "check.Trace")
	census(check.TraceEnd{}, []string{"Endpoint", "Net", "PinName"}, nil, "check.TraceEnd")
	census(check.TraceCross{}, []string{"Class", "EnterPin", "ExitPin", "FromNet", "RefDes", "ToNet"}, nil, "check.TraceCross")
	census(check.TraceNet{}, []string{"BusLike", "Name", "Stubs", "StubsElided"}, nil, "check.TraceNet")
	census(check.TraceStub{}, []string{"Class", "Pin", "RefDes"}, nil, "check.TraceStub")
	census(check.Endpoint{}, []string{"Pin", "RefDes"}, nil, "check.Endpoint")
}

// The service answers the same question the CLI does, over the same walk, so the two cannot drift.
// It also has to keep the three outcomes apart on the wire, since a client that rendered an
// unresolved endpoint as "not connected" sends its reader to the board instead of to what they typed.
func TestTraceDesignServesTheSameAnswerAsTheWalk(t *testing.T) {
	d := traceServiceFixture()
	svc := NewDesignService(fakeLoader{design: d}, noNative{}, render.Style{}, nil)
	ctx := context.Background()

	routed, err := svc.TraceDesign(ctx, &webapi.TraceDesignRequest{
		Uri:  "mount://m/x.edn",
		From: &webapi.TraceEndpoint{RefDes: "U1", Pin: "1"},
		To:   &webapi.TraceEndpoint{RefDes: "U2", Pin: "1"},
	})
	if err != nil {
		t.Fatalf("TraceDesign: %v", err)
	}
	got := TraceFromProto(routed.GetTrace())
	want := check.TracePins(check.NewModel(d), check.Endpoint{RefDes: "U1", Pin: "1"},
		check.Endpoint{RefDes: "U2", Pin: "1"}, 0)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the served answer differs from the walk's\n got: %+v\nwant: %+v", got, want)
	}
	if got.Outcome != check.TraceRouted {
		t.Fatalf("outcome = %q, want routed", got.Outcome)
	}

	unresolved, err := svc.TraceDesign(ctx, &webapi.TraceDesignRequest{
		Uri:  "mount://m/x.edn",
		From: &webapi.TraceEndpoint{RefDes: "U99", Pin: "1"},
		To:   &webapi.TraceEndpoint{RefDes: "U2", Pin: "1"},
	})
	if err != nil {
		t.Fatalf("TraceDesign on a bad endpoint: %v", err)
	}
	if o := unresolved.GetTrace().GetOutcome(); o != webapi.TraceOutcome_TRACE_OUTCOME_UNRESOLVED {
		t.Errorf("outcome = %v, want UNRESOLVED; a client must not read this as a disconnection", o)
	}
	if unresolved.GetTrace().GetReason() == "" {
		t.Error("an unresolved answer carries no reason, so a client cannot say which name failed")
	}
}

// traceServiceFixture: U1.1 -> R1 -> U2.1, one series crossing, which is enough for the served
// answer and the walk's own answer to be compared field by field.
func traceServiceFixture() *ir.Design {
	net := func(name string, conns ...[2]string) *ir.Net {
		n := &ir.Net{Name: name, Prov: &ir.Provenance{SourceFile: "t"}}
		for _, c := range conns {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: c[0], PinRef: c[1]})
		}
		return n
	}
	comp := func(ref string) *ir.Component {
		return &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
	}
	return &ir.Design{
		Components: []*ir.Component{comp("U1"), comp("U2"), comp("R1"), comp("TP1")},
		Nets: []*ir.Net{
			net("SIG_A", [2]string{"U1", "1"}, [2]string{"R1", "1"}, [2]string{"TP1", "1"}),
			net("SIG_B", [2]string{"R1", "2"}, [2]string{"U2", "1"}),
		},
	}
}
