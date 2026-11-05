package check

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// The board tier of the Model (WS3-008) holds net-grouped views over the board-geometry
// sidecar (geom.BoardGeometry, WS1-006). As with the netlist facts, rules read these
// through the Model interface and never the raw sidecar, so a future spatially-indexed
// implementation (WS3-004) can replace this one without touching a rule. A model built
// without a board (no WithBoard) yields an empty set, so geometric rules are silent by
// construction on netlist-only designs. Catalog-level gating is Available's "board."
// read-prefix rule.

// WithBoard attaches the board-geometry tier. A nil bg is the same as omitting the option. The model
// reports HasBoard false and every board rule is not-applicable.
func WithBoard(bg *geom.BoardGeometry) ModelOption {
	return func(m *irModel) { m.board = bg }
}

// attachBoard builds the board tier from the geometry that WithBoard staged. It runs after the
// netlist half of the model is built.
func (m *irModel) attachBoard() {
	bg := m.board
	if bg == nil {
		return
	}
	m.hasBoard = true
	for _, nc := range bg.Nets {
		bn := BoardNet{Net: nc.Net}
		// An arc track becomes the chords of geomath.TrackPath, so the width and clearance rules,
		// which measure straight runs, see the curve rather than the chord across it.
		for _, s := range nc.Segments {
			pts := geomath.TrackPath(s)
			for i := 1; i < len(pts); i++ {
				bn.Segments = append(bn.Segments, BoardSeg{Layer: s.Layer, A: pts[i-1], B: pts[i], Width: s.Width})
			}
		}
		for _, v := range nc.Vias {
			bn.Vias = append(bn.Vias, BoardVia{At: v.At, Size: v.Size, Drill: v.Drill, LayerFrom: v.LayerFrom, LayerTo: v.LayerTo})
		}
		for _, sh := range nc.Shapes {
			bs := BoardShape{Layer: sh.Layer, Outline: sh.GetOutline().GetPoints()}
			for _, h := range sh.Holes {
				bs.Holes = append(bs.Holes, h.GetPoints())
			}
			bn.Shapes = append(bn.Shapes, bs)
		}
		m.boardNets = append(m.boardNets, bn)
	}
	for _, z := range bg.Zones {
		m.boardZones = append(m.boardZones, BoardZone{Net: z.Net, Layer: z.Layer, Outline: z.GetOutline().GetPoints()})
	}
}
