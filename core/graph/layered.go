package graph

import (
	"sort"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// layeredPlace is a layered (Sugiyama-style) placement. Components are ranked into rows by distance
// from a high-degree root, ordered within each row to reduce edge crossings, then given coordinates.
// Unlike the grid placer it uses connectivity, so connected components land near each other.
//
// It is rank plus barycenter ordering plus straight edges, with no dummy-node edge routing. Adjacency
// and roots are chosen in ref_des order and the sweep count is fixed, so a design always lays out
// identically.
//
// Netlists are heavily cyclic (feedback loops, power/ground nets), and that needs no cycle removal
// here. The component graph is undirected and ranking is by BFS distance, so there is no directed
// cycle to break. A cycle yields intra-layer or layer-skipping edges, drawn as-is.
func layeredPlace(d *ir.Design) Placement {
	adj := adjacency(d)

	refs := make([]string, 0, len(adj))
	for r := range adj {
		refs = append(refs, r)
	}
	sort.Strings(refs)

	layer := assignLayers(refs, adj)
	rows := orderRows(refs, layer, adj)

	pos := make(map[string]*geom.Point, len(refs))
	for l, row := range rows {
		for i, ref := range row {
			pos[ref] = &geom.Point{X: int64(i) * pitch, Y: -int64(l) * pitch}
		}
	}
	return Placement{Positions: pos}
}

// adjacency builds the component graph, where two components are adjacent if they share a net.
// Every component appears as a key (isolated ones map to an empty set), so the placement covers the
// whole design.
func adjacency(d *ir.Design) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(d.Components))
	for _, c := range d.Components {
		if adj[c.RefDes] == nil {
			adj[c.RefDes] = map[string]bool{}
		}
	}
	for _, net := range d.Nets {
		members := make([]string, 0, len(net.Connections))
		seen := map[string]bool{}
		for _, conn := range net.Connections {
			if _, ok := adj[conn.ComponentRef]; ok && !seen[conn.ComponentRef] {
				seen[conn.ComponentRef] = true
				members = append(members, conn.ComponentRef)
			}
		}
		for i := range members {
			for j := i + 1; j < len(members); j++ {
				adj[members[i]][members[j]] = true
				adj[members[j]][members[i]] = true
			}
		}
	}
	return adj
}

// assignLayers ranks each component by BFS distance from a root, processing connected
// components in turn. Roots are picked by highest degree (ties by ref_des), and neighbors
// are visited in ref_des order, so the ranking is deterministic. Each connected component
// starts its own layer 0.
func assignLayers(refs []string, adj map[string]map[string]bool) map[string]int {
	layer := make(map[string]int, len(refs))
	visited := map[string]bool{}

	// Roots by degree descending, then ref_des ascending.
	roots := append([]string(nil), refs...)
	sort.SliceStable(roots, func(i, j int) bool {
		di, dj := len(adj[roots[i]]), len(adj[roots[j]])
		if di != dj {
			return di > dj
		}
		return roots[i] < roots[j]
	})

	for _, root := range roots {
		if visited[root] {
			continue
		}
		visited[root] = true
		layer[root] = 0
		queue := []string{root}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			nbrs := make([]string, 0, len(adj[cur]))
			for n := range adj[cur] {
				nbrs = append(nbrs, n)
			}
			sort.Strings(nbrs)
			for _, n := range nbrs {
				if visited[n] {
					continue
				}
				visited[n] = true
				layer[n] = layer[cur] + 1
				queue = append(queue, n)
			}
		}
	}
	return layer
}

// orderRows groups components by layer and orders each row to reduce crossings with barycenter
// sweeps (a node moves toward the average position of its neighbors in the adjacent row). A fixed
// sweep count and a ref_des initial order keep the result stable.
func orderRows(refs []string, layer map[string]int, adj map[string]map[string]bool) [][]string {
	maxLayer := 0
	for _, l := range layer {
		if l > maxLayer {
			maxLayer = l
		}
	}
	rows := make([][]string, maxLayer+1)
	for _, r := range refs { // refs is ref_des-sorted, so rows start ordered
		rows[layer[r]] = append(rows[layer[r]], r)
	}

	posIn := func(row []string) map[string]int {
		m := make(map[string]int, len(row))
		for i, r := range row {
			m[r] = i
		}
		return m
	}
	// Barycenter of row against a fixed neighbor row. Nodes with no neighbor there keep their place.
	sweep := func(row []string, refRow map[string]int) {
		key := make(map[string]float64, len(row))
		for i, r := range row {
			sum, n := 0.0, 0
			for nb := range adj[r] {
				if idx, ok := refRow[nb]; ok {
					sum += float64(idx)
					n++
				}
			}
			if n > 0 {
				key[r] = sum / float64(n)
			} else {
				key[r] = float64(i)
			}
		}
		sort.SliceStable(row, func(a, b int) bool { return key[row[a]] < key[row[b]] })
	}

	const sweeps = 4
	for s := 0; s < sweeps; s++ {
		for l := 1; l <= maxLayer; l++ { // downward, against the row above
			sweep(rows[l], posIn(rows[l-1]))
		}
		for l := maxLayer - 1; l >= 0; l-- { // upward, against the row below
			sweep(rows[l], posIn(rows[l+1]))
		}
	}
	return rows
}
