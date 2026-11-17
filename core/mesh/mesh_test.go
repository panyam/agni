package mesh

import (
	"fmt"
	"math"
	"testing"
)

// sphere is a UV sphere of radius r with n bands, about 2n² triangles.
func sphere(r float64, n int) *Mesh {
	var p Part
	p.Color = [4]float64{0.2, 0.4, 0.6, 1}
	for i := 0; i <= n; i++ {
		th := math.Pi * float64(i) / float64(n)
		for j := 0; j < n; j++ {
			ph := 2 * math.Pi * float64(j) / float64(n)
			p.Positions = append(p.Positions, [3]float64{r * math.Sin(th) * math.Cos(ph), r * math.Sin(th) * math.Sin(ph), r * math.Cos(th)})
		}
	}
	at := func(i, j int) uint32 { return uint32(i*n + j%n) }
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			p.Indices = append(p.Indices, at(i, j), at(i+1, j), at(i+1, j+1), at(i, j), at(i+1, j+1), at(i, j+1))
		}
	}
	return &Mesh{Parts: []Part{p}}
}

func TestSimplifyFitsTheBudgetAndKeepsTheShape(t *testing.T) {
	m := sphere(5, 120)
	full := m.Triangles()
	if full < 20000 {
		t.Fatalf("the test sphere has %d triangles, want at least 20000", full)
	}
	lo, hi, _ := m.Bounds()
	for _, max := range []int{10000, 2000, 300} {
		s := Simplify(m, max)
		got := s.Triangles()
		if got > max || got < max/4 {
			t.Errorf("Simplify(%d) gave %d triangles, want between %d and %d", max, got, max/4, max)
		}
		slo, shi, ok := s.Bounds()
		if !ok {
			t.Fatalf("Simplify(%d) gave an empty mesh", max)
		}
		// Each merged vertex is a mean of its cell, so the box only shrinks, and by under 15% of the
		// radius even at 300 triangles.
		for k := 0; k < 3; k++ {
			if slo[k] < lo[k]-1e-9 || shi[k] > hi[k]+1e-9 {
				t.Errorf("Simplify(%d) grew the box on axis %d: %v..%v outside %v..%v", max, k, slo[k], shi[k], lo[k], hi[k])
			}
			if lo[k]-slo[k] < -0.75 || shi[k]-hi[k] < -0.75 {
				t.Errorf("Simplify(%d) shrank axis %d to %v..%v from %v..%v", max, k, slo[k], shi[k], lo[k], hi[k])
			}
		}
		if s.Parts[0].Color != m.Parts[0].Color {
			t.Errorf("Simplify(%d) changed the colour to %v", max, s.Parts[0].Color)
		}
		for _, v := range s.Parts[0].Indices {
			if int(v) >= len(s.Parts[0].Positions) {
				t.Fatalf("Simplify(%d) left index %d past %d positions", max, v, len(s.Parts[0].Positions))
			}
		}
	}
	if m.Triangles() != full {
		t.Errorf("Simplify changed its input from %d to %d triangles", full, m.Triangles())
	}
}

func TestSimplifyReturnsAMeshWithinTheBudgetUnchanged(t *testing.T) {
	m := sphere(1, 10)
	if s := Simplify(m, m.Triangles()); s != m {
		t.Errorf("Simplify copied a mesh already within budget")
	}
	if s := Simplify(m, 0); s.Triangles() != 0 {
		t.Errorf("Simplify(0) gave %d triangles", s.Triangles())
	}
}

func items(sizes ...int) []Item {
	var out []Item
	for i, n := range sizes {
		// sphere(1, b) has 2b² triangles.
		b := int(math.Round(math.Sqrt(float64(n) / 2)))
		out = append(out, Item{Key: fmt.Sprintf("m%d", i), Mesh: sphere(1, b), Footprint: float64(i + 1)})
	}
	return out
}

func TestFitCapsEachMeshThenTheLargestFirst(t *testing.T) {
	in := items(800, 2000, 20000) // 800, 2048 and 20000 triangles after rounding the band count
	tri := func(p Plan, k string) int { return p.Meshes[k].Triangles() }

	p := Fit(in, 100000, 1000000)
	if len(p.Simplified) != 0 || len(p.Blocks) != 0 || p.Meshes["m2"] != in[2].Mesh {
		t.Errorf("a board within budget lost detail: simplified %v, blocks %v", p.Simplified, p.Blocks)
	}

	p = Fit(in, 5000, 1000000)
	if fmt.Sprint(p.Simplified) != "[m2]" || tri(p, "m2") > 5000 {
		t.Errorf("the per-mesh cap simplified %v, m2 has %d triangles, want only m2 at 5000 or fewer", p.Simplified, tri(p, "m2"))
	}

	p = Fit(in, 100000, 6000)
	if p.Triangles > 6000 || len(p.Blocks) != 0 {
		t.Errorf("Fit(6000) kept %d triangles with blocks %v", p.Triangles, p.Blocks)
	}
	// 6000 - 800 - 2048 leaves a cap of 3152 for m2, and m0 and m1 sit under it.
	if p.Meshes["m0"] != in[0].Mesh || p.Meshes["m1"] != in[1].Mesh {
		t.Errorf("Fit(6000) simplified a mesh under the cap: %v", p.Simplified)
	}
	if fmt.Sprint(p.Simplified) != "[m2]" || tri(p, "m2") > 3152 || tri(p, "m2") < 1500 {
		t.Errorf("Fit(6000) simplified %v and left m2 at %d, want only m2, near the cap of 3152", p.Simplified, tri(p, "m2"))
	}
}

func TestFitDropsTheSmallestPartsToBlocksLast(t *testing.T) {
	in := items(800, 800, 800, 800)
	p := Fit(in, 100000, 2*MinTriangles)
	if fmt.Sprint(p.Blocks) != "[m0 m1]" {
		t.Errorf("blocks %v, want the two smallest footprints, m0 then m1", p.Blocks)
	}
	if p.Triangles > 2*MinTriangles || len(p.Meshes) != 2 {
		t.Errorf("kept %d meshes with %d triangles, want 2 within %d", len(p.Meshes), p.Triangles, 2*MinTriangles)
	}
	p = Fit(in, 100000, 0)
	if len(p.Blocks) != 4 || len(p.Meshes) != 0 {
		t.Errorf("a zero budget kept %d meshes and blocked %v", len(p.Meshes), p.Blocks)
	}
}
