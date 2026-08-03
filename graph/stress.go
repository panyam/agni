package graph

import (
	"maps"
	"math"
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// stressIterations is the fixed SMACOF sweep count. Fixed on purpose: majorization is
// monotone (each sweep can only lower stress), so a constant budget gives deterministic
// output with no convergence-threshold wobble. 128 is far past visual convergence for
// corpus-sized sheets.
const stressIterations = 128

// stressPlace is the stress-majorization layout (WS7-009): positions are chosen so drawn
// distance matches graph-theoretic distance (BFS hops over shared nets), by minimizing
// stress(X) = sum w_ij (||xi-xj|| - pitch*d_ij)^2 with w_ij = d_ij^-2. Each connected
// component is solved independently (d_ij is undefined across components) from the layered
// placement as a deterministic init, majorized a fixed number of sweeps, then snapped to
// free pitch cells so compactBySize's grid-alignment assumption keeps holding. Components
// pack left to right in sorted-first-ref order.
func stressPlace(d *ir.Design) Placement {
	adj := adjacency(d)
	init := layeredPlace(d).Positions

	pos := map[string]*geom.Point{}
	var offsetX int64
	for _, comp := range connectedComponents(adj) {
		placed := stressComponent(comp, adj, init)
		snapped := snapToGrid(comp, placed)
		maxX := shiftInto(snapped, offsetX)
		maps.Copy(pos, snapped)
		offsetX = maxX + 2*pitch
	}
	return Placement{Positions: pos}
}

// connectedComponents splits the component graph into its connected pieces, each sorted,
// ordered by their first ref so the output is stable.
func connectedComponents(adj map[string]map[string]bool) [][]string {
	refs := make([]string, 0, len(adj))
	for r := range adj {
		refs = append(refs, r)
	}
	sort.Strings(refs)
	seen := map[string]bool{}
	var comps [][]string
	for _, start := range refs {
		if seen[start] {
			continue
		}
		var comp []string
		queue := []string{start}
		seen[start] = true
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			comp = append(comp, cur)
			next := make([]string, 0, len(adj[cur]))
			for n := range adj[cur] {
				next = append(next, n)
			}
			sort.Strings(next)
			for _, n := range next {
				if !seen[n] {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		sort.Strings(comp)
		comps = append(comps, comp)
	}
	return comps
}

// bfsDistances returns d(src, *) in hops over the component graph, -1 for unreachable.
func bfsDistances(src string, refs []string, adj map[string]map[string]bool) map[string]int {
	dist := make(map[string]int, len(refs))
	for _, r := range refs {
		dist[r] = -1
	}
	dist[src] = 0
	queue := []string{src}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		next := make([]string, 0, len(adj[cur]))
		for n := range adj[cur] {
			next = append(next, n)
		}
		sort.Strings(next)
		for _, n := range next {
			if dist[n] == -1 {
				dist[n] = dist[cur] + 1
				queue = append(queue, n)
			}
		}
	}
	return dist
}

// stressComponent runs the fixed-budget SMACOF sweeps over one connected component and
// returns float positions in layout units (target distance for a pair is pitch * d_ij).
func stressComponent(refs []string, adj map[string]map[string]bool, init map[string]*geom.Point) map[string][2]float64 {
	n := len(refs)
	out := make(map[string][2]float64, n)
	if n == 0 {
		return out
	}
	if n == 1 {
		out[refs[0]] = [2]float64{0, 0}
		return out
	}

	// All-pairs targets and weights: target t = pitch*d, weight w = 1/d^2.
	idx := make(map[string]int, n)
	for i, r := range refs {
		idx[r] = i
	}
	target := make([][]float64, n)
	weight := make([][]float64, n)
	for i, r := range refs {
		target[i] = make([]float64, n)
		weight[i] = make([]float64, n)
		for peer, dd := range bfsDistances(r, refs, adj) {
			j, ok := idx[peer]
			if !ok || dd <= 0 {
				continue
			}
			target[i][j] = float64(dd) * pitch
			weight[i][j] = 1 / float64(dd*dd)
		}
	}

	// Init from the layered placement: deterministic, and already globally reasonable, so
	// majorization refines instead of rescuing. Coincident inits get an index-proportional
	// nudge so vote directions are defined.
	x := make([]float64, n)
	y := make([]float64, n)
	for i, r := range refs {
		if p := init[r]; p != nil {
			x[i], y[i] = float64(p.X), float64(p.Y)
		}
		x[i] += float64(i) * 1e-3
	}

	// SMACOF sweeps: each node moves to the weighted average of its pairwise votes
	// (x_j + t_ij * unit(x_i - x_j)): "sit at exactly the target distance from j, in my
	// current direction". Majorization guarantees stress is non-increasing per sweep.
	for range stressIterations {
		nx := make([]float64, n)
		ny := make([]float64, n)
		for i := range n {
			var sx, sy, sw float64
			for j := range n {
				w := weight[i][j]
				if w == 0 {
					continue
				}
				dx, dy := x[i]-x[j], y[i]-y[j]
				dd := math.Hypot(dx, dy)
				if dd < 1e-9 {
					dx, dy, dd = 1, 0, 1 // coincident: pick a fixed direction
				}
				sx += w * (x[j] + target[i][j]*dx/dd)
				sy += w * (y[j] + target[i][j]*dy/dd)
				sw += w
			}
			nx[i], ny[i] = sx/sw, sy/sw
		}
		x, y = nx, ny
	}
	for i, r := range refs {
		out[r] = [2]float64{x[i], y[i]}
	}
	return out
}

// snapToGrid maps continuous positions onto free pitch cells: each ref (sorted order) takes
// the cell nearest its position, spiraling outward deterministically when the cell is taken.
// Grid alignment is what lets compactBySize treat distinct X as columns and distinct Y as
// rows, the same contract grid and layered satisfy.
func snapToGrid(refs []string, placed map[string][2]float64) map[string]*geom.Point {
	taken := map[[2]int64]bool{}
	out := make(map[string]*geom.Point, len(refs))
	for _, r := range refs {
		p := placed[r]
		cx := int64(math.Round(p[0] / pitch))
		cy := int64(math.Round(p[1] / pitch))
		gx, gy := cx, cy
		for ring := int64(1); taken[[2]int64{gx, gy}]; ring++ {
			found := false
			for _, c := range ringCells(cx, cy, ring) {
				if !taken[c] {
					gx, gy = c[0], c[1]
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		taken[[2]int64{gx, gy}] = true
		out[r] = &geom.Point{X: gx * pitch, Y: gy * pitch}
	}
	return out
}

// ringCells enumerates the cells at Chebyshev distance ring from (cx,cy) in a fixed order
// (top row, then bottom row, then the side columns), so collision resolution is deterministic.
func ringCells(cx, cy, ring int64) [][2]int64 {
	var cells [][2]int64
	for dx := -ring; dx <= ring; dx++ {
		cells = append(cells, [2]int64{cx + dx, cy - ring}, [2]int64{cx + dx, cy + ring})
	}
	for dy := -ring + 1; dy <= ring-1; dy++ {
		cells = append(cells, [2]int64{cx - ring, cy + dy}, [2]int64{cx + ring, cy + dy})
	}
	return cells
}

// shiftInto translates a component's cells so its leftmost cell starts at offsetX, and
// returns the new rightmost X (for packing the next component).
func shiftInto(pos map[string]*geom.Point, offsetX int64) int64 {
	minX := int64(math.MaxInt64)
	maxX := int64(math.MinInt64)
	for _, p := range pos {
		minX = min(minX, p.X)
	}
	for _, p := range pos {
		p.X += offsetX - minX
		maxX = max(maxX, p.X)
	}
	return maxX
}

// StressOf scores positions against the design's graph distances: Kruskal's normalized
// stress-1, sqrt(sum w (s*delta - t)^2 / sum w t^2) with the optimal uniform scale s applied
// before scoring, so the number is scale-invariant (a layout is not penalized for its units)
// and comparable across strategies on the same design. Pairs in different connected
// components are skipped (their graph distance is undefined). 0 is a perfect embedding;
// higher is worse. Returns 0 for designs with fewer than two connected nodes.
func StressOf(d *ir.Design, positions map[string]*geom.Point) float64 {
	adj := adjacency(d)
	var num, den, cross float64 // cross = sum w t delta, den here = sum w delta^2
	var tsq float64
	type pair struct{ t, delta float64 }
	var pairs []pair
	for _, comp := range connectedComponents(adj) {
		for i, r := range comp {
			dist := bfsDistances(r, comp, adj)
			for j := i + 1; j < len(comp); j++ {
				dd := dist[comp[j]]
				pa, pb := positions[r], positions[comp[j]]
				if dd <= 0 || pa == nil || pb == nil {
					continue
				}
				t := float64(dd) * pitch
				delta := math.Hypot(float64(pa.X-pb.X), float64(pa.Y-pb.Y))
				pairs = append(pairs, pair{t, delta})
				w := 1 / (t * t) // weights in target units: w = (pitch*d)^-2, same ordering as d^-2
				cross += w * t * delta
				den += w * delta * delta
				tsq += w * t * t
			}
		}
	}
	if len(pairs) == 0 || den == 0 || tsq == 0 {
		return 0
	}
	s := cross / den // optimal uniform scale for the drawn distances
	num = 0
	for _, p := range pairs {
		w := 1 / (p.t * p.t)
		num += w * (s*p.delta - p.t) * (s*p.delta - p.t)
	}
	return math.Sqrt(num / tsq)
}

// PositionsByRef extracts a geometry's placement origins keyed by ref_des, across all
// sheets. Placements with no ref_des (power/flag virtuals) are skipped, so an auto-layout
// and faithful geometry join on the real components. It is the position form StressOf and
// GroundTruthResidual compare.
func PositionsByRef(g *geom.SchematicGeometry) map[string]*geom.Point {
	pos := map[string]*geom.Point{}
	for _, sh := range g.GetSheets() {
		for _, pl := range sh.GetPlacements() {
			if pl.GetRefDes() != "" && pl.GetTransform().GetOrigin() != nil {
				pos[pl.GetRefDes()] = pl.GetTransform().GetOrigin()
			}
		}
	}
	return pos
}

// MeasureWith is Measure plus the design-aware Stress score (Measure alone reads finished
// geometry, which does not carry graph distances). Use it wherever the design is at hand,
// e.g. the --compare table, so every strategy gets stress-scored, not just the stress layout.
func MeasureWith(d *ir.Design, g *geom.SchematicGeometry) Quality {
	q := Measure(g)
	q.Stress = StressOf(d, PositionsByRef(g))
	return q
}
