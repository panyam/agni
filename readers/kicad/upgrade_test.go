package kicad

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestKiCad6SheetPropertiesReadAsKiCad7 reads the hierarchy fixture respelled the way KiCad 6 wrote
// it, ("Sheet name" and "Sheet file"), and requires the same components and nets as the fixture
// itself. Before upgrade, the root named no sub-sheets and the read held only the root's parts, which
// is how HackRF One read as an empty design (agni issue 1233).
func TestKiCad6SheetPropertiesReadAsKiCad7(t *testing.T) {
	root := readFixture(t, "hier_root.kicad_sch")
	if !bytes.Contains(root, []byte(`"Sheetfile"`)) {
		t.Fatal(`fixture has no "Sheetfile" property to respell, so this test would compare a file with itself`)
	}
	v6 := bytes.ReplaceAll(bytes.ReplaceAll(root, []byte(`"Sheetfile"`), []byte(`"Sheet file"`)), []byte(`"Sheetname"`), []byte(`"Sheet name"`))
	want, _, err := ReadSchematicHierarchyNets("hier_root.kicad_sch", root, hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	got, complete, err := ReadSchematicHierarchyNets("hier_root.kicad_sch", v6, hierOpen(t))
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Error("KiCad 6 spelling: walk reports sub-sheets it could not open")
	}
	if g, w := refsOf(got.Components), refsOf(want.Components); g != w {
		t.Errorf("KiCad 6 spelling read components %q, KiCad 7 spelling %q", g, w)
	}
	if len(got.Nets) != len(want.Nets) || len(want.Nets) == 0 {
		t.Errorf("KiCad 6 spelling read %d nets, KiCad 7 spelling %d", len(got.Nets), len(want.Nets))
	}
}

// kicad7Board is two footprints in the form KiCad 6 and 7 wrote, with each designator in an fp_text.
// R2's reference is hidden with the bare hide atom those versions used.
const kicad7Board = `(kicad_pcb (version 20221018) (generator pcbnew)
  (net 0 "")
  (net 1 "VCC")
  (net 2 "SIG")
  (footprint "Resistor_SMD:R_0402" (layer "F.Cu") (at 10 10)
    (fp_text reference "R1" (at 0 -1.5) (layer "F.SilkS") (effects (font (size 1 1))))
    (fp_text value "10k" (at 0 1.5) (layer "F.Fab") (effects (font (size 1 1))))
    (fp_text user "${REFERENCE}" (at 0 0) (layer "F.Fab") (effects (font (size 1 1))))
    (pad "1" smd rect (at -0.5 0) (size 0.5 0.5) (layers "F.Cu") (net 1 "VCC"))
    (pad "2" smd rect (at 0.5 0) (size 0.5 0.5) (layers "F.Cu") (net 2 "SIG"))
  )
  (footprint "Resistor_SMD:R_0402" (layer "F.Cu") (at 20 10)
    (fp_text reference "R2" (at 0 -1.5) (layer "F.SilkS") hide (effects (font (size 1 1))))
    (fp_text value "1k" (at 0 1.5) (layer "F.Fab") (effects (font (size 1 1))))
    (pad "1" smd rect (at -0.5 0) (size 0.5 0.5) (layers "F.Cu") (net 2 "SIG"))
    (pad "2" smd rect (at 0.5 0) (size 0.5 0.5) (layers "F.Cu") (net 1 "VCC"))
  )
)`

// TestKiCad7FootprintTextReadsAsReference reads a board whose designators are fp_text, and requires
// both footprints as components on their nets, and the drawn texts to honour the bare hide. Before
// upgrade the board had no components, and a .kicad_pro read took its nets from it (agni issue 1233).
func TestKiCad7FootprintTextReadsAsReference(t *testing.T) {
	d, err := Read(strings.NewReader(kicad7Board), "b.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if got := refsOf(d.Components); got != "R1 R2" {
		t.Errorf("components = %q, want R1 R2", got)
	}
	if len(d.Nets) != 2 {
		t.Errorf("nets = %d, want VCC and SIG", len(d.Nets))
	}
	for _, n := range d.Nets {
		if len(n.Connections) != 2 {
			t.Errorf("net %s has %d connections, want one pad of each resistor", n.Name, len(n.Connections))
		}
	}

	g, err := ReadBoardGeometry(strings.NewReader(kicad7Board), "b.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, tx := range g.Texts {
		texts = append(texts, tx.RefDes+":"+tx.Kind+"="+tx.Text)
	}
	sort.Strings(texts)
	// R2's hidden reference is not drawn, and the fp_text user stays out of this tier.
	if got, want := strings.Join(texts, " "), "R1:reference=R1 R1:value=10k R2:value=1k"; got != want {
		t.Errorf("board texts = %q, want %q", got, want)
	}
}

func refsOf(comps []*ir.Component) string {
	var refs []string
	for _, c := range comps {
		refs = append(refs, c.RefDes)
	}
	sort.Strings(refs)
	return strings.Join(refs, " ")
}
