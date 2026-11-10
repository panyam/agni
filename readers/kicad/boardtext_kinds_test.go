package kicad

import (
	"sort"
	"strings"
	"testing"
)

// logoBoard is a part, a logo footprint whose reference is the REF** placeholder, and a free text
// in a TrueType face, in the form KiCad 7 wrote.
const logoBoard = `(kicad_pcb (version 20221018) (generator pcbnew)
  (net 0 "")
  (footprint "Logo:OSHW" (layer "F.Cu") (at 30 30)
    (fp_text reference "REF**" (at 0 -3) (layer "F.SilkS") hide (effects (font (size 1 1) (thickness 0.15))))
    (fp_text value "OSHW" (at 0 3) (layer "F.Fab") hide (effects (font (size 1 1) (thickness 0.15))))
    (fp_text user "open hardware" (at 0 2) (layer "F.SilkS") (effects (font (size 1 1) (thickness 0.2))))
    (fp_poly (pts (xy -1 -1) (xy 1 -1) (xy 0 1)) (layer "F.SilkS") (width 0.1) (fill solid))
  )
  (footprint "Resistor_SMD:R_0402" (layer "F.Cu") (at 10 10)
    (fp_text reference "R1" (at 0 -1.5) (layer "F.SilkS") (effects (font (size 1 1) (thickness 0.15))))
    (fp_text value "10k" (at 0 1.5) (layer "F.Fab") (effects (font (size 1 1) (thickness 0.15))))
    (fp_text user "${REFERENCE}" (at 0 0) (layer "F.Fab") (effects (font (size 0.5 0.5) (thickness 0.08))))
    (fp_text user "hidden note" (at 0 0) (layer "F.Fab") hide (effects (font (size 0.5 0.5))))
    (fp_text user "comment" (at 0 0) (layer "Cmts.User") (effects (font (size 0.5 0.5))))
    (fp_line (start -1 -0.5) (end 1 -0.5) (layer "F.SilkS") (width 0.12))
    (pad "1" smd rect (at -0.5 0) (size 0.5 0.5) (layers "F.Cu"))
  )
  (gr_text "Title" (at 50 50) (layer "F.SilkS") (effects (font (face "Arial") (size 2 2))))
)`

// A logo footprint is no part, but KiCad draws its artwork and its user text, so the board does
// too, owned by no part. A part's user text draws with ${REFERENCE} filled in, a hidden one does
// not draw, and one on a layer the board view does not draw stays out. Stroke-font text names its
// font and pen, and text in a TrueType face names neither.
func TestBoardReadsUserTextAndLogoArtwork(t *testing.T) {
	g, err := ReadBoardGeometry(strings.NewReader(logoBoard), "b.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, tx := range g.Texts {
		texts = append(texts, tx.RefDes+":"+tx.Kind+"="+tx.Text+"/"+tx.Font)
	}
	sort.Strings(texts)
	want := ":gr=Title/ :user=open hardware/kicad_stroke R1:reference=R1/kicad_stroke R1:user=R1/kicad_stroke R1:value=10k/kicad_stroke"
	if got := strings.Join(texts, " "); got != want {
		t.Errorf("texts = %q, want %q", got, want)
	}
	for _, tx := range g.Texts {
		if tx.Text == "open hardware" && tx.Thickness != 200_000 {
			t.Errorf("open hardware pen = %d nm, want 200000", tx.Thickness)
		}
	}
	var logo, part int
	for _, gr := range g.Graphics {
		switch gr.RefDes {
		case "":
			logo++
		case "R1":
			part++
		}
	}
	if logo != 1 || part != 1 {
		t.Errorf("graphics: %d owned by no part and %d by R1, want the logo's polygon and R1's line", logo, part)
	}
	if len(g.Placements) != 1 || g.Placements[0].RefDes != "R1" {
		t.Errorf("placements = %v, want R1 alone, since a logo is no part", g.Placements)
	}
}
