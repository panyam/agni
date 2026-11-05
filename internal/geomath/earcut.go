// This file ports mapbox's earcut (https://github.com/mapbox/earcut, v2.2.4) to Go.
//
// ISC License
//
// Copyright (c) 2016, Mapbox
//
// Permission to use, copy, modify, and/or distribute this software for any purpose
// with or without fee is hereby granted, provided that the above copyright notice
// and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND ISC DISCLAIMS ALL WARRANTIES WITH REGARD TO
// THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS.
// IN NO EVENT SHALL ISC BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR
// CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA
// OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
// ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

package geomath

import (
	"math"
	"sort"
)

// Earcut triangulates a polygon with holes, the way a GPU fills one (agni 1357). data holds the
// vertices as x, y pairs, the outer ring first and each hole after it, and holes lists the vertex
// index where each hole starts. It returns the triangles as vertex indices, three per triangle.
//
// It is ear clipping, with each hole bridged into the outer ring first, and a z-order index that
// keeps a ring of thousands of vertices fast. A ring that crosses itself still triangulates, though
// the triangles may cover a little more or less than the ring.
func Earcut(data []float64, holes []int) []int {
	const dim = 2
	hasHoles := len(holes) > 0
	outerLen := len(data)
	if hasHoles {
		outerLen = holes[0] * dim
	}
	outer := linkedList(data, 0, outerLen, dim, true)
	var tris []int
	if outer == nil || outer.next == outer.prev {
		return tris
	}
	if hasHoles {
		outer = eliminateHoles(data, holes, outer, dim)
	}
	var minX, minY, invSize float64
	// A ring past 80 points gets the z-order index.
	if len(data) > 80*dim {
		minX, minY = data[0], data[1]
		maxX, maxY := minX, minY
		for i := dim; i < outerLen; i += dim {
			x, y := data[i], data[i+1]
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
		}
		invSize = math.Max(maxX-minX, maxY-minY)
		if invSize != 0 {
			invSize = 32767 / invSize
		}
	}
	earcutLinked(outer, &tris, dim, minX, minY, invSize, 0)
	return tris
}

type ecNode struct {
	i            int
	x, y         float64
	prev, next   *ecNode
	z            int32
	prevZ, nextZ *ecNode
	steiner      bool
}

// linkedList makes a circular doubly linked list from the ring's points, in the winding asked for.
func linkedList(data []float64, start, end, dim int, clockwise bool) *ecNode {
	var last *ecNode
	if clockwise == (signedArea(data, start, end, dim) > 0) {
		for i := start; i < end; i += dim {
			last = insertNode(i, data[i], data[i+1], last)
		}
	} else {
		for i := end - dim; i >= start; i -= dim {
			last = insertNode(i, data[i], data[i+1], last)
		}
	}
	if last != nil && equals(last, last.next) {
		removeNode(last)
		last = last.next
	}
	return last
}

// filterPoints removes duplicate and collinear points.
func filterPoints(start, end *ecNode) *ecNode {
	if start == nil {
		return start
	}
	if end == nil {
		end = start
	}
	p := start
	for {
		again := false
		if !p.steiner && (equals(p, p.next) || area(p.prev, p, p.next) == 0) {
			removeNode(p)
			p, end = p.prev, p.prev
			if p == p.next {
				break
			}
			again = true
		} else {
			p = p.next
		}
		if !again && p == end {
			break
		}
	}
	return end
}

// earcutLinked clips ears off the ring until one triangle is left, curing it as it goes.
func earcutLinked(ear *ecNode, tris *[]int, dim int, minX, minY, invSize float64, pass int) {
	if ear == nil {
		return
	}
	if pass == 0 && invSize != 0 {
		indexCurve(ear, minX, minY, invSize)
	}
	stop := ear
	for ear.prev != ear.next {
		prev, next := ear.prev, ear.next
		var isEarNow bool
		if invSize != 0 {
			isEarNow = isEarHashed(ear, minX, minY, invSize)
		} else {
			isEarNow = isEar(ear)
		}
		if isEarNow {
			*tris = append(*tris, prev.i/dim, ear.i/dim, next.i/dim)
			removeNode(ear)
			// Skipping the next vertex leaves fewer sliver triangles.
			ear = next.next
			stop = next.next
			continue
		}
		ear = next
		if ear == stop {
			switch pass {
			case 0:
				earcutLinked(filterPoints(ear, nil), tris, dim, minX, minY, invSize, 1)
			case 1:
				ear = cureLocalIntersections(filterPoints(ear, nil), tris, dim)
				earcutLinked(ear, tris, dim, minX, minY, invSize, 2)
			case 2:
				splitEarcut(ear, tris, dim, minX, minY, invSize)
			}
			break
		}
	}
}

// isEar reports whether the triangle at ear has no other point of the ring inside it.
func isEar(ear *ecNode) bool {
	a, b, c := ear.prev, ear, ear.next
	if area(a, b, c) >= 0 {
		return false // reflex
	}
	ax, bx, cx, ay, by, cy := a.x, b.x, c.x, a.y, b.y, c.y
	x0, y0 := math.Min(ax, math.Min(bx, cx)), math.Min(ay, math.Min(by, cy))
	x1, y1 := math.Max(ax, math.Max(bx, cx)), math.Max(ay, math.Max(by, cy))
	for p := c.next; p != a; p = p.next {
		if p.x >= x0 && p.x <= x1 && p.y >= y0 && p.y <= y1 &&
			pointInTriangle(ax, ay, bx, by, cx, cy, p.x, p.y) && area(p.prev, p, p.next) >= 0 {
			return false
		}
	}
	return true
}

func isEarHashed(ear *ecNode, minX, minY, invSize float64) bool {
	a, b, c := ear.prev, ear, ear.next
	if area(a, b, c) >= 0 {
		return false
	}
	ax, bx, cx, ay, by, cy := a.x, b.x, c.x, a.y, b.y, c.y
	x0, y0 := math.Min(ax, math.Min(bx, cx)), math.Min(ay, math.Min(by, cy))
	x1, y1 := math.Max(ax, math.Max(bx, cx)), math.Max(ay, math.Max(by, cy))
	minZ := zOrder(x0, y0, minX, minY, invSize)
	maxZ := zOrder(x1, y1, minX, minY, invSize)
	p, n := ear.prevZ, ear.nextZ
	inside := func(q *ecNode) bool {
		return q.x >= x0 && q.x <= x1 && q.y >= y0 && q.y <= y1 && q != a && q != c &&
			pointInTriangle(ax, ay, bx, by, cx, cy, q.x, q.y) && area(q.prev, q, q.next) >= 0
	}
	for p != nil && p.z >= minZ && n != nil && n.z <= maxZ {
		if inside(p) {
			return false
		}
		p = p.prevZ
		if inside(n) {
			return false
		}
		n = n.nextZ
	}
	for p != nil && p.z >= minZ {
		if inside(p) {
			return false
		}
		p = p.prevZ
	}
	for n != nil && n.z <= maxZ {
		if inside(n) {
			return false
		}
		n = n.nextZ
	}
	return true
}

// cureLocalIntersections removes a small self-intersection by cutting its triangle off.
func cureLocalIntersections(start *ecNode, tris *[]int, dim int) *ecNode {
	p := start
	for {
		a, b := p.prev, p.next.next
		if !equals(a, b) && intersects(a, p, p.next, b) && locallyInside(a, b) && locallyInside(b, a) {
			*tris = append(*tris, a.i/dim, p.i/dim, b.i/dim)
			removeNode(p)
			removeNode(p.next)
			p, start = b, b
		}
		p = p.next
		if p == start {
			break
		}
	}
	return filterPoints(p, nil)
}

// splitEarcut splits the ring along a valid diagonal and triangulates each half.
func splitEarcut(start *ecNode, tris *[]int, dim int, minX, minY, invSize float64) {
	a := start
	for {
		b := a.next.next
		for b != a.prev {
			if a.i != b.i && isValidDiagonal(a, b) {
				c := splitPolygon(a, b)
				a = filterPoints(a, a.next)
				c = filterPoints(c, c.next)
				earcutLinked(a, tris, dim, minX, minY, invSize, 0)
				earcutLinked(c, tris, dim, minX, minY, invSize, 0)
				return
			}
			b = b.next
		}
		a = a.next
		if a == start {
			return
		}
	}
}

// eliminateHoles bridges each hole into the outer ring, leftmost hole first.
func eliminateHoles(data []float64, holes []int, outer *ecNode, dim int) *ecNode {
	var queue []*ecNode
	for i := range holes {
		start := holes[i] * dim
		end := len(data)
		if i < len(holes)-1 {
			end = holes[i+1] * dim
		}
		list := linkedList(data, start, end, dim, false)
		if list == nil {
			continue
		}
		if list == list.next {
			list.steiner = true
		}
		queue = append(queue, getLeftmost(list))
	}
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].x < queue[j].x })
	for _, h := range queue {
		outer = eliminateHole(h, outer)
	}
	return outer
}

func eliminateHole(hole, outer *ecNode) *ecNode {
	bridge := findHoleBridge(hole, outer)
	if bridge == nil {
		return outer
	}
	bridgeReverse := splitPolygon(bridge, hole)
	filterPoints(bridgeReverse, bridgeReverse.next)
	return filterPoints(bridge, bridge.next)
}

// findHoleBridge finds a vertex of the outer ring that the hole's leftmost point can join with a
// segment crossing nothing (David Eberly's algorithm).
func findHoleBridge(hole, outer *ecNode) *ecNode {
	p := outer
	hx, hy := hole.x, hole.y
	qx := math.Inf(-1)
	var m *ecNode
	for {
		if hy <= p.y && hy >= p.next.y && p.next.y != p.y {
			x := p.x + (hy-p.y)*(p.next.x-p.x)/(p.next.y-p.y)
			if x <= hx && x > qx {
				qx = x
				if p.x < p.next.x {
					m = p
				} else {
					m = p.next
				}
				if x == hx {
					return m // the hole touches the outer segment
				}
			}
		}
		p = p.next
		if p == outer {
			break
		}
	}
	if m == nil {
		return nil
	}
	stop := m
	mx, my := m.x, m.y
	tanMin := math.Inf(1)
	p = m
	for {
		if hx >= p.x && p.x >= mx && hx != p.x {
			var inside bool
			if hy < my {
				inside = pointInTriangle(hx, hy, mx, my, qx, hy, p.x, p.y)
			} else {
				inside = pointInTriangle(qx, hy, mx, my, hx, hy, p.x, p.y)
			}
			if inside {
				tan := math.Abs(hy-p.y) / (hx - p.x)
				if locallyInside(p, hole) && (tan < tanMin || (tan == tanMin && (p.x > m.x || (p.x == m.x && sectorContainsSector(m, p))))) {
					m = p
					tanMin = tan
				}
			}
		}
		p = p.next
		if p == stop {
			break
		}
	}
	return m
}

func sectorContainsSector(m, p *ecNode) bool {
	return area(m.prev, m, p.prev) < 0 && area(p.next, m, m.next) < 0
}

// indexCurve sets each node's z-order value and links the nodes in z order.
func indexCurve(start *ecNode, minX, minY, invSize float64) {
	p := start
	for {
		if p.z == 0 {
			p.z = zOrder(p.x, p.y, minX, minY, invSize)
		}
		p.prevZ = p.prev
		p.nextZ = p.next
		p = p.next
		if p == start {
			break
		}
	}
	p.prevZ.nextZ = nil
	p.prevZ = nil
	sortLinked(p)
}

// sortLinked sorts the z-order list by z, a merge sort over the linked list (Simon Tatham).
func sortLinked(list *ecNode) *ecNode {
	inSize := 1
	for {
		p := list
		list = nil
		var tail *ecNode
		numMerges := 0
		for p != nil {
			numMerges++
			q := p
			pSize := 0
			for i := 0; i < inSize; i++ {
				pSize++
				q = q.nextZ
				if q == nil {
					break
				}
			}
			qSize := inSize
			for pSize > 0 || (qSize > 0 && q != nil) {
				var e *ecNode
				if pSize != 0 && (qSize == 0 || q == nil || p.z <= q.z) {
					e = p
					p = p.nextZ
					pSize--
				} else {
					e = q
					q = q.nextZ
					qSize--
				}
				if tail != nil {
					tail.nextZ = e
				} else {
					list = e
				}
				e.prevZ = tail
				tail = e
			}
			p = q
		}
		tail.nextZ = nil
		inSize *= 2
		if numMerges <= 1 {
			return list
		}
	}
}

// zOrder interleaves a point's scaled coordinates into a Morton code.
func zOrder(x0, y0, minX, minY, invSize float64) int32 {
	x := int32((x0 - minX) * invSize)
	y := int32((y0 - minY) * invSize)
	x = (x | (x << 8)) & 0x00FF00FF
	x = (x | (x << 4)) & 0x0F0F0F0F
	x = (x | (x << 2)) & 0x33333333
	x = (x | (x << 1)) & 0x55555555
	y = (y | (y << 8)) & 0x00FF00FF
	y = (y | (y << 4)) & 0x0F0F0F0F
	y = (y | (y << 2)) & 0x33333333
	y = (y | (y << 1)) & 0x55555555
	return x | (y << 1)
}

func getLeftmost(start *ecNode) *ecNode {
	p, leftmost := start, start
	for {
		if p.x < leftmost.x || (p.x == leftmost.x && p.y < leftmost.y) {
			leftmost = p
		}
		p = p.next
		if p == start {
			break
		}
	}
	return leftmost
}

func pointInTriangle(ax, ay, bx, by, cx, cy, px, py float64) bool {
	return (cx-px)*(ay-py) >= (ax-px)*(cy-py) &&
		(ax-px)*(by-py) >= (bx-px)*(ay-py) &&
		(bx-px)*(cy-py) >= (cx-px)*(by-py)
}

// isValidDiagonal reports whether a diagonal from a to b lies inside the ring and crosses nothing.
func isValidDiagonal(a, b *ecNode) bool {
	return a.next.i != b.i && a.prev.i != b.i && !intersectsPolygon(a, b) &&
		(locallyInside(a, b) && locallyInside(b, a) && middleInside(a, b) &&
			(area(a.prev, a, b.prev) != 0 || area(a, b.prev, b) != 0) ||
			equals(a, b) && area(a.prev, a, a.next) > 0 && area(b.prev, b, b.next) > 0)
}

// area is twice the signed area of the triangle p, q, r.
func area(p, q, r *ecNode) float64 {
	return (q.y-p.y)*(r.x-q.x) - (q.x-p.x)*(r.y-q.y)
}

func equals(p1, p2 *ecNode) bool { return p1.x == p2.x && p1.y == p2.y }

func intersects(p1, q1, p2, q2 *ecNode) bool {
	o1, o2 := sign(area(p1, q1, p2)), sign(area(p1, q1, q2))
	o3, o4 := sign(area(p2, q2, p1)), sign(area(p2, q2, q1))
	if o1 != o2 && o3 != o4 {
		return true
	}
	return o1 == 0 && onSegment(p1, p2, q1) || o2 == 0 && onSegment(p1, q2, q1) ||
		o3 == 0 && onSegment(p2, p1, q2) || o4 == 0 && onSegment(p2, q1, q2)
}

func onSegment(p, q, r *ecNode) bool {
	return q.x <= math.Max(p.x, r.x) && q.x >= math.Min(p.x, r.x) && q.y <= math.Max(p.y, r.y) && q.y >= math.Min(p.y, r.y)
}

func sign(v float64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func intersectsPolygon(a, b *ecNode) bool {
	p := a
	for {
		if p.i != a.i && p.next.i != a.i && p.i != b.i && p.next.i != b.i && intersects(p, p.next, a, b) {
			return true
		}
		p = p.next
		if p == a {
			return false
		}
	}
}

func locallyInside(a, b *ecNode) bool {
	if area(a.prev, a, a.next) < 0 {
		return area(a, b, a.next) >= 0 && area(a, a.prev, b) >= 0
	}
	return area(a, b, a.prev) < 0 || area(a, a.next, b) < 0
}

// middleInside reports whether the middle of the diagonal a, b is inside the ring.
func middleInside(a, b *ecNode) bool {
	p := a
	inside := false
	px, py := (a.x+b.x)/2, (a.y+b.y)/2
	for {
		if (p.y > py) != (p.next.y > py) && p.next.y != p.y && px < (p.next.x-p.x)*(py-p.y)/(p.next.y-p.y)+p.x {
			inside = !inside
		}
		p = p.next
		if p == a {
			return inside
		}
	}
}

// splitPolygon links a to b with a bridge, splitting the ring in two or joining a hole to it.
func splitPolygon(a, b *ecNode) *ecNode {
	a2 := &ecNode{i: a.i, x: a.x, y: a.y}
	b2 := &ecNode{i: b.i, x: b.x, y: b.y}
	an, bp := a.next, b.prev
	a.next = b
	b.prev = a
	a2.next = an
	an.prev = a2
	b2.next = a2
	a2.prev = b2
	bp.next = b2
	b2.prev = bp
	return b2
}

func insertNode(i int, x, y float64, last *ecNode) *ecNode {
	p := &ecNode{i: i, x: x, y: y}
	if last == nil {
		p.prev = p
		p.next = p
	} else {
		p.next = last.next
		p.prev = last
		last.next.prev = p
		last.next = p
	}
	return p
}

func removeNode(p *ecNode) {
	p.next.prev = p.prev
	p.prev.next = p.next
	if p.prevZ != nil {
		p.prevZ.nextZ = p.nextZ
	}
	if p.nextZ != nil {
		p.nextZ.prevZ = p.prevZ
	}
}

func signedArea(data []float64, start, end, dim int) float64 {
	sum := 0.0
	for i, j := start, end-dim; i < end; i += dim {
		sum += (data[j] - data[i]) * (data[i+1] + data[j+1])
		j = i
	}
	return sum
}
