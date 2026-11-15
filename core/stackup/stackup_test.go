package stackup

import (
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func nm(v int64) *int64 { return &v }

func stack(total *int64, rows ...*geom.StackLayer) *geom.Stackup {
	return &geom.Stackup{Thickness: total, Layers: rows}
}

func cu(t int64) *geom.StackLayer {
	return &geom.StackLayer{Name: "Cu", Kind: "copper", Thickness: nm(t)}
}
func di(t int64) *geom.StackLayer {
	return &geom.StackLayer{Name: "d", Kind: "dielectric", Thickness: nm(t)}
}

func TestCheck(t *testing.T) {
	cases := []struct {
		name string
		s    *geom.Stackup
		want string
	}{
		{"agrees", stack(nm(1_600_000), cu(35_000), di(1_530_000), cu(35_000)), ""},
		{"inside the tolerance", stack(nm(1_600_000), cu(35_000), di(1_400_000), cu(35_000)), ""},
		{"too thin", stack(nm(1_600_000), cu(35_000), di(1_000_000), cu(35_000)),
			"the layers sum to 1.07 mm, and the file states a board thickness of 1.6 mm"},
		{"too thick", stack(nm(1_000_000), cu(35_000), di(1_530_000), cu(35_000)),
			"the layers sum to 1.6 mm, and the file states a board thickness of 1 mm"},
		{"no stated total", stack(nil, cu(35_000), di(1_000_000), cu(35_000)), ""},
		{"a dielectric with no thickness", stack(nm(1_600_000), cu(35_000), &geom.StackLayer{Kind: "dielectric"}, cu(35_000)), ""},
		{"no layers", stack(nm(1_600_000)), ""},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		if got := Check(c.s, 1); got != c.want {
			t.Errorf("%s: Check = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSumLeavesOutSurfaceRowsWithNoThickness(t *testing.T) {
	s := stack(nil, &geom.StackLayer{Kind: "silk"}, cu(35_000), &geom.StackLayer{Kind: "dielectric"}, &geom.StackLayer{Kind: "paste"})
	sum, missing := Sum(s)
	if sum != 35_000 || missing != 1 {
		t.Fatalf("Sum = %d, %d missing; want 35000, 1 missing", sum, missing)
	}
}
