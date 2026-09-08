package model

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// chainReach hand-builds the walk result for N0 -R1- N1 -R2- N2, which is what the BFS records:
// every reached net has a Depth, and every net past the start has the step that entered it.
func chainReach() (Reach, map[string]*ir.Net) {
	nets := map[string]*ir.Net{}
	for _, n := range []string{"N0", "N1", "N2", "OFF"} {
		nets[n] = &ir.Net{Name: n}
	}
	r := Reach{
		Nets:    []*ir.Net{nets["N0"], nets["N1"], nets["N2"]},
		Crossed: map[string]bool{"R1": true, "R2": true},
		Parent: map[string]ReachStep{
			"N1": {From: "N0", Through: "R1", FromPin: "1", ToPin: "2"},
			"N2": {From: "N1", Through: "R2", FromPin: "1", ToPin: "2"},
		},
		Depth: map[string]int{"N0": 0, "N1": 1, "N2": 2},
	}
	return r, nets
}

// TestRouteLineRendersTheCrossings covers the three returns separately, because two of them are
// answers and one is not, and collapsing them is what lets a caller print an empty line and call it
// a route (agni issue 518).
func TestRouteLineRendersTheCrossings(t *testing.T) {
	r, nets := chainReach()
	for _, c := range []struct {
		name, target, want string
	}{
		{"a route names the parts it crossed", "N2", "N0 -[R1]- N1 -[R2]- N2"},
		{"one crossing", "N1", "N0 -[R1]- N1"},
		{"the start is its own route", "N0", "N0"},
		{"an unreached net has no route", "OFF", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := r.RouteLine(nets[c.target]); got != c.want {
				t.Errorf("RouteLine(%s) = %q, want %q", c.target, got, c.want)
			}
		})
	}
	if got := r.RouteLine(nil); got != "" {
		t.Errorf("RouteLine(nil) = %q, want empty", got)
	}
}

// TestRouteLineReadsTheSameWalkAsStepsTo pins the two against each other rather than against a
// literal. A rendering that drifted from the steps would still look right in the case above, since
// the expected string was written by reading the same fixture.
func TestRouteLineReadsTheSameWalkAsStepsTo(t *testing.T) {
	r, nets := chainReach()
	steps := r.StepsTo(nets["N2"])
	line := r.RouteLine(nets["N2"])
	if len(steps) == 0 {
		t.Fatal("no steps: the assertion below would be vacuous")
	}
	for _, s := range steps {
		if !strings.Contains(line, s.Through) || !strings.Contains(line, s.From) {
			t.Errorf("RouteLine(N2) = %q, missing step %s through %s", line, s.From, s.Through)
		}
	}
}
