package sexpr

import (
	"strings"
	"testing"
)

// A list is left out by the value of its direct child, a quoted value as well as a bare one, a nested
// child of the same name does not count, and a list without the child is kept (agni issue 1005).
func TestSkipWhenDecidesByAChildsValue(t *testing.T) {
	const src = `(fp (g (start 0 0) (layer "User.9") (s "a) (")) (g (layer F.SilkS)) (g (start 1 1))
		(g (inner (layer User.9))) (h (layer User.9)))`
	root, err := ParseWith(strings.NewReader(src), KiCadStrings, Skips{When: []SkipWhen{{
		Head: "g", Child: "layer", Drop: func(v string) bool { return strings.HasPrefix(v, "User.") },
	}}})
	if err != nil {
		t.Fatal(err)
	}
	gs := root.Children("g")
	if len(gs) != 3 {
		t.Fatalf("kept %d g lists, want 3 (silk, no layer, nested layer)", len(gs))
	}
	if got := gs[0].Child("layer").Arg(1).Text(); got != "F.SilkS" {
		t.Errorf("first kept g is on %q, want F.SilkS", got)
	}
	if len(root.Children("h")) != 1 {
		t.Error("a list with another head was left out")
	}
	full, err := Parse(strings.NewReader(src), KiCadStrings)
	if err != nil {
		t.Fatal(err)
	}
	if got := full.Children("g")[1].Child("layer").Arg(1).Text(); got != gs[0].Child("layer").Arg(1).Text() {
		t.Errorf("a kept list parses differently with the rule, %q against %q", gs[0].Child("layer").Arg(1).Text(), got)
	}
}
