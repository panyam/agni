package builtin

import (
	"fmt"
	"testing"

	"github.com/panyam/agni/core/check"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// benchBoard synthesizes nSegs track segments spread over nNets nets on two layers in a
// grid pitched safely above the clearance floor, so the benchmark measures the pairwise
// walk (with its bbox reject), not finding construction.
func benchBoard(nSegs, nNets int) *geom.BoardGeometry {
	g := &geom.BoardGeometry{UnitNm: 1}
	nets := make([]*geom.NetCopper, nNets)
	for i := range nets {
		nets[i] = &geom.NetCopper{Net: fmt.Sprintf("N%03d", i)}
	}
	layers := []string{"F.Cu", "B.Cu"}
	for i := range nSegs {
		row, col := i/50, i%50
		s := &geom.TrackSegment{
			A:     &geom.Point{X: int64(col) * 2_000_000, Y: int64(row) * 2_000_000},
			B:     &geom.Point{X: int64(col)*2_000_000 + 1_500_000, Y: int64(row) * 2_000_000},
			Width: 200_000,
			Layer: layers[i%2],
		}
		nc := nets[i%nNets]
		nc.Segments = append(nc.Segments, s)
	}
	g.Nets = nets
	return g
}

// BenchmarkCopperClearance is the standing evidence for the WS3-004 spatial-index
// question: the pairwise walk is O(S^2) with a cheap bbox reject. Corpus boards top out
// near 400 segments; the sizes below bracket where the naive walk stops being free.
func BenchmarkCopperClearance(b *testing.B) {
	for _, n := range []int{400, 2000, 10000} {
		b.Run(fmt.Sprintf("segs=%d", n), func(b *testing.B) {
			m := check.NewModelWithBoard(&ir.Design{}, benchBoard(n, 40))
			b.ResetTimer()
			for b.Loop() {
				copperClearance.Findings(m)
			}
		})
	}
}
