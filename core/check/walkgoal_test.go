package check

import (
	"errors"
	"strings"
	"testing"
)

// goalOf walks the walk fixture and asks one goal question from it.
func goalOf(t *testing.T, from Endpoint, kind string, hops int, g WalkGoal) *WalkGoalAnswer {
	t.Helper()
	m := NewModel(walkFixture())
	w, err := WalkFrom(m, from, kind, hops, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := WalkGoalFrom(m, w, g)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// routeOf renders one route as its nets joined by the parts crossed, "SDA_TP [R2] SDA [R1] +3V3".
func routeOf(r WalkRoute) string {
	if len(r.Nets) == 0 {
		return ""
	}
	parts := []string{r.Nets[0].Name}
	for i, c := range r.Crossings {
		parts = append(parts, "["+c.RefDes+"]", r.Nets[i+1].Name)
	}
	return strings.Join(parts, " ")
}

func TestPullUpGoalExtendsTheWalkToTheResistorAndItsRail(t *testing.T) {
	a := goalOf(t, Endpoint{RefDes: "TP1"}, "small-parts", 2, WalkGoal{Kind: WalkGoalPullUp, At: "SDA"})
	if a.Outcome != TraceRouted || len(a.Routes) != 1 {
		t.Fatalf("got %s with %d routes (%s), want one route", a.Outcome, len(a.Routes), a.Reason)
	}
	r := a.Routes[0]
	if got, want := routeOf(r), "SDA_TP [R2] SDA [R1] +3V3"; got != want {
		t.Fatalf("route %q, want %q: the walk's own path to SDA, then the pull-up", got, want)
	}
	if r.EndsAt != "+3V3" {
		t.Errorf("ends at %q, want the rail", r.EndsAt)
	}
	if c := r.Crossings[1]; c.EnterPin != "1" || c.ExitPin != "2" || c.FromNet != "SDA" {
		t.Errorf("the pull-up crossing is %+v, want R1 pin 1 --> pin 2 from SDA", c)
	}
}

func TestPullUpGoalWithNoResistorSaysNoRouteWithinItsReach(t *testing.T) {
	a := goalOf(t, Endpoint{RefDes: "U1", Pin: "4"}, "small-parts", 1, WalkGoal{Kind: WalkGoalPullUp})
	if a.At != "SCL" {
		t.Errorf("asked from %q, want the start pin's own net SCL", a.At)
	}
	if a.Outcome != TraceNoRoute || len(a.Routes) != 0 {
		t.Fatalf("got %s with %d routes, want no route", a.Outcome, len(a.Routes))
	}
	if !strings.Contains(a.Reason, "within 3 hops") || a.Radius != PullUpReachHops {
		t.Errorf("reason %q radius %d, want the pull-up reach stated", a.Reason, a.Radius)
	}
}

func TestGoalFromANetTheWalkNeverReachedIsUnresolved(t *testing.T) {
	a := goalOf(t, Endpoint{RefDes: "TP1"}, "small-parts", 1, WalkGoal{Kind: WalkGoalRail, At: "INT"})
	if a.Outcome != TraceUnresolved || !strings.Contains(a.Reason, "did not reach a net INT") {
		t.Fatalf("got %s %q, want unresolved naming INT", a.Outcome, a.Reason)
	}
}

func TestPinGoalFromAPinStartAnswersAsTraceDoes(t *testing.T) {
	from, to := Endpoint{RefDes: "TP1", Pin: "1"}, Endpoint{RefDes: "U2", Pin: "4"}
	a := goalOf(t, from, "small-parts", 1, WalkGoal{Kind: WalkGoalPin, To: to})
	tr := TracePins(NewModel(walkFixture()), from, to, 0)
	if a.Outcome != TraceRouted || tr.Outcome != TraceRouted || len(a.Routes) != 1 {
		t.Fatalf("goal %s, trace %s: want both routed", a.Outcome, tr.Outcome)
	}
	if got, want := routeOf(a.Routes[0]), routeOf(WalkRoute{Crossings: tr.Crossings, Nets: tr.Nets}); got != want {
		t.Fatalf("goal route %q, trace route %q", got, want)
	}
	for i, n := range a.Routes[0].Nets {
		if len(n.Stubs) != len(tr.Nets[i].Stubs) {
			t.Errorf("net %s lists %d parts, trace lists %d", n.Name, len(n.Stubs), len(tr.Nets[i].Stubs))
		}
	}
	if a.To.Net != "SDA" {
		t.Errorf("the pin resolved to net %q, want SDA", a.To.Net)
	}
}

func TestPinGoalKeepsTracesOtherTwoOutcomesApart(t *testing.T) {
	none := goalOf(t, Endpoint{RefDes: "TP1"}, "small-parts", 2, WalkGoal{Kind: WalkGoalPin, At: "SDA", To: Endpoint{RefDes: "U1", Pin: "4"}})
	if none.Outcome != TraceNoRoute || !strings.Contains(none.Reason, "from SDA to SCL within 6 crossings") {
		t.Errorf("SCL: got %s %q, want no route within the radius", none.Outcome, none.Reason)
	}
	typo := goalOf(t, Endpoint{RefDes: "TP1"}, "small-parts", 2, WalkGoal{Kind: WalkGoalPin, To: Endpoint{RefDes: "U9", Pin: "1"}})
	if typo.Outcome != TraceUnresolved || !strings.Contains(typo.Reason, "no component U9") {
		t.Errorf("U9: got %s %q, want unresolved", typo.Outcome, typo.Reason)
	}
}

func TestRailGoalRoutesToTheNearestRail(t *testing.T) {
	a := goalOf(t, Endpoint{RefDes: "TP1"}, "small-parts", 1, WalkGoal{Kind: WalkGoalRail})
	if a.At != "SDA_TP" {
		t.Errorf("asked from %q, want TP1's own net", a.At)
	}
	if a.Outcome != TraceRouted || len(a.Routes) != 1 {
		t.Fatalf("got %s %q, want one route", a.Outcome, a.Reason)
	}
	if got, want := routeOf(a.Routes[0]), "SDA_TP [R2] SDA [R1] +3V3"; got != want {
		t.Errorf("route %q, want %q", got, want)
	}
	none := goalOf(t, Endpoint{RefDes: "U1", Pin: "4"}, "small-parts", 1, WalkGoal{Kind: WalkGoalRail})
	if none.Outcome != TraceNoRoute || !strings.Contains(none.Reason, "from SCL") {
		t.Errorf("SCL: got %s %q, want no route", none.Outcome, none.Reason)
	}
}

// Jetson's U80 translator has only do-not-populate pull-ups on one side. The goal reports that side
// as having no pull-up, and the fitted side's pull-up as a route through the translator's pin pair.
func TestPullUpGoalAcrossATranslatorSkipsAnUnfittedPullUp(t *testing.T) {
	ask := func(dnp bool) *WalkGoalAnswer {
		d := translatorFixture()
		if dnp {
			for _, c := range d.Components {
				if c.RefDes == "R5" {
					c.Attributes = map[string]string{"dnp": "yes"}
				}
			}
		}
		m := NewModel(d)
		w, err := WalkFrom(m, Endpoint{RefDes: "R1"}, "small-parts", 2, nil)
		if err != nil {
			t.Fatal(err)
		}
		a, err := WalkGoalFrom(m, w, WalkGoal{Kind: WalkGoalPullUp, At: "SDA_3V3"})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	fitted := ask(false)
	if fitted.Outcome != TraceRouted || len(fitted.Routes) != 1 {
		t.Fatalf("fitted: got %s %q", fitted.Outcome, fitted.Reason)
	}
	if got, want := routeOf(fitted.Routes[0]), "SDA_1V8 [U5] SDA_3V3 [R5] +3V3"; got != want {
		t.Errorf("fitted: route %q, want %q", got, want)
	}
	if !fitted.Routes[0].Crossings[0].Paired {
		t.Errorf("the translator crossing lost its pin pair: %+v", fitted.Routes[0].Crossings[0])
	}
	if unfitted := ask(true); unfitted.Outcome != TraceNoRoute {
		t.Errorf("unfitted: got %s, want no route", unfitted.Outcome)
	}
}

func TestUnknownWalkGoalIsAnError(t *testing.T) {
	m := NewModel(walkFixture())
	w, _ := WalkFrom(m, Endpoint{RefDes: "TP1"}, "", 1, nil)
	if _, err := WalkGoalFrom(m, w, WalkGoal{Kind: "ground"}); !errors.Is(err, ErrUnknownWalkGoal) {
		t.Fatalf("got %v, want ErrUnknownWalkGoal", err)
	}
}

// A part start asks from its first own net that is not a rail. Here the pull-up's rail comes first in
// the netlist, and the goal still asks from the signal.
func TestGoalFromAPartStartSkipsItsRail(t *testing.T) {
	d := walkFixture()
	d.Nets[0], d.Nets[2] = d.Nets[2], d.Nets[0] // +3V3 before SDA
	m := NewModel(d)
	w, err := WalkFrom(m, Endpoint{RefDes: "R1"}, "small-parts", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.Nets[0].Net.Name != "+3V3" {
		t.Fatalf("the fixture walks %s first, want +3V3 so the test means something", w.Nets[0].Net.Name)
	}
	a, err := WalkGoalFrom(m, w, WalkGoal{Kind: WalkGoalPullUp})
	if err != nil {
		t.Fatal(err)
	}
	if a.At != "SDA" || a.Outcome != TraceRouted {
		t.Errorf("asked from %q with %s, want SDA routed", a.At, a.Outcome)
	}
}
