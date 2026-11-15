package odb

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/core/stackup"
)

func describeStack(t *testing.T, fsys fs.FS) ([]string, *int64) {
	t.Helper()
	_, g := readJob(t, fsys)
	var got []string
	for _, l := range g.GetStackup().GetLayers() {
		got = append(got, stackup.Describe(l, g.GetUnitNm()))
	}
	if g.GetStackup() == nil {
		return got, nil
	}
	return got, g.GetStackup().Thickness
}

// mixer is KiCad's own ODB++ export of a two-layer board, which writes .layer_dielectric on every
// layer it has a thickness for and no board thickness.
func TestStackupFromKiCadExport(t *testing.T) {
	got, total := describeStack(t, os.DirFS("testdata/mixer"))
	want := []string{
		"F.SilkS silk thickness unknown",
		"F.Paste paste thickness unknown",
		"F.Mask mask 0.01 mm er=3.3",
		"F.Cu copper 0.035 mm",
		"DIELECTRIC_1 dielectric 1.51 mm fr4 core er=4.5 tand=0.02",
		"B.Cu copper 0.035 mm",
		"B.Mask mask 0.01 mm er=3.3",
		"B.Paste paste thickness unknown",
		"B.SilkS silk thickness unknown",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	if total != nil {
		t.Errorf("board thickness = %d, want unset since the job states none", *total)
	}
}

// fourLayerJob is the board fixture made four layers deep, with a blind drill layer from the top to
// the first inner layer and one via on it. Its attrlists are in inches. The first inner layer gives
// only a copper weight, and the step's attrlist states the board's thickness.
func fourLayerJob(t *testing.T) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	err := fs.WalkDir(os.DirFS("testdata/board"), ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		b, err := os.ReadFile(path.Join("testdata/board", p))
		fsys[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	layer := func(row int, typ, name, extra string) string {
		return "LAYER {\nROW=" + string(rune('0'+row/10)) + string(rune('0'+row%10)) + "\nCONTEXT=BOARD\nTYPE=" + typ + "\nNAME=" + name + "\nPOLARITY=POSITIVE\n" + extra + "}\n"
	}
	fsys["matrix/matrix"] = &fstest.MapFile{Data: []byte("STEP {\nCOL=1\nNAME=PCB\n}\n" +
		layer(1, "COMPONENT", "COMP_+_TOP", "") +
		layer(2, "SIGNAL", "F.CU", "") +
		layer(3, "DIELECTRIC", "DIELECTRIC_1", "DIELECTRIC_TYPE=PREPREG\n") +
		layer(4, "SIGNAL", "IN1.CU", "") +
		layer(5, "DIELECTRIC", "DIELECTRIC_2", "DIELECTRIC_TYPE=CORE\n") +
		layer(6, "POWER_GROUND", "IN2.CU", "") +
		layer(7, "DIELECTRIC", "DIELECTRIC_3", "DIELECTRIC_TYPE=PREPREG\n") +
		layer(8, "SIGNAL", "B.CU", "") +
		layer(9, "DRILL", "DRILL_BLIND", "START_NAME=F.CU\nEND_NAME=IN1.CU\n") +
		"LAYER {\nROW=10\nCONTEXT=MISC\nTYPE=DOCUMENT\nNAME=EDGE.CUTS\nPOLARITY=POSITIVE\n}\n")}
	attrs := map[string]string{
		"f.cu":         "UNITS=INCH\n.layer_dielectric=0.0014\n",
		"dielectric_1": "UNITS=INCH\n.layer_dielectric=0.004\n.material=PR1080\n",
		"in1.cu":       ".copper_weight=0.5\n",
		"dielectric_2": "UNITS=INCH\n.layer_dielectric=0.04\n.material=FR4\n.dielectric_constant=4.5\n",
		"in2.cu":       "UNITS=INCH\n.layer_dielectric=0.0007\n",
		"dielectric_3": "UNITS=INCH\n.layer_dielectric=0.004\n",
	}
	for l, a := range attrs {
		fsys["steps/pcb/layers/"+l+"/attrlist"] = &fstest.MapFile{Data: []byte(a)}
	}
	fsys["steps/pcb/attrlist"] = &fstest.MapFile{Data: []byte("UNITS=INCH\n.board_thickness=0.062\n")}
	fsys["steps/pcb/layers/drill_blind/features"] = &fstest.MapFile{Data: []byte(
		"UNITS=MM\n$0 r200\n@0 .drill\n#\nP 5 5 0 P 0 0 ;0=2\n")}
	return fsys
}

func TestStackupFourLayersInInches(t *testing.T) {
	got, total := describeStack(t, fourLayerJob(t))
	want := []string{
		"F.Cu copper 0.03556 mm",
		"DIELECTRIC_1 dielectric 0.1016 mm PR1080 prepreg",
		// in1.cu gives only a copper weight, half an ounce.
		"In1.Cu copper 0.017399 mm",
		"DIELECTRIC_2 dielectric 1.016 mm FR4 core er=4.5",
		"In2.Cu copper 0.01778 mm",
		"DIELECTRIC_3 dielectric 0.1016 mm prepreg",
		// b.cu's attrlist from the fixture is in the job's millimetres.
		"B.Cu copper 0.035 mm",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	if total == nil || *total != 1_574_800 {
		t.Fatalf("board thickness = %v, want 1574800 nm (0.062 in)", total)
	}
}

func TestDielectricIsNoLayerTableRow(t *testing.T) {
	_, g := readJob(t, fourLayerJob(t))
	var names []string
	for _, l := range g.GetLayers() {
		names = append(names, l.GetName()+":"+l.GetKind())
	}
	for _, n := range names {
		if n == "DIELECTRIC_1:dielectric" || n == "DIELECTRIC_2:dielectric" {
			t.Fatalf("layer table %q lists a dielectric", names)
		}
	}
	if !slices.Contains(names, "In1.Cu:signal") || !slices.Contains(names, "In2.Cu:signal") {
		t.Fatalf("layer table %q is missing the inner copper", names)
	}
}

func TestBlindDrillLayerSpansItsLayers(t *testing.T) {
	_, g := readJob(t, fourLayerJob(t))
	var spans []string
	for _, nc := range g.GetNets() {
		for _, v := range nc.GetVias() {
			spans = append(spans, v.GetLayerFrom()+".."+v.GetLayerTo())
		}
	}
	if !slices.Contains(spans, "F.Cu..In1.Cu") {
		t.Fatalf("via spans %q, want one F.Cu..In1.Cu from the blind drill layer", spans)
	}
}
