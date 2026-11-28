package netgraph

import (
	"slices"
	"testing"
)

// A pin lost to an unresolved symbol leaves whatever was drawn to it alone at its point (agni issue
// 88). Each case puts one thing at the lost pin's point and asks whether LoneNets names its net.
func TestLoneNetsNamesWhatALostPinLeftAlone(t *testing.T) {
	p := func(x int64) Point { return Point{x, 0} }
	for _, tc := range []struct {
		name      string
		wires     []Wire
		anchors   []Anchor
		pins      []Pin
		terminals []Point
		want      []string
	}{
		{"a wire end", []Wire{{A: p(0), B: p(10)}}, nil, []Pin{{At: p(10), Comp: "R1", Pin: "1"}}, nil, []string{"N$1"}},
		{"a label on nothing", nil, []Anchor{{At: p(0), Label: "SDA"}}, nil, nil, []string{"SDA"}},
		{"a pin abutting the lost pin", nil, nil, []Pin{{At: p(0), Comp: "R1", Pin: "1"}}, nil, []string{"N$1"}},
		{"two wire ends meeting", []Wire{{A: p(0), B: p(10)}, {A: p(10), B: p(20)}}, []Anchor{{At: p(0), Label: "A"}, {At: p(20), Label: "A"}}, nil, nil, nil},
		{"a wire end on a junction", []Wire{{A: p(0), B: p(10)}}, []Anchor{{At: p(10), Label: "B"}}, nil, []Point{p(0)}, nil},
		{"a wire end on a pin", []Wire{{A: p(0), B: p(10)}}, nil, []Pin{{At: p(0), Comp: "R1", Pin: "1"}, {At: p(10), Comp: "R2", Pin: "1"}}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, points := BuildWithPoints(tc.wires, tc.anchors, tc.pins, tc.terminals)
			if got := LoneNets(tc.wires, tc.anchors, tc.pins, tc.terminals, points); !slices.Equal(got, tc.want) {
				t.Errorf("LoneNets = %v, want %v", got, tc.want)
			}
		})
	}
}
