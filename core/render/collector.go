package render

import (
	"encoding/binary"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	geomath "github.com/panyam/agni/internal/geomath"
)

// wprim is one drawable primitive in world coordinates, before the rebase to a sheet-relative
// origin. Both the schematic packer (PackSheet) and the board packer (PackBoard) gather these.
type wprim struct {
	kind, group                    uint8
	refDes, net, netID, pin, busID string
	pts                            [][2]int64
}

// primCollector accumulates world-coordinate primitives and their bounds, then emits the
// tier-2 columnar form (rebased int32 vertex pool, fixed-width primitive records, and the
// PrimitiveKeys for picking). The gather-then-emit sequence is identical for schematics and
// boards, so it lives here once; the two packers differ only in what primitives they add and
// in the label/image tails they build around the shared vertex origin.
type primCollector struct {
	prims  []wprim
	bounds geomath.Bounds
}

// Add records one primitive, growing the bounds to include its points. Empty primitives are
// dropped so they never open a degenerate vertex range.
func (c *primCollector) Add(kind, group uint8, refDes, net, pin string, pts [][2]int64) {
	c.add(wprim{kind: kind, group: group, refDes: refDes, net: net, pin: pin, pts: pts})
}

// AddWire records a schematic wire primitive carrying its per-instance net id (WS9) alongside the
// net name, so a highlight can target one of two nets that share a name. Only wires carry a net id;
// every other primitive uses Add (net id empty).
func (c *primCollector) AddWire(group uint8, net, netID string, pts [][2]int64) {
	c.add(wprim{kind: primLineStrip, group: group, net: net, netID: netID, pts: pts})
}

// AddBus records a bus trunk/entry primitive (a filled quad) carrying its source id (the KiCad
// uuid), so a bus-not-modeled finding highlights its own bus (WS7-042b). A bus has no net, so
// busID is its only join key; every segment of one bus shares the id.
func (c *primCollector) AddBus(group uint8, busID string, pts [][2]int64) {
	c.add(wprim{kind: primTriangles, group: group, busID: busID, pts: pts})
}

func (c *primCollector) add(p wprim) {
	if len(p.pts) == 0 {
		return
	}
	for _, pt := range p.pts {
		c.bounds.Add(&geom.Point{X: pt[0], Y: pt[1]})
	}
	c.prims = append(c.prims, p)
}

// build rebases the gathered primitives to the bounds' min corner (so GLSL ES's 32-bit
// vertex attributes stay exact) and serializes them into the vertex pool, primitive records,
// and picking keys. ox,oy are the rebase origin the caller also applies to labels and images.
func (c *primCollector) build() (vertices, records []byte, keys []*geom.PrimitiveKey, ox, oy int64) {
	if c.bounds.Valid() {
		ox, oy = c.bounds.Min()
	}
	npts := 0
	for _, p := range c.prims {
		npts += len(p.pts)
	}
	vertices = make([]byte, 0, npts*8)
	records = make([]byte, 0, len(c.prims)*primRecordBytes)
	vcount := uint32(0)
	for i, p := range c.prims {
		first := vcount
		for _, pt := range p.pts {
			vertices = binary.LittleEndian.AppendUint32(vertices, uint32(int32(pt[0]-ox)))
			vertices = binary.LittleEndian.AppendUint32(vertices, uint32(int32(pt[1]-oy)))
			vcount++
		}
		records = appendRecord(records, p.kind, p.group, first, uint32(len(p.pts)))
		if p.refDes != "" || p.net != "" || p.busID != "" {
			keys = append(keys, &geom.PrimitiveKey{Primitive: uint32(i), RefDes: p.refDes, Net: p.net, NetId: p.netID, Pin: p.pin, BusId: p.busID})
		}
	}
	return vertices, records, keys, ox, oy
}
