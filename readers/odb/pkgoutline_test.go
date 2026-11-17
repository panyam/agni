package odb

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// odbJobs is every ODB++ job the tests can reach: the fixtures, and the sample jobs Altium, Allegro
// and KiCad wrote when `make samples` has fetched them.
func odbJobs(t *testing.T) map[string]fs.FS {
	t.Helper()
	jobs := map[string]fs.FS{}
	for _, d := range []string{"board", "demo-board", "inch-job", "mixer", "board_stackup"} {
		jobs[d] = os.DirFS(filepath.Join("testdata", d))
	}
	_ = filepath.WalkDir(filepath.Join("..", "..", "tools", "samples", "odb"), func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !(strings.HasSuffix(p, ".tgz") || strings.HasSuffix(p, ".zip")) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		job, err := jobIn(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		jobs[filepath.Base(p)] = job
		return nil
	})
	return jobs
}

// The reader places a package outline only where the package's pins land on the part's own pads
// (agni 1656). Whichever tool wrote the job, nearly every part must pass, or the frame rule is wrong.
// On KiCad's sample jobs, flipping a back part by its side alone must fit fewer parts than the rule,
// since KiCad writes a back part's package already flipped. Those jobs come with `make odb-oracle`,
// and TestABackPartsPackageFlipsUnlessMarkedM holds the rule without them.
func TestAPackageFitsItsPartsPadsInEveryTool(t *testing.T) {
	total, kicadFit, kicadBySide := 0, 0, 0
	for name, fsys := range odbJobs(t) {
		p, err := parseJob(fsys)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		kicadSample := false
		if b, err := fs.ReadFile(fsys, "misc/info"); err == nil && strings.HasSuffix(name, ".odb.tgz") {
			kicadSample = strings.Contains(string(b), "ODB_SOURCE=KiCad")
		}
		fit, n := 0, 0
		for _, c := range p.allComponents() {
			if c.pkg < 0 || c.pkg >= len(p.eda.pkgs) || len(c.pins) == 0 {
				continue
			}
			pkg := &p.eda.pkgs[c.pkg]
			rot := normDeg(-c.rot)
			n++
			ok := packageFitsPads(c, pkg, rot, packageFlips(c))
			if ok {
				fit++
			}
			if kicadSample {
				if ok {
					kicadFit++
				}
				if packageFitsPads(c, pkg, rot, c.back) {
					kicadBySide++
				}
			}
		}
		if n > 0 && fit*100 < n*94 {
			t.Errorf("%s: %d of %d parts fit their package, want at least 94%%", name, fit, n)
		}
		total += n
	}
	if total < 10 {
		t.Errorf("%d parts checked, want at least 10", total)
	}
	if kicadFit > 0 && kicadBySide >= kicadFit {
		t.Errorf("on KiCad's sample jobs %d parts fit by the rule and %d by side alone; want the side alone to fit fewer", kicadFit, kicadBySide)
	}
}

// A back part's package flips when the part is on the back or marked M, not both (agni 1656). Both
// parts sit at (10, 10) mm on the back, and their package's pins are (-1, 0) and (1, 0.5) mm. The
// Altium-style part is marked N, and its pads lie flipped across x. The KiCad-style part is marked M,
// and its package is already flipped, so its pads lie as the package draws them.
func TestABackPartsPackageFlipsUnlessMarkedM(t *testing.T) {
	const mm = 1_000_000
	pkg := &edaPkg{pins: []edaPin{{name: "1", at: pt{-1 * mm, 0}}, {name: "2", at: pt{1 * mm, mm / 2}}}}
	part := func(mirror bool, p1, p2 pt) *component {
		return &component{at: pt{10 * mm, 10 * mm}, back: true, mirror: mirror, pins: []toeprint{{num: 0, at: p1}, {num: 1, at: p2}}}
	}
	altium := part(false, pt{11 * mm, 10 * mm}, pt{9 * mm, 10*mm + mm/2})
	kicad := part(true, pt{9 * mm, 10 * mm}, pt{11 * mm, 10*mm + mm/2})
	for name, c := range map[string]*component{"Altium": altium, "KiCad": kicad} {
		if !packageFitsPads(c, pkg, 0, packageFlips(c)) {
			t.Errorf("%s-style back part: its package misses its pads under the rule", name)
		}
		if packageFitsPads(c, pkg, 0, !packageFlips(c)) {
			t.Errorf("%s-style back part: its package fits the other way too, so the test can't tell", name)
		}
	}
	if packageFlips(altium) == packageFlips(kicad) {
		t.Error("the two styles flip alike, want the M part unflipped")
	}
}

// KiCad's export draws a package outline around the pads, so each of a KiCad part's pads must sit
// inside its outline on the board. That holds the composed outline to the board, not just to the
// package.
func TestAKiCadPartsOutlineCoversItsPads(t *testing.T) {
	checked := 0
	for name, fsys := range odbJobs(t) {
		if b, err := fs.ReadFile(fsys, "misc/info"); err != nil || !strings.Contains(string(b), "ODB_SOURCE=KiCad") {
			continue
		}
		_, g := readJob(t, fsys)
		outline := map[string]*geom.BBox{}
		for _, gr := range g.GetGraphics() {
			if gr.GetRefDes() == "" || !strings.HasSuffix(gr.GetLayer(), ".Fab") {
				continue
			}
			var box geomath.Bounds
			for _, p := range gr.GetShape().GetPoints() {
				box.Add(p)
			}
			outline[gr.GetRefDes()] = box.BBox()
		}
		if n := len(g.GetPlacements()); len(outline)*100 < n*94 {
			t.Errorf("%s: %d of %d parts have a package outline, want at least 94%%", name, len(outline), n)
		}
		for _, pl := range g.GetPlacements() {
			o := outline[pl.GetRefDes()]
			if o == nil {
				continue
			}
			for _, pad := range pl.GetPads() {
				at := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), pad.GetAt())
				checked++
				if at.GetX() < o.GetMin().GetX()-pinSlack || at.GetX() > o.GetMax().GetX()+pinSlack ||
					at.GetY() < o.GetMin().GetY()-pinSlack || at.GetY() > o.GetMax().GetY()+pinSlack {
					t.Errorf("%s %s pad %s at %v lies outside its outline %v", name, pl.GetRefDes(), pad.GetNumber(), at, o)
				}
			}
		}
	}
	if checked < 10 {
		t.Errorf("checked %d pads on KiCad's jobs, want at least 10", checked)
	}
}

func TestEachOutlineRecordGivesARing(t *testing.T) {
	const mm = 1_000_000
	data := `UNITS=MM
PKG P 1 -2 -1 2 1
RC -2 -1 4 2
SQ 5 5 1
CR 10 0 1
CT
OB 0 0 I
OS 3 0
OC 3 2 3 1 N
OS 0 2
OE
OB 1 1 H
OS 2 1
OS 2 1.5
OE
CE
PIN 1 S 0 0 0 E S
RC 0 0 1 1
`
	e, err := parseEDA([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	rings := e.pkgs[0].outline
	if len(rings) != 4 {
		t.Fatalf("%d rings, want RC, SQ, CR and the contour's island; the hole and the pin's RC draw none", len(rings))
	}
	box := func(r []pt) [4]int64 {
		b := [4]int64{r[0].x, r[0].y, r[0].x, r[0].y}
		for _, p := range r {
			b = [4]int64{min(b[0], p.x), min(b[1], p.y), max(b[2], p.x), max(b[3], p.y)}
		}
		return b
	}
	for i, want := range [][4]int64{
		{-2 * mm, -1 * mm, 2 * mm, 1 * mm},
		{4 * mm, 4 * mm, 6 * mm, 6 * mm},
		{9 * mm, -1 * mm, 11 * mm, 1 * mm},
		{0, 0, 4 * mm, 2 * mm}, // the counter-clockwise arc about (3, 1) bulges out to x = 4
	} {
		if got := box(rings[i]); got != want {
			t.Errorf("ring %d box = %v, want %v", i, got, want)
		}
	}
}

func TestCompHeightTakesItsNumberFromTheFile(t *testing.T) {
	for _, tc := range []struct{ name, data string }{
		{"Altium", "UNITS=INCH\n@0 .comp_mount_type\n@1 .comp_height\nCMP 0 1 1 0 N L1 P ;0=1,1=0.1\nCMP 0 2 2 0 N L2 P ;0=1\n"},
		{"Allegro", "UNITS=INCH\n@0 .comp_ignore\n@4 .comp_height\nCMP 0 1 1 0 N L1 P ;0,1,2=1,4=0.1\nCMP 0 2 2 0 N L2 P ;0\n"},
	} {
		comps, _, err := parseComponents([]byte(tc.data), false)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(comps) != 2 || comps[0].height != 2_540_000 || comps[1].height != -1 {
			t.Errorf("%s: heights = %v, want 2.54 mm for L1 and none for L2", tc.name, heights(comps))
		}
	}
}

func heights(cs []component) []int64 {
	var out []int64
	for _, c := range cs {
		out = append(out, c.height)
	}
	return out
}

// A .comp_height reaches the placement, in the components file's units (agni 1656).
func TestAPartTakesItsHeightOntoTheBoard(t *testing.T) {
	job := inchJobCopy(t)
	const comps = "steps/pcb/layers/comp_+_top/components"
	old := string(job[comps].Data)
	withHeight := strings.Replace(old, "UNITS=INCH\n", "UNITS=INCH\n@0 .comp_height\n", 1)
	withHeight = strings.Replace(withHeight, "CMP 0 0.5 0.5 90 N U1 LDO-PART ;0=0", "CMP 0 0.5 0.5 90 N U1 LDO-PART ;0=0.05", 1)
	if withHeight == old || !strings.Contains(withHeight, "0=0.05") {
		t.Fatal("inch-job's U1 line moved; this test gives it a height")
	}
	job[comps] = &fstest.MapFile{Data: []byte(withHeight)}
	_, g := readJob(t, job)
	if u1 := placement(t, g, "U1"); u1.Height == nil || u1.GetHeight() != 1_270_000 {
		t.Errorf("U1 height = %v, want 50 mil", u1.Height)
	}
	if u2 := placement(t, g, "U2"); u2.Height != nil {
		t.Errorf("U2 height = %d, want none, as the file states none", u2.GetHeight())
	}
}

// inchJobCopy is the inch-job fixture in memory, for a test to change one file of.
func inchJobCopy(t *testing.T) fstest.MapFS {
	t.Helper()
	job := fstest.MapFS{}
	root := filepath.Join("testdata", "inch-job")
	err := fs.WalkDir(os.DirFS(root), ".", func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := os.ReadFile(filepath.Join(root, p))
		job[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return job
}

// A package whose pins miss the part's pads draws no outline, since the reader can't tell where the
// outline goes (agni 1656). Moving U1's first toeprint 20 mil off its package pin is enough.
func TestAPackageThatMissesItsPadsDrawsNoOutline(t *testing.T) {
	job := inchJobCopy(t)
	const comps = "steps/pcb/layers/comp_+_top/components"
	old := string(job[comps].Data)
	moved := strings.Replace(old, "TOP 0 0.5 0.55 90 N 1 0 1", "TOP 0 0.52 0.55 90 N 1 0 1", 1)
	if moved == old {
		t.Fatal("inch-job's U1 toeprint moved; this test moves it")
	}
	job[comps] = &fstest.MapFile{Data: []byte(moved)}
	_, g := readJob(t, job)
	for _, gr := range g.GetGraphics() {
		if gr.GetRefDes() == "U1" {
			t.Errorf("U1 draws %s on %s, want no outline", gr.GetShape().GetKind(), gr.GetLayer())
		}
	}
}
