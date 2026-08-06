package check

import (
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// The board tier of the Model (WS3-008): net-grouped views over the board-geometry
// sidecar (geom.BoardGeometry, WS1-006), the same posture as the netlist facts — rules
// read these through the Model interface, never the raw sidecar, so a future
// spatially-indexed implementation (WS3-004) lands behind the same seam. A model built
// without a board (NewModel) yields an empty set, so geometric rules are silent by
// construction on netlist-only designs; catalog-level gating is Available's "board."
// read-prefix rule.

// NewModelWithBoard builds the default Model with the board-geometry tier attached.
// NewModel remains the netlist-only constructor; every existing caller is unchanged.
func NewModelWithBoard(d *ir.Design, bg *geom.BoardGeometry, opts ...ModelOption) Model {
	m := NewModel(d, opts...).(*irModel)
	if bg == nil {
		return m
	}
	m.hasBoard = true
	for _, nc := range bg.Nets {
		bn := BoardNet{Net: nc.Net}
		for _, s := range nc.Segments {
			bn.Segments = append(bn.Segments, BoardSeg{Layer: s.Layer, A: s.A, B: s.B, Width: s.Width})
		}
		for _, v := range nc.Vias {
			bn.Vias = append(bn.Vias, BoardVia{At: v.At, Size: v.Size, Drill: v.Drill})
		}
		m.boardNets = append(m.boardNets, bn)
	}
	return m
}
