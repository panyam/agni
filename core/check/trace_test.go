package check

import (
	"fmt"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// traceFixture holds five topologies, each answering one question a trace has to get right:
//
//	SDA:   U7.3 -> R5 -> U12.4, with a test point on one net and a cap on the other (the route)
//	VCC:   SDA_FILT -> R6 -> VCC, where U10.1 sits on the rail (a bus-like DESTINATION)
//	BLOCK: U8.1 -> C9 -> U9.1, a capacitor between the two (no route; a cap is not a pass element)
//	CHAIN: U20.1 -> R21 -> R22 -> R23 -> U21.1 (three crossings, for the radius)
//	WIDE:  a net carrying fourteen stubs besides the route (the stub cap)
func traceFixture() *ir.Design {
	lib := &ir.PartLibrary{Name: "lib", Parts: []*ir.PartType{
		{Name: "MCU", Pins: []*ir.Pin{
			{Designator: "3", Name: "SDA"},
			{Designator: "9", Name: "NC_SPARE"}, // declared, on no net
		}},
		{Name: "EEP", Pins: []*ir.Pin{{Designator: "4", Name: "SDIO"}}},
	}}
	comp := func(ref, part string) *ir.Component {
		c := &ir.Component{RefDes: ref, Prov: &ir.Provenance{SourceFile: "t"}}
		if part != "" {
			c.Sections = []*ir.ComponentSection{{PartRef: part, LibraryRef: "lib"}}
		}
		return c
	}
	comps := []*ir.Component{
		comp("U7", "MCU"), comp("U12", "EEP"), comp("R5", ""), comp("R6", ""),
		comp("TP1", ""), comp("C7", ""), comp("U10", ""),
		comp("U8", ""), comp("U9", ""), comp("C9", ""),
		comp("U20", ""), comp("U21", ""), comp("R21", ""), comp("R22", ""), comp("R23", ""),
		comp("U30", ""), comp("R30", ""),
	}
	vcc := tnet("VCC", "R6.2", "U10.1")
	vcc.Attributes = map[string]string{"global": "true"}

	wide := []string{"U30.1", "R30.1"}
	for i := 1; i <= 14; i++ {
		ref := fmt.Sprintf("CW%02d", i)
		comps = append(comps, comp(ref, ""))
		wide = append(wide, ref+".1")
	}

	return &ir.Design{
		Libraries:  []*ir.PartLibrary{lib},
		Components: comps,
		Nets: []*ir.Net{
			tnet("SDA_RAW", "U7.3", "R5.1", "TP1.1"),
			tnet("SDA_FILT", "R5.2", "U12.4", "C7.1", "R6.1"),
			vcc,
			tnet("BLOCK_A", "U8.1", "C9.1"),
			tnet("BLOCK_B", "C9.2", "U9.1"),
			tnet("CH0", "U20.1", "R21.1"),
			tnet("CH1", "R21.2", "R22.1"),
			tnet("CH2", "R22.2", "R23.1"),
			tnet("CH3", "R23.2", "U21.1"),
			tnet(strings.ToUpper("wide"), wide...),
			tnet("WIDE_FAR", "R30.2"),
		},
	}
}

func traceModel() Model { return NewModel(traceFixture()) }

func TestTraceRoutesThroughASeriesElement(t *testing.T) {
	tr := TracePins(traceModel(), Endpoint{"U7", "3"}, Endpoint{"U12", "4"}, DefaultTraceHops)
	if tr.Outcome != TraceRouted {
		t.Fatalf("outcome = %q (%s), want routed", tr.Outcome, tr.Reason)
	}
	if tr.From.PinName != "SDA" || tr.To.PinName != "SDIO" {
		t.Errorf("endpoint pin names = %q/%q, want SDA/SDIO", tr.From.PinName, tr.To.PinName)
	}
	if len(tr.Crossings) != 1 {
		t.Fatalf("crossings = %d, want 1: %+v", len(tr.Crossings), tr.Crossings)
	}
	c := tr.Crossings[0]
	if c.RefDes != "R5" || c.EnterPin != "1" || c.ExitPin != "2" {
		t.Errorf("crossing = %+v, want R5 entered at pin 1 and left at pin 2", c)
	}
	if c.FromNet != "SDA_RAW" || c.ToNet != "SDA_FILT" {
		t.Errorf("crossing nets = %s -> %s, want SDA_RAW -> SDA_FILT", c.FromNet, c.ToNet)
	}
	if len(tr.Nets) != 2 || tr.Nets[0].Name != "SDA_RAW" || tr.Nets[1].Name != "SDA_FILT" {
		t.Fatalf("route nets = %+v, want SDA_RAW then SDA_FILT", tr.Nets)
	}
}

// A test point on a route net is the most useful thing on the line for a reviewer, so it is reported
// and it is reported FIRST. The series element the route passes through is not a stub.
func TestTraceReportsStubsWithTestPointsFirst(t *testing.T) {
	tr := TracePins(traceModel(), Endpoint{"U7", "3"}, Endpoint{"U12", "4"}, DefaultTraceHops)
	if tr.Outcome != TraceRouted {
		t.Fatalf("outcome = %q (%s)", tr.Outcome, tr.Reason)
	}
	first := tr.Nets[0]
	if len(first.Stubs) != 1 || first.Stubs[0].RefDes != "TP1" {
		t.Errorf("SDA_RAW stubs = %+v, want just TP1", first.Stubs)
	}
	if first.Stubs[0].Class != string(ClassTestPoint) {
		t.Errorf("TP1 class = %q, want %q", first.Stubs[0].Class, ClassTestPoint)
	}
	var refs []string
	for _, s := range tr.Nets[1].Stubs {
		refs = append(refs, s.RefDes)
	}
	if strings.Join(refs, ",") != "C7,R6" {
		t.Errorf("SDA_FILT stubs = %v, want C7 and R6 (R5 is on the route, not a stub)", refs)
	}
}

// The terminus rule, with its own positive control. A route may END on a rail, and the walk that
// refuses a rail outright must still refuse it, or this test would pass for the wrong reason.
func TestTraceEndsOnARailThatReachRefuses(t *testing.T) {
	m := traceModel()
	tr := TracePins(m, Endpoint{"U7", "3"}, Endpoint{"U10", "1"}, DefaultTraceHops)
	if tr.Outcome != TraceRouted {
		t.Fatalf("outcome = %q (%s), want a route onto the VCC rail", tr.Outcome, tr.Reason)
	}
	last := tr.Nets[len(tr.Nets)-1]
	if last.Name != "VCC" || !last.BusLike {
		t.Errorf("last net = %+v, want VCC marked bus-like", last)
	}
	// Positive control: the plain walk must not reach VCC at all, so the terminus admission is
	// what produced the route above rather than the rail having stopped being bus-like.
	var start *ir.Net
	for _, n := range m.Nets() {
		if n.Name == "SDA_RAW" {
			start = n
		}
	}
	for _, n := range m.Reach(start, DefaultTraceHops).Nets {
		if n.Name == "VCC" {
			t.Fatal("Reach reached VCC; the rail exclusion it shares with the protection rules is gone")
		}
	}
}

// A bus-like net is a destination and never a transit node, so a route may not continue out of one.
func TestTraceWillNotTransitARail(t *testing.T) {
	d := traceFixture()
	// R7 hangs a further net off the VCC rail. Reaching U40.1 would mean crossing THROUGH VCC.
	d.Components = append(d.Components,
		&ir.Component{RefDes: "R7", Prov: &ir.Provenance{SourceFile: "t"}},
		&ir.Component{RefDes: "U40", Prov: &ir.Provenance{SourceFile: "t"}})
	for _, n := range d.Nets {
		if n.Name == "VCC" {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: "R7", PinRef: "1"})
		}
	}
	d.Nets = append(d.Nets, tnet("BEYOND", "R7.2", "U40.1"))

	tr := TracePins(NewModel(d), Endpoint{"U7", "3"}, Endpoint{"U40", "1"}, DefaultTraceHops)
	if tr.Outcome != TraceNoRoute {
		t.Fatalf("outcome = %q, want no-route: a rail is a destination, not a doorway", tr.Outcome)
	}
}

// The positive control for the whole command: a topology that must NOT route. A capacitor is a DC
// block, so two pins either side of one are not connected however short the path looks.
func TestTraceDoesNotCrossACapacitor(t *testing.T) {
	tr := TracePins(traceModel(), Endpoint{"U8", "1"}, Endpoint{"U9", "1"}, DefaultTraceHops)
	if tr.Outcome != TraceNoRoute {
		t.Fatalf("outcome = %q, want no-route across C9", tr.Outcome)
	}
	if !strings.Contains(tr.Reason, "6 crossings") {
		t.Errorf("reason = %q, want the radius the answer rests on", tr.Reason)
	}
	if tr.From.Net != "BLOCK_A" || tr.To.Net != "BLOCK_B" {
		t.Errorf("both endpoints resolved wrongly: %+v %+v", tr.From, tr.To)
	}
}

// The radius is a real bound and the answer states it, so "no route" can be re-asked wider.
func TestTraceRadiusBoundsTheSearch(t *testing.T) {
	m := traceModel()
	if tr := TracePins(m, Endpoint{"U20", "1"}, Endpoint{"U21", "1"}, 2); tr.Outcome != TraceNoRoute {
		t.Errorf("at radius 2 outcome = %q, want no-route across three crossings", tr.Outcome)
	}
	tr := TracePins(m, Endpoint{"U20", "1"}, Endpoint{"U21", "1"}, 3)
	if tr.Outcome != TraceRouted {
		t.Fatalf("at radius 3 outcome = %q (%s), want routed", tr.Outcome, tr.Reason)
	}
	if tr.Radius != 3 {
		t.Errorf("radius = %d, want the 3 the caller asked for", tr.Radius)
	}
	var refs []string
	for _, c := range tr.Crossings {
		refs = append(refs, c.RefDes)
	}
	if strings.Join(refs, ",") != "R21,R22,R23" {
		t.Errorf("crossings = %v, want R21,R22,R23 in order", refs)
	}
}

// Two pins on one net are one electrical node, which is a route with nothing crossed.
func TestTraceSameNetIsRoutedWithNoCrossings(t *testing.T) {
	tr := TracePins(traceModel(), Endpoint{"U7", "3"}, Endpoint{"TP1", "1"}, DefaultTraceHops)
	if tr.Outcome != TraceRouted || len(tr.Crossings) != 0 {
		t.Fatalf("outcome = %q with %d crossings, want routed with none", tr.Outcome, len(tr.Crossings))
	}
	if len(tr.Nets) != 1 || tr.Nets[0].Name != "SDA_RAW" {
		t.Errorf("nets = %+v, want just SDA_RAW", tr.Nets)
	}
}

// The case the ticket is emphatic about: an endpoint that does not resolve must never read as a
// disconnection, and the four ways it can fail send a reader to four different places.
func TestTraceUnresolvedEndpointsAreNotDisconnections(t *testing.T) {
	m := traceModel()
	cases := []struct {
		name string
		from Endpoint
		want string
	}{
		{"unknown ref-des", Endpoint{"U99", "1"}, "no component U99"},
		{"declared pin on no net", Endpoint{"U7", "9"}, "U7.9 is declared and sits on no net"},
		{"pin the part type does not declare", Endpoint{"U7", "77"}, "U7 declares no pin 77"},
		{"part carrying no pin list", Endpoint{"TP1", "77"}, "no pin list for TP1"},
		{"not a pin at all", Endpoint{"U7", ""}, "not a pin"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := TracePins(m, c.from, Endpoint{"U12", "4"}, DefaultTraceHops)
			if tr.Outcome != TraceUnresolved {
				t.Fatalf("outcome = %q, want unresolved", tr.Outcome)
			}
			if !strings.Contains(tr.Reason, c.want) {
				t.Errorf("reason = %q, want it to contain %q", tr.Reason, c.want)
			}
		})
	}
}

// A capped stub list says how many it dropped, so a truncated net does not read as a complete one.
func TestTraceStubCapIsCounted(t *testing.T) {
	tr := TracePins(traceModel(), Endpoint{"U30", "1"}, Endpoint{"R30", "2"}, DefaultTraceHops)
	if tr.Outcome != TraceRouted {
		t.Fatalf("outcome = %q (%s)", tr.Outcome, tr.Reason)
	}
	wide := tr.Nets[0]
	if len(wide.Stubs) != traceStubLimit {
		t.Fatalf("stubs listed = %d, want the cap of %d", len(wide.Stubs), traceStubLimit)
	}
	if wide.StubsElided != 2 {
		t.Errorf("elided = %d, want 2 (14 stubs, cap %d)", wide.StubsElided, traceStubLimit)
	}
}

// The walk records the crossed element's pin on each side, which is what makes a step renderable.
func TestReachStepCarriesPinsOnBothSides(t *testing.T) {
	m := traceModel()
	var start, target *ir.Net
	for _, n := range m.Nets() {
		switch n.Name {
		case "SDA_RAW":
			start = n
		case "SDA_FILT":
			target = n
		}
	}
	steps := m.Reach(start, 2).StepsTo(target)
	if len(steps) != 1 {
		t.Fatalf("steps = %+v, want one", steps)
	}
	if steps[0].FromPin != "1" || steps[0].ToPin != "2" {
		t.Errorf("step pins = %q/%q, want 1/2", steps[0].FromPin, steps[0].ToPin)
	}
}
