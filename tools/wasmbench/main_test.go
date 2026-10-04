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
