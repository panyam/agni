package main

import (
	"path/filepath"
	"testing"
)

// TestRunMeasuresTheTutorialBoard keeps the bench honest about what it measured: a run that read
// nothing, ran no rules or answered no query would print fast, plausible timings over an empty board.
func TestRunMeasuresTheTutorialBoard(t *testing.T) {
	r, err := Run(filepath.Join("..", "..", "examples", "tutorial-project"), "designs/gateway")
	if err != nil {
		t.Fatal(err)
	}
	if r.Components == 0 || r.Nets == 0 || r.Rules == 0 || r.Findings == 0 || r.QueryRows == 0 || r.Files == 0 {
		t.Errorf("the bench measured an empty run: %+v", r)
	}
}

// TestLayersMeasuresEachLayer keeps --layers from printing a split over layers it never built: each
// step must allocate, the order must be the one a Run checks needs, and the restored pass must have
// come from the store rather than a read.
func TestLayersMeasuresEachLayer(t *testing.T) {
	r, err := Layers(filepath.Join("..", "..", "examples", "tutorial-project"), "designs/gateway")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"design files", "design", "schematic geometry", "check model without the board (probe, dropped)", "board geometry", "check model", "fact base", "after a check run"}
	if len(r.Fresh) != len(want) || len(r.Restored) != len(want)-1 {
		t.Fatalf("fresh %d layers, restored %d; want %d and %d", len(r.Fresh), len(r.Restored), len(want), len(want)-1)
	}
	for i, l := range r.Fresh {
		if l.Name != want[i] {
			t.Errorf("fresh layer %d is %q, want %q", i, l.Name, want[i])
		}
		if l.AllocatedMB <= 0 || l.HeapMB <= 0 {
			t.Errorf("fresh layer %q allocated %v MB with a %v MB heap; a layer that was built allocates", l.Name, l.AllocatedMB, l.HeapMB)
		}
	}
	if r.RestoredReads == 0 {
		t.Error("the second pass restored nothing from the store, so it measured a second read")
	}
}
