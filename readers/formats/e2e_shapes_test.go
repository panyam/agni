package formats

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// shapeRows lists a read's copper shapes as "<copper layer position> <net> holes=<n>", sorted, so
// reads that name their layers differently compare.
func shapeRows(g *geom.BoardGeometry) []string {
	pos := map[string]int{}
	n := 0
	for _, l := range g.GetLayers() {
		switch strings.ToUpper(l.GetKind()) {
		case "SIGNAL", "POWER", "CONDUCTOR", "PLANE":
			pos[l.GetName()] = n
			n++
		}
	}
	if _, ok := pos["F.Cu"]; !ok {
		pos["F.Cu"] = 0
	}
	if _, ok := pos["B.Cu"]; !ok {
		pos["B.Cu"] = n - 1
	}
	var out []string
	for _, nc := range g.GetNets() {
		// IPC-2581 names a net the board leaves unnamed Unused_<id>, so both read as unnamed.
		net := nc.GetNet()
		if strings.HasPrefix(net, "Unused_") {
			net = ""
		}
		for _, s := range nc.GetShapes() {
			out = append(out, fmt.Sprintf("L%d %q holes=%d", pos[s.GetLayer()], net, len(s.GetHoles())))
		}
	}
	sort.Strings(out)
	return out
}

// TestCopperShapesAgreeAcrossParasdrsReads holds parasdr's filled copper across the three reads
// Allegro wrote (agni 1357). The board's ETCH shapes and the IPC-2581 contours must give the same
// shapes on the same layers and nets, with the same number of holes. The ODB++X read keeps only
// copper eda/data puts on a named net, so most of the board's shapes, which sit on unnamed nets,
// are missing from it (agni 1383). Each shape it does keep must be one of the board's.
func TestCopperShapesAgreeAcrossParasdrsReads(t *testing.T) {
	l := &Loader{}
	read := func(file string) []string {
		g, err := l.BoardGeometry(parasdr + file)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		return shapeRows(g)
	}
	board := read("parasdr_layout.brd")
	holes := 0
	for _, r := range board {
		var n int
		fmt.Sscanf(r[strings.LastIndex(r, "=")+1:], "%d", &n)
		holes += n
	}
	t.Logf("the board reads %d shapes with %d holes", len(board), holes)
	if len(board) < 5 || holes < 500 {
		t.Fatalf("the board reads %d shapes with %d holes, too few to mean anything", len(board), holes)
	}
	if diffs := diffRows("IPC-2581", board, read("parasdr_ipc2581.xml")); len(diffs) > 0 {
		t.Errorf("IPC-2581: %d differences:\n  %s", len(diffs), capLines(diffs))
	}
	left := map[string]int{}
	for _, r := range board {
		left[r]++
	}
	odx := read("parasdr_odb++.xml")
	for _, r := range odx {
		if left[r] == 0 {
			t.Errorf("the ODB++X read has %s, which the board has no more of", r)
			continue
		}
		left[r]--
	}
	t.Logf("the ODB++X read keeps %d of the board's %d shapes", len(odx), len(board))
	if len(odx) < 100 {
		t.Errorf("the ODB++X read keeps %d shapes, too few to mean anything", len(odx))
	}
}
