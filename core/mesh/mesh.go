// Package mesh holds a part model's triangles and fits them to a triangle budget (agni issue 1652).
//
// Say the stack view draws Jetson's board in 3D. Each part's model is a triangle mesh read from a
// .wrl or glTF file. One connector model alone has 270,000 triangles, and a weak laptop's browser
// runs out of memory long before the board is drawn. Simplify cuts a mesh down to a triangle count
// by vertex clustering. It snaps every vertex to a grid cell, merges the vertices in each cell, and
// drops the triangles that collapse. Fit spreads one budget over a board's distinct meshes. The
// largest meshes lose detail first, and the smallest parts fall back to plain blocks after that.
//
// A mesh never changes a part's height. Heights come from the full model before any simplification,
// so a check or a review reads the same numbers whatever budget a viewer asks for.
package mesh

import (
	"math"
	"sort"
)

// Part is one coloured triangle list. Indices name three positions per triangle. Color is RGBA in 0
// to 1.
type Part struct {
	Positions [][3]float64
	Indices   []uint32
	Color     [4]float64
}

// Mesh is a model's parts, in millimetres in the model's own frame.
type Mesh struct {
	Parts []Part
}

// Triangles counts the triangles of every part.
func (m *Mesh) Triangles() int {
	n := 0
	for _, p := range m.Parts {
		n += len(p.Indices) / 3
	}
	return n
}

// Points returns every position of every part.
func (m *Mesh) Points() [][3]float64 {
	var out [][3]float64
	for _, p := range m.Parts {
		out = append(out, p.Positions...)
	}
	return out
}

// Bounds returns the smallest box holding every position. ok is false for a mesh with none.
func (m *Mesh) Bounds() (lo, hi [3]float64, ok bool) {
	for _, p := range m.Parts {
		for _, q := range p.Positions {
			if !ok {
				lo, hi, ok = q, q, true
				continue
			}
			for k := 0; k < 3; k++ {
				lo[k] = math.Min(lo[k], q[k])
				hi[k] = math.Max(hi[k], q[k])
			}
		}
	}
	return lo, hi, ok
}

// Scale multiplies every position by s, in place.
func (m *Mesh) Scale(s float64) {
	for _, p := range m.Parts {
		for i, q := range p.Positions {
			p.Positions[i] = [3]float64{q[0] * s, q[1] * s, q[2] * s}
		}
	}
}

// Simplify returns a mesh of at most max triangles. A mesh already within max comes back unchanged
// and shared. Otherwise Simplify clusters vertices on a grid, and searches for the finest grid that
// fits. Each part keeps its colour, and a part whose triangles all collapse is dropped. The result's
// bounds stay within one grid cell of the original's, since each merged vertex is the mean of its
// cell. max below 1 gives an empty mesh.
func Simplify(m *Mesh, max int) *Mesh {
	if m.Triangles() <= max {
		return m
	}
	if max < 1 {
		return &Mesh{}
	}
	lo, hi, _ := m.Bounds()
	span := 0.0
	for k := 0; k < 3; k++ {
		span = math.Max(span, hi[k]-lo[k])
	}
	if span == 0 {
		return &Mesh{}
	}
	// The cell count along the longest side. Fewer cells give fewer triangles, so search for the
	// largest count whose result fits. One cell collapses every triangle, so it always fits.
	best := &Mesh{}
	a, b := 1, 4096
	for a <= b {
		n := (a + b) / 2
		out := cluster(m, lo, span/float64(n))
		if out.Triangles() <= max {
			best, a = out, n+1
		} else {
			b = n - 1
		}
	}
	return best
}

// cluster merges the vertices of each part that share a grid cell of side cell, anchored at lo.
func cluster(m *Mesh, lo [3]float64, cell float64) *Mesh {
	type key [3]int32
	out := &Mesh{}
	for _, p := range m.Parts {
		cellOf := make([]int32, len(p.Positions))
		ids := map[key]int32{}
		var sums [][3]float64
		var counts []float64
		for i, q := range p.Positions {
			var k key
			for d := 0; d < 3; d++ {
				k[d] = int32(math.Floor((q[d] - lo[d]) / cell))
			}
			id, ok := ids[k]
			if !ok {
				id = int32(len(sums)) // #nosec G115 -- a part has fewer vertices than its uint32 indices can name
				ids[k] = id
				sums = append(sums, [3]float64{})
				counts = append(counts, 0)
			}
			cellOf[i] = id
			sums[id] = [3]float64{sums[id][0] + q[0], sums[id][1] + q[1], sums[id][2] + q[2]}
			counts[id]++
		}
		seen := map[[3]int32]bool{}
		var idx []uint32
		for t := 0; t+2 < len(p.Indices); t += 3 {
			a, b, c := cellOf[p.Indices[t]], cellOf[p.Indices[t+1]], cellOf[p.Indices[t+2]]
			if a == b || b == c || a == c {
				continue
			}
			// The same triangle can arrive twice, from two triangles that collapsed onto one. Its key
			// is its vertices rotated to start at the smallest, which keeps the winding.
			k := [3]int32{a, b, c}
			for k[0] > k[1] || k[0] > k[2] {
				k = [3]int32{k[1], k[2], k[0]}
			}
			if seen[k] {
				continue
			}
			seen[k] = true
			idx = append(idx, uint32(a), uint32(b), uint32(c)) // #nosec G115 -- cell ids are 0 or more
		}
		if len(idx) == 0 {
			continue
		}
		// Keep only the vertices a triangle uses, renumbered in order of first use.
		renum := make([]int32, len(sums))
		for i := range renum {
			renum[i] = -1
		}
		var pos [][3]float64
		for i, v := range idx {
			if renum[v] < 0 {
				renum[v] = int32(len(pos)) // #nosec G115 -- a part has fewer vertices than its uint32 indices can name
				s, n := sums[v], counts[v]
				pos = append(pos, [3]float64{s[0] / n, s[1] / n, s[2] / n})
			}
			idx[i] = uint32(renum[v]) // #nosec G115 -- set just above, 0 or more
		}
		out.Parts = append(out.Parts, Part{Positions: pos, Indices: idx, Color: p.Color})
	}
	return out
}

// Item is one distinct mesh on a board, as Fit sees it. Footprint is its part's area on the board in
// square millimetres, and Fit turns the smallest parts into blocks first.
type Item struct {
	Key       string
	Mesh      *Mesh
	Footprint float64
}

// Plan says what Fit did to fit a budget. Meshes holds each kept mesh by key, simplified where Fit
// had to. Simplified names each mesh that lost triangles, and Blocks each one dropped to a block,
// smallest footprint first.
type Plan struct {
	Meshes     map[string]*Mesh
	Simplified []string
	Blocks     []string
	Triangles  int
}

// MinTriangles is the fewest triangles Fit simplifies a mesh to. Below it a clustered mesh stops
// looking like its part, and Fit draws the part as a block instead.
const MinTriangles = 200

// Fit fits items to perMesh triangles each and to budget triangles in all. It goes in three steps
// and stops at the first that fits.
//
//  1. Each mesh over perMesh is simplified to perMesh.
//  2. If the total is still over budget, Fit finds the largest cap that brings the total under it,
//     and simplifies every mesh over the cap down to it. So the largest meshes lose detail first,
//     and a small part's mesh is untouched until the cap reaches it.
//  3. If that cap would fall below MinTriangles, Fit uses MinTriangles, and drops meshes to blocks,
//     smallest footprint first, until the rest fit.
//
// A budget of 0 or less drops every mesh to a block. Fit never changes an item's Mesh.
func Fit(items []Item, perMesh, budget int) Plan {
	p := Plan{Meshes: map[string]*Mesh{}}
	if budget <= 0 {
		for _, it := range bySize(items) {
			p.Blocks = append(p.Blocks, it.Key)
		}
		return p
	}
	counts := make([]int, len(items))
	total := 0
	for i, it := range items {
		counts[i] = min(it.Mesh.Triangles(), perMesh)
		total += counts[i]
	}
	keep := make([]bool, len(items))
	for i := range keep {
		keep[i] = true
	}
	limit := perMesh
	if total > budget {
		limit = waterLevel(counts, keep, budget)
		if limit < MinTriangles {
			limit = MinTriangles
			sum := 0
			for i := range items {
				sum += min(counts[i], limit)
			}
			for _, i := range bySizeIndex(items) {
				if sum <= budget {
					break
				}
				keep[i] = false
				sum -= min(counts[i], limit)
				p.Blocks = append(p.Blocks, items[i].Key)
			}
		}
	}
	for i, it := range items {
		if !keep[i] {
			continue
		}
		m := it.Mesh
		if want := min(counts[i], limit); m.Triangles() > want {
			m = Simplify(m, want)
			p.Simplified = append(p.Simplified, it.Key)
		}
		p.Meshes[it.Key] = m
		p.Triangles += m.Triangles()
	}
	sort.Strings(p.Simplified)
	return p
}

// waterLevel finds the largest cap c with the sum of min(counts[i], c) over kept items at most
// budget.
func waterLevel(counts []int, keep []bool, budget int) int {
	a, b := 0, 0
	for i, n := range counts {
		if keep[i] {
			b = max(b, n)
		}
	}
	for a < b {
		c := (a + b + 1) / 2
		sum := 0
		for i, n := range counts {
			if keep[i] {
				sum += min(n, c)
			}
		}
		if sum <= budget {
			a = c
		} else {
			b = c - 1
		}
	}
	return a
}

// bySizeIndex orders items by footprint, smallest first, then by key.
func bySizeIndex(items []Item) []int {
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		x, y := items[idx[a]], items[idx[b]]
		if x.Footprint != y.Footprint {
			return x.Footprint < y.Footprint
		}
		return x.Key < y.Key
	})
	return idx
}

func bySize(items []Item) []Item {
	out := make([]Item, 0, len(items))
	for _, i := range bySizeIndex(items) {
		out = append(out, items[i])
	}
	return out
}
