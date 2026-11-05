package geda

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/panyam/agni/internal/geomath"
)

// orient-matrix.sch places one three-pin part, which no turn or flip maps onto itself, at 0, 90,
// 180 and 270 degrees, unmirrored (U1-U4) and mirrored (U5-U8). Each place a pin could land, under
// either order of mirror and rotate, has a net stub named `U<k>_<x>_<y>` for the offset from the
// part's origin. So the net each pin joins says where it landed. orient-matrix.lepton.net is
// Lepton's own netlist of the file (`lepton-netlist -g geda` in the agni-native image), and
// orient-matrix.gen.py writes the schematic and its symbol (agni issue 1344).

func testdataOpener(name string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", name)) }

// leptonNets reads a gEDA-backend netlist into `REF.PIN=NET` rows.
func leptonNets(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, pins, ok := strings.Cut(sc.Text(), " : ")
		if !ok {
			continue
		}
		for _, p := range strings.Split(pins, ",") {
			if ref, pin, ok := strings.Cut(strings.TrimSpace(p), " "); ok {
				out = append(out, ref+"."+pin+"="+strings.TrimSpace(name))
			}
		}
	}
	sort.Strings(out)
	return out
}

// TestGEDAOrientationMatchesLepton holds each pin's net to Lepton's netlist of the same file. gEDA
// mirrors a part across its Y axis and then turns it, so a mirrored part at 90 or 270 is where the
// two orders disagree.
func TestGEDAOrientationMatchesLepton(t *testing.T) {
	want := leptonNets(t, "testdata/orient-matrix.lepton.net")
	if len(want) != 24 {
		t.Fatalf("Lepton's netlist has %d pin rows, want 24 (8 parts, 3 pins each)", len(want))
	}
	d, err := ReadWithSymbols(bytes.NewReader(readFixture(t, "orient-matrix.sch")), "orient-matrix.sch", testdataOpener)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range d.GetNets() {
		for _, c := range n.GetConnections() {
			got = append(got, c.GetComponentRef()+"."+c.GetPinRef()+"="+n.GetName())
		}
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("pin nets differ from Lepton's\n got: %v\nwant: %v", got, want)
	}
}

// TestGEDADrawingPutsPinsWhereLeptonConnectsThem holds the drawing to the same netlist. Each pin's
// drawn connect point, through the placement's transform, is the end of the stub its net names.
func TestGEDADrawingPutsPinsWhereLeptonConnectsThem(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "orient-matrix.sch")), "orient-matrix.sch", testdataOpener)
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string][]string{}
	for _, row := range leptonNets(t, "testdata/orient-matrix.lepton.net") {
		pin, net, _ := strings.Cut(row, "=")
		defs[pin] = append(defs[pin], net)
	}
	symbols := map[string]map[string]*[2]int64{}
	for _, s := range g.GetSymbols() {
		m := map[string]*[2]int64{}
		for _, p := range s.GetPins() {
			m[p.GetPortRef()] = &[2]int64{p.GetLoc().GetX(), p.GetLoc().GetY()}
		}
		symbols[s.GetCellRef()] = m
	}
	checked := 0
	for _, sheet := range g.GetSheets() {
		for _, pl := range sheet.GetPlacements() {
			for num, loc := range symbols[pl.GetCellRef()] {
				at := geomath.ApplyTransform(pl.GetTransform(), gedaPt(float64(loc[0]), float64(loc[1])))
				o := pl.GetTransform().GetOrigin()
				want := pl.GetRefDes() + "_" + offset(at.GetX()-o.GetX()) + "_" + offset(at.GetY()-o.GetY())
				nets := defs[pl.GetRefDes()+"."+num]
				if len(nets) != 1 || nets[0] != want {
					t.Errorf("%s.%s draws at the end of stub %s, Lepton connects it to %v", pl.GetRefDes(), num, want, nets)
				}
				checked++
			}
		}
	}
	if checked != 24 {
		t.Errorf("checked %d drawn pins, want 24", checked)
	}
}

func offset(v int64) string { return strings.Replace(strconv.FormatInt(v, 10), "-", "m", 1) }

// TestGEDAFieldKeepsItsSheetAngle draws each ref-des at the angle its T line gives on the sheet,
// as Lepton does. The fixture writes each part's ref-des at the part's own angle, as gschem does
// when it turns a part. The renderer adds the placement's turn to the field's, so the two must sum
// to the sheet angle rather than to twice it.
func TestGEDAFieldKeepsItsSheetAngle(t *testing.T) {
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, "orient-matrix.sch")), "orient-matrix.sch", testdataOpener)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, sheet := range g.GetSheets() {
		for _, pl := range sheet.GetPlacements() {
			for _, f := range pl.GetFields() {
				if f.GetName() != "Reference" && f.GetName() != "refdes" {
					continue
				}
				seen++
				want := pl.GetTransform().GetRotationDeg()
				if got := (pl.GetTransform().GetRotationDeg() + f.GetRotationDeg()) % 360; got != want {
					t.Errorf("%s ref-des draws at %d degrees, want the T line's %d", pl.GetRefDes(), got, want)
				}
			}
		}
	}
	if seen != 8 {
		t.Errorf("checked %d ref-des fields, want 8", seen)
	}
}
