package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/formats"
)

// The note names what back-annotation relabelled, the largest lists capped, and says nothing for a
// schematic it relabelled none of.
func TestBackAnnotatedNote(t *testing.T) {
	g := &geom.SchematicGeometry{BackAnnotated: []*geom.BackAnnotatedPlacement{
		{RefDes: "R1001", DrawnRefDes: "R4", SheetId: "P1"},
		{RefDes: "U101", SheetId: "P2"},
	}}
	want := "note: x.eds back-annotates 2 placement(s) under a designator its page does not draw, and they are read by the back-annotated one (R4 is R1001, U101 has no designator drawn).\n"
	if got := backAnnotatedNote("x.eds", g); got != want {
		t.Errorf("note:\n got %q\nwant %q", got, want)
	}
	var many []*geom.BackAnnotatedPlacement
	for i := range 7 {
		many = append(many, &geom.BackAnnotatedPlacement{RefDes: fmt.Sprintf("R%d", 100+i), DrawnRefDes: fmt.Sprintf("R%d", i)})
	}
	if got := backAnnotatedNote("x.eds", &geom.SchematicGeometry{BackAnnotated: many}); !strings.Contains(got, "7 placement(s)") || !strings.Contains(got, "R4 is R104, 2 more)") {
		t.Errorf("capped note = %q", got)
	}
	if got := backAnnotatedNote("x.eds", &geom.SchematicGeometry{}); got != "" {
		t.Errorf("note with nothing relabelled = %q, want none", got)
	}
}

// The service-backed commands read geometry through localLoader, so the note reaches check, query,
// review and trace, once per file however often one command asks.
func TestLocalLoaderNotesBackAnnotatedPlacements(t *testing.T) {
	freshWorkspace(t)
	var notes bytes.Buffer
	l := &localLoader{loader: &formats.Loader{}, notes: &notes}
	u := uriOf(t, filepath.Join(backannotateDir, "demo_netlist.edn"))
	for range 2 {
		if _, err := l.Geometry(context.Background(), u, "faithful", false); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(notes.String(), "note: demo_schematic.eds back-annotates 2 placement(s)"); got != 1 {
		t.Errorf("back-annotation note written %d times, want once:\n%s", got, notes.String())
	}
}
