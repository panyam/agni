package geomath

import (
	"math"
	"math/rand"
	"testing"
)

// ringArea is a ring's area by the shoelace formula.
func ringArea(pts [][2]float64) float64 {
	s := 0.0
	for i := range pts {
		j := (i + 1) % len(pts)
		s += pts[i][0]*pts[j][1] - pts[j][0]*pts[i][1]
	}
	return math.Abs(s) / 2
}

// trianglesArea sums the area of earcut's triangles over the flattened vertices.
func trianglesArea(data []float64, tris []int) float64 {
	s := 0.0
	for i := 0; i+2 < len(tris); i += 3 {
		a, b, c := tris[i]*2, tris[i+1]*2, tris[i+2]*2
		s += math.Abs((data[b]-data[a])*(data[c+1]-data[a+1])-(data[c]-data[a])*(data[b+1]-data[a+1])) / 2
	}
	return s
}

func flatten(rings ...[][2]float64) ([]float64, []int) {
	var data []float64
	var holes []int
	for i, r := range rings {
		if i > 0 {
			holes = append(holes, len(data)/2)
		}
		for _, p := range r {
			data = append(data, p[0], p[1])
		}
	}
	return data, holes
}

func circle(cx, cy, r float64, n int, clockwise bool) [][2]float64 {
	var out [][2]float64
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		if clockwise {
			a = -a
		}
		out = append(out, [2]float64{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return out
}

// The triangles of a polygon cover its area less its holes', on a concave ring, a ring with a
// hole, and a plane-sized ring with hundreds of holes that takes the z-order path.
func TestEarcutCoversThePolygonLessItsHoles(t *testing.T) {
	square := func(x0, y0, x1, y1 float64) [][2]float64 {
		return [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
	}
	comb := [][2]float64{{0, 0}, {10, 0}, {10, 10}, {8, 10}, {8, 2}, {6, 2}, {6, 10}, {4, 10}, {4, 2}, {2, 2}, {2, 10}, {0, 10}}
	plane := square(0, 0, 1000, 600)
	rings := [][][2]float64{plane}
	want := ringArea(plane)
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		for j := 0; j < 12; j++ {
			h := circle(40+float64(i)*48+rng.Float64()*4, 40+float64(j)*45+rng.Float64()*4, 12, 16, j%2 == 0)
			rings = append(rings, h)
			want -= ringArea(h)
		}
	}
	cases := []struct {
		name  string
		rings [][][2]float64
		want  float64
	}{
		{"concave comb", [][][2]float64{comb}, ringArea(comb)},
		{"square with a hole", [][][2]float64{square(0, 0, 10, 10), square(3, 3, 7, 6)}, 100 - 12},
		{"plane with 240 holes", rings, want},
	}
	for _, c := range cases {
		data, holes := flatten(c.rings...)
		tris := Earcut(data, holes)
		if len(tris)%3 != 0 || len(tris) == 0 {
			t.Fatalf("%s: %d indices", c.name, len(tris))
		}
		if got := trianglesArea(data, tris); math.Abs(got-c.want) > 1e-6*c.want {
			t.Errorf("%s: triangles cover %.6f, want %.6f", c.name, got, c.want)
		}
	}
}

func TestEarcutOfADegenerateRingIsEmpty(t *testing.T) {
	if tris := Earcut([]float64{0, 0, 1, 1}, nil); len(tris) != 0 {
		t.Errorf("two points gave %v", tris)
	}
}
