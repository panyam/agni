package query

import (
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestRouteBindsTheRouteItWalked is the artifact agni issue 518 asks for, over the straight chain
// N0 -R1- N1 -R2- N2 -R3- N3: the answer names the parts it crossed and the nets between them, so a
// reviewer reading a hundred rows can check any one of them without re-asking the question.
//
// The reflexive row is asserted alongside the others rather than skipped. A net reaches itself at
// distance zero, and its route is its own name: the two points are one electrical node, which is the
// strongest form of connected there is and not a degenerate case to filter out.
func TestRouteBindsTheRouteItWalked(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `route("N0", ?n, ?p) => ?n, ?p`)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Bind["n"].S] = r.Bind["p"].S
	}
	want := map[string]string{
		"N0": "N0",
		"N1": "N0 -[R1]- N1",
		"N2": "N0 -[R1]- N1 -[R2]- N2",
		"N3": "N0 -[R1]- N1 -[R2]- N2 -[R3]- N3",
	}
	if len(got) != len(want) {
		t.Errorf("route bound %d destinations, want %d: %v", len(got), len(want), got)
	}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("route(N0, %s, ?p) bound %q, want %q", n, got[n], w)
		}
	}
}

// TestRouteNeverBindsAnEmptyPath is the requirement issue 518 states as the one thing that must not
// happen: a path question that finds nothing must never render as an empty line. In a query the
// honest form of "no route" is NO ROW, so what this guards is the other shape — a row that arrives
// with an empty string in its path column, which a table renders as a blank cell beside two net
// names and a reader takes for a route with nothing on it.
func TestRouteNeverBindsAnEmptyPath(t *testing.T) {
	rows := runQuery(t, check.NewModel(reachChainDesign()), `route(?a, ?b, ?p) => ?a, ?b, ?p`)
	if len(rows) == 0 {
		t.Fatal("no rows: the fixture should route, so this test would pass vacuously")
	}
	for _, r := range rows {
		if r.Bind["p"].S == "" {
			t.Errorf("route(%s, %s) bound an empty path", r.Bind["a"].S, r.Bind["b"].S)
		}
	}
}

// TestRouteAgreesWithReachesOnWhatIsConnected is the sibling guard. route and reaches run the same
// walk, so a pair one of them yields and the other does not is an inconsistency in the engine rather
// than a difference anyone asked for.
//
// The fixture carries a GROUND net for the assertion to have teeth. A bus-like net is excluded from
// the walk outright rather than admitted as a terminus, so a route implementation that reached for
// ReachToTerminus (which `agni trace` does use, correctly, for a pin-to-pin question) would bind
// pairs reaches denies. Without the rail both sides agree whatever the walk does.
func TestRouteAgreesWithReachesOnWhatIsConnected(t *testing.T) {
	m := check.NewModel(railChainDesign())
	pairs := func(text, a, b string) []string {
		var out []string
		for _, r := range runQuery(t, m, text) {
			out = append(out, r.Bind[Var(a)].S+"->"+r.Bind[Var(b)].S)
		}
		sort.Strings(out)
		return out
	}
	reached := pairs(`reaches(?a, ?b) => ?a, ?b`, "a", "b")
	routed := pairs(`route(?a, ?b, ?p) => ?a, ?b`, "a", "b")
	if strings.Join(reached, ",") != strings.Join(routed, ",") {
		t.Errorf("route and reaches disagree about what is connected:\n  reaches: %v\n  route:   %v", reached, routed)
	}
	// Two positive controls, because the assertion above passes on an empty result and on a fixture
	// where nothing is bus-like.
	//
	// The first: the rail must not be a DESTINATION of any other net, which is exactly the pair a
	// terminus-admitting walk would add. Reflexively GND reaches itself, since a walk's own start net
	// is never treated as a stop, so the reflexive pair is not the one to look for.
	for _, p := range routed {
		if a, b, _ := strings.Cut(p, "->"); b == "GND" && a != "GND" {
			t.Errorf("route reached the ground net from %s: a rail is a stop, not a destination", a)
		}
	}
	// The second: the chain has to route at all, so agreement is agreement about something.
	if len(routed) < 4 {
		t.Fatalf("fixture routed %d pairs, too few to prove agreement: %v", len(routed), routed)
	}
}

// railChainDesign is reachChainDesign with a ground net hung off the far end through R4. GND is
// bus-like by name, so the series walk refuses it: it is the net that separates a walk which stops
// at a rail from one that ends on it.
func railChainDesign() *ir.Design {
	d := reachChainDesign()
	d.Components = append(d.Components, &ir.Component{
		RefDes: "R4", Sections: []*ir.ComponentSection{{PartRef: "R"}},
		Prov: &ir.Provenance{SourceFile: "c"},
	})
	for _, n := range d.Nets {
		if n.Name == "N3" {
			n.Connections = append(n.Connections, &ir.Connection{ComponentRef: "R4", PinRef: "1"})
		}
	}
	d.Nets = append(d.Nets, &ir.Net{
		Name: "GND", Connections: []*ir.Connection{{ComponentRef: "R4", PinRef: "2"}},
		Prov: &ir.Provenance{SourceFile: "c"},
	})
	return d
}

// TestRouteIsLintedAsAGenerator wires the new predicate into the guard that already exists for the
// old one. route walks from every net on the board when its first argument is unbound, so a rule
// opening with it is the same non-terminating shape WS3-114 named, and a generator the lint cannot
// see is a generator that ships without the warning.
func TestRouteIsLintedAsAGenerator(t *testing.T) {
	q, err := Parse(`bad(?n) :- route(?a, ?n, ?p); bad(?n) => ?n`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := GeneratorFirstRules(q); len(got) != 1 || got[0] != "bad" {
		t.Errorf("GeneratorFirstRules = %v, want [bad]: route is a generator and the lint must see it", got)
	}
}

// TestRouteArityIsFixed pins that route takes exactly three arguments. reaches next door accepts two
// or three, and the reason route does not is the point of it being a separate predicate: there is no
// spelling where a caller binds a path without asking for one.
func TestRouteArityIsFixed(t *testing.T) {
	for _, text := range []string{`route(?a, ?b) => ?a`, `route(?a, ?b, ?p, ?q) => ?a`} {
		q, err := Parse(text)
		if err != nil {
			continue // a parse error is a fine way to reject it too
		}
		if _, err := (Naive{}).Eval(q, NewBase(check.NewModel(reachChainDesign()))); err == nil {
			t.Errorf("%s: want an error naming the accepted arity", text)
		}
	}
}
