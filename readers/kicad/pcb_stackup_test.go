package kicad

import (
	"bytes"
	"slices"
	"testing"

	"github.com/panyam/agni/core/stackup"
)

func TestBoardStackupReadsEachPly(t *testing.T) {
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board_stackup.kicad_pcb")), "board_stackup.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	s := g.GetStackup()
	if s.Thickness == nil || s.GetThickness() != 1_600_000 {
		t.Fatalf("board thickness = %v, want 1600000 nm", s.Thickness)
	}
	var got []string
	for _, l := range s.GetLayers() {
		got = append(got, stackup.Describe(l, g.GetUnitNm()))
	}
	want := []string{
		"F.SilkS silk thickness unknown",
		"F.Mask mask 0.01 mm",
		"F.Cu copper 0.035 mm",
		"dielectric 1 dielectric 0.1 mm PR1080 prepreg er=3.96 tand=0.02",
		"dielectric 1 dielectric 0.12 mm PR2116 prepreg er=4.1 tand=0.02",
		"In1.Cu copper 0.018 mm",
		"dielectric 2 dielectric 1 mm FR4 core er=4.5 tand=0.02",
		"In2.Cu copper 0.018 mm",
		"dielectric 3 dielectric 0.2 mm prepreg",
		"B.Cu copper 0.035 mm",
		"B.Mask mask 0.01 mm",
		"B.SilkS silk thickness unknown",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	for _, l := range s.GetLayers() {
		if l.GetKind() == "dielectric" && l.GetLayer() != "" {
			t.Errorf("dielectric %q names layer-table row %q", l.GetName(), l.GetLayer())
		}
		if l.GetKind() == "copper" && l.GetLayer() != l.GetName() {
			t.Errorf("copper %q names layer-table row %q", l.GetName(), l.GetLayer())
		}
	}
	if msg := stackup.Check(s, g.GetUnitNm()); msg != "" {
		t.Errorf("Check = %q, want agreement (1.546 mm of 1.6 mm)", msg)
	}
}

func TestBoardStackupBlindAndBuriedVias(t *testing.T) {
	g, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board_stackup.kicad_pcb")), "board_stackup.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	var spans []string
	for _, nc := range g.GetNets() {
		for _, v := range nc.GetVias() {
			spans = append(spans, v.GetLayerFrom()+".."+v.GetLayerTo())
		}
	}
	slices.Sort(spans)
	want := []string{"F.Cu..B.Cu", "F.Cu..In1.Cu", "In1.Cu..In2.Cu"}
	if !slices.Equal(spans, want) {
		t.Fatalf("via spans = %q, want %q", spans, want)
	}
}

func TestBoardWithNoStackupLeavesItUnset(t *testing.T) {
	g := readBoardFixture(t)
	if g.Stackup != nil {
		t.Fatalf("board.kicad_pcb has no stackup or thickness, read %v", g.Stackup)
	}
}
