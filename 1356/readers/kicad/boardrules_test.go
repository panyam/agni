package kicad

import (
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

func TestParseBoardRulesReadsTheDeclaredMinimums(t *testing.T) {
	pro := `{"board": {"design_settings": {"rules": {
		"min_track_width": 0.0969, "min_clearance": 0.1, "min_through_hole_diameter": 0.1,
		"min_via_annular_width": 0.125, "min_copper_edge_clearance": 0.3, "min_hole_clearance": 0.0}}}}`
	got := ParseBoardRules(strings.NewReader(pro))
	want := map[string]float64{"min_track_width": 0.0969, "min_clearance": 0.1, "min_through_hole_diameter": 0.1, "min_via_annular_width": 0.125}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

// Zero is KiCad's "no constraint", so it reads as undeclared, as do a missing block and a file that
// is not JSON at all.
func TestParseBoardRulesLeavesUndeclaredMinimumsOut(t *testing.T) {
	for name, pro := range map[string]string{
		"zero":      `{"board": {"design_settings": {"rules": {"min_track_width": 0}}}}`,
		"no block":  `{"net_settings": {}}`,
		"malformed": `not json`,
	} {
		if got := ParseBoardRules(strings.NewReader(pro)); len(got) != 0 {
			t.Errorf("%s: got %v, want nothing", name, got)
		}
	}
}

func TestAnnotateBoardRulesAddsOneConstraint(t *testing.T) {
	d := &ir.Design{}
	AnnotateBoardRules(d, nil)
	if len(d.Constraints) != 0 {
		t.Fatalf("annotated %v from no rules", d.Constraints)
	}
	AnnotateBoardRules(d, map[string]float64{"min_track_width": 0.0969})
	if len(d.Constraints) != 1 || d.Constraints[0].Kind != ConstraintKindBoardRules || d.Constraints[0].Params["min_track_width"] != "0.0969" {
		t.Errorf("constraints = %v", d.Constraints)
	}
}
