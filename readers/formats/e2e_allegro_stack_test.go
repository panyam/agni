package formats

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestAllegro166StackMatchesItsIPC2581Twin holds parasdr's 16.6 board's stack to the IPC-2581 file
// Allegro exported from it (agni 1601): the same rows in the same order, each with the same kind,
// copper layer, thickness and material, summing to the total the export states.
func TestAllegro166StackMatchesItsIPC2581Twin(t *testing.T) {
	brd, ipc := parasdr+"parasdr_layout.brd", parasdr+"parasdr_ipc2581.xml"
	if _, err := os.Stat(brd); err != nil {
		t.Fatalf("parasdr's board is missing; run `make samples-oracle`: %v", err)
	}
	l := &Loader{}
	bg, err := l.BoardGeometry(brd)
	if err != nil {
		t.Fatal(err)
	}
	ig, err := l.BoardGeometry(ipc)
	if err != nil {
		t.Fatal(err)
	}
	ours, theirs := bg.GetStackup().GetLayers(), ig.GetStackup().GetLayers()
	if len(ours) < 11 || len(ours) != len(theirs) {
		t.Fatalf("%d rows, the export has %d", len(ours), len(theirs))
	}
	sum := int64(0)
	for i, o := range ours {
		w := theirs[i]
		sum += o.GetThickness()
		if o.GetKind() != w.GetKind() || o.GetLayer() != w.GetLayer() || o.GetMaterial() != w.GetMaterial() || abs(o.GetThickness()-w.GetThickness()) > 100 {
			t.Errorf("row %d = %s %s %d nm %s, the export says %s %s %d nm %s", i,
				o.GetKind(), o.GetLayer(), o.GetThickness(), o.GetMaterial(), w.GetKind(), w.GetLayer(), w.GetThickness(), w.GetMaterial())
		}
	}
	if st := ig.GetStackup(); st.Thickness == nil || abs(sum-st.GetThickness()) > 100 {
		t.Errorf("rows sum to %d nm, the export states %v", sum, st.Thickness)
	}
}

// TestAllegro172StackMatchesItsExtract holds the BeagleBone AI's 17.2 stack to the extract Allegro
// wrote from the board (agni 1601). The extract's J record states the board thickness and the copper
// layer count, and the rows must sum to that and name that many copper layers.
func TestAllegro172StackMatchesItsExtract(t *testing.T) {
	_, g, _ := readAllegroSample(t)
	thickness, layers := extractStack(t, filepath.Join(allegroBoards, "beaglebone-ai", "cds2f_BeagleBone-AI.txt"))
	sum, copper := int64(0), 0
	for _, l := range g.GetStackup().GetLayers() {
		sum += l.GetThickness()
		if l.GetKind() == "copper" {
			copper++
		}
	}
	if copper != layers || abs(sum-thickness) > 100 {
		t.Errorf("%d copper layers summing to %d nm with their dielectrics, the extract states %d layers and %d nm", copper, sum, layers, thickness)
	}
}

// TestEveryAllegroBoardStacksItsCopperApart checks the stack of each Allegro board, Clay R6 among
// them, whose stack no other file states (agni 1601). Every copper layer the board names appears
// once, in order, with a dielectric between each two, and every row states a thickness above zero
// and under 5 mm.
func TestEveryAllegroBoardStacksItsCopperApart(t *testing.T) {
	boards := []string{parasdr + "parasdr_layout.brd", clayR6 + "CLAYR6.brd", filepath.Join(allegroBoards, "beaglebone-ai", "BeagleBone-AI.brd")}
	for _, brd := range boards {
		if _, err := os.Stat(brd); err != nil {
			t.Fatalf("%s is missing; run `make samples-oracle`: %v", brd, err)
		}
		g, err := (&Loader{}).BoardGeometry(brd)
		if err != nil {
			t.Fatal(err)
		}
		rows := g.GetStackup().GetLayers()
		var copper []string
		for i, r := range rows {
			if r.Thickness == nil || r.GetThickness() <= 0 || r.GetThickness() > 5_000_000 {
				t.Errorf("%s row %d (%s) is %v nm thick, want above zero and under 5 mm", brd, i, r.GetName(), r.Thickness)
			}
			if want := []string{"copper", "dielectric"}[i%2]; r.GetKind() != want {
				t.Errorf("%s row %d is %s, want %s", brd, i, r.GetKind(), want)
			}
			if r.GetKind() == "copper" {
				copper = append(copper, r.GetLayer())
			}
		}
		var named []string
		for _, l := range g.GetLayers() {
			named = append(named, l.GetName())
		}
		if len(copper) < 4 || strings.Join(copper, " ") != strings.Join(named, " ") {
			t.Errorf("%s: stack copper %v, the board names %v", brd, copper, named)
		}
	}
}

// extractStack reads an extract's J record: the board thickness as "<value> mil" in field 10, and
// the copper layer count in field 11.
func extractStack(t *testing.T, path string) (int64, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), "!")
		if fields[0] != "J" || len(fields) < 13 {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(fields[10]), " mil"), 64)
		if err != nil {
			t.Fatalf("J record thickness %q: %v", fields[10], err)
		}
		n, err := strconv.Atoi(fields[11])
		if err != nil {
			t.Fatalf("J record layer count %q: %v", fields[11], err)
		}
		return int64(v*milNm + 0.5), n
	}
	t.Fatalf("%s has no J record", path)
	return 0, 0
}
