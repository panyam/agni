package formats

import (
	"strings"
	"testing"
	"testing/fstest"
)

const boxStep = `ISO-10303-21;
HEADER;
ENDSEC;
DATA;
#1=CARTESIAN_POINT('',(0.,0.,0.));
#2=CARTESIAN_POINT('',(1.,1.,%s));
#3=VERTEX_POINT('',#1);
#4=VERTEX_POINT('',#2);
ENDSEC;
END-ISO-10303-21;
`

// stepOfHeight is a 1 x 1 mm block h mm tall, as a STEP file.
func stepOfHeight(h string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(strings.Replace(boxStep, "%s", h, 1))}
}

func modelFootprint(ref, model string) string {
	return `  (footprint "x:y" (layer "F.Cu") (at 0 0)
    (property "Reference" "` + ref + `")
    (pad "1" smd rect (at 0 0) (size 1 1) (layers "F.Cu"))
    (model "` + model + `")
  )
`
}

// TestLoaderResolvesKiCadModels reads a board through an in-memory FS, as the browser engine does
// with a mounted library (agni 1652). A board model resolves beside the board. A library model
// resolves in the first --model-path that holds it. A .wrl name falls back to the .step beside it.
func TestLoaderResolvesKiCadModels(t *testing.T) {
	board := `(kicad_pcb (version 20221018) (generator pcbnew)
  (layers (0 "F.Cu" signal))
` + modelFootprint("A", "${KIPRJMOD}/3d/a.step") +
		modelFootprint("B", "${KICAD9_3DMODEL_DIR}/L.3dshapes/b.step") +
		modelFootprint("C", "${KICAD6_3DMODEL_DIR}/L.3dshapes/c.wrl") +
		modelFootprint("D", "${KICAD6_3DMODEL_DIR}/L.3dshapes/missing.step") +
		modelFootprint("E", "${MY_MODELS}/e.step") + ")\n"
	fsys := fstest.MapFS{
		"proj/board.kicad_pcb":   {Data: []byte(board)},
		"proj/3d/a.step":         stepOfHeight("1."),
		"lib1/L.3dshapes/b.step": stepOfHeight("2."),
		"lib2/L.3dshapes/b.step": stepOfHeight("7."),
		"lib2/L.3dshapes/c.step": stepOfHeight("3."),
	}
	g, err := (&Loader{FS: fsys, ModelPaths: []string{"lib1", "lib2"}}).BoardGeometry("proj/board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"A": 1_000_000, "B": 2_000_000, "C": 3_000_000}
	got := map[string]int64{}
	for _, p := range g.GetPlacements() {
		if p.Height != nil {
			got[p.GetRefDes()] = p.GetHeight() * g.GetUnitNm()
		}
	}
	if len(got) != len(want) {
		t.Errorf("heights %v, want %v", got, want)
	}
	for ref, w := range want {
		if got[ref] != w {
			t.Errorf("%s: height %d nm, want %d", ref, got[ref], w)
		}
	}
	reasons := map[string]string{}
	for _, u := range g.GetUnreadModels() {
		reasons[u.GetPath()] = u.GetReason()
	}
	if len(reasons) != 2 ||
		reasons["${KICAD6_3DMODEL_DIR}/L.3dshapes/missing.step"] != "not found in 2 model path(s)" ||
		reasons["${MY_MODELS}/e.step"] != "names a path variable agni does not know" {
		t.Errorf("unread models %v", reasons)
	}
}

func TestLoaderWithNoModelPathNamesTheFlag(t *testing.T) {
	board := `(kicad_pcb (version 20221018) (generator pcbnew)
  (layers (0 "F.Cu" signal))
` + modelFootprint("B", "${KICAD9_3DMODEL_DIR}/L.3dshapes/b.step") + ")\n"
	g, err := (&Loader{FS: fstest.MapFS{"board.kicad_pcb": {Data: []byte(board)}}}).BoardGeometry("board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if u := g.GetUnreadModels(); len(u) != 1 || u[0].GetReason() != "no model library given; pass --model-path" {
		t.Errorf("unread models %v", u)
	}
}
