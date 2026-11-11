package render

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A board note of three lines draws as three lines, the middle one on the note's anchor and the
// others 1.61 heights above and below it, as kicad-cli strokes Glasgow revC3's caution note. Until
// this, both renderers folded the newlines into spaces and drew one long line.
func TestABoardNoteDrawsEachLine(t *testing.T) {
	note := &geom.BoardText{Text: "Caution:\nCheck washer\nconflicts", At: &geom.Point{X: 10_000_000, Y: 20_000_000},
		Height: 1_000_000, Layer: "Dwgs.User", Kind: "gr", Justify: "left"}
	b := &geom.BoardGeometry{Texts: []*geom.BoardText{note}}

	lines := boardLabels(b)
	if len(lines) != 3 {
		t.Fatalf("boardLabels gave %d texts, want one per line (3)", len(lines))
	}
	for n, wantY := range []int64{21_610_000, 20_000_000, 18_390_000} {
		l := lines[n]
		if strings.Contains(l.GetText(), "\n") || l.GetAt().GetX() != 10_000_000 || l.GetAt().GetY() != wantY || l.GetJustify() != "left" {
			t.Errorf("line %d = %q at (%d,%d) justify %q, want one line at (10000000,%d) justify left",
				n, l.GetText(), l.GetAt().GetX(), l.GetAt().GetY(), l.GetJustify(), wantY)
		}
	}

	svg := BoardSVG(b)
	for _, line := range []string{">Caution:</text>", ">Check washer</text>", ">conflicts</text>"} {
		if !strings.Contains(svg, line) {
			t.Errorf("BoardSVG has no %s", line)
		}
	}
	var packed []string
	for _, l := range PackedLabels(PackBoard(b)) {
		packed = append(packed, l.GetText())
	}
	if strings.Join(packed, "|") != "Caution:|Check washer|conflicts" {
		t.Errorf("packed labels = %q, want the three lines", packed)
	}
}

// A turned note stacks its lines along its turn. At 90 degrees the next line sits to the right.
func TestATurnedBoardNoteStacksAlongItsTurn(t *testing.T) {
	note := &geom.BoardText{Text: "A\nB", At: &geom.Point{X: 0, Y: 0}, Height: 1_000_000, RotationDeg: 90}
	lines := boardTextLines(note)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if a, b := lines[0].GetAt(), lines[1].GetAt(); a.GetX() != -805_000 || b.GetX() != 805_000 || a.GetY() != 0 || b.GetY() != 0 {
		t.Errorf("lines at (%d,%d) and (%d,%d), want (-805000,0) and (805000,0)", a.GetX(), a.GetY(), b.GetX(), b.GetY())
	}
}
