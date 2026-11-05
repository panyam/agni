package odb

import (
	"fmt"
	"os"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/internal/geomath"
)

const odxFixture = "testdata/odx/two-sided.xml"

func readODX(t *testing.T, path string) (*ir.Design, *geom.BoardGeometry) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := ReadODX(f, path)
	if err != nil {
		t.Fatalf("ReadODX: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	g, err := ReadODXBoardGeometry(f, path)
	if err != nil {
		t.Fatalf("ReadODXBoardGeometry: %v", err)
	}
	return d, g
}

func TestReadsAnODXNetlist(t *testing.T) {
	d, _ := readODX(t, odxFixture)
	if d.GetName() != "two-sided" || d.GetSourceFormat() != "odb++" || d.GetAttributes()["odb_form"] != "odx" {
		t.Errorf("design = %q from %q (form %q), want two-sided from odb++ (odx)", d.GetName(), d.GetSourceFormat(), d.GetAttributes()["odb_form"])
	}
	if got := d.GetAttributes()["odb_source"]; got != "hand-written" {
		t.Errorf("odb_source = %q, want the header's SOURCE", got)
	}
	// R2.1 is on $NONE$, so it is unconnected rather than on a net of its own. J1's pin is named by
	// its NUMBER, as CAD_LNP and the pad's PIN= name it, and not by the placeholder NAME "un_1".
	got := netPins(d)
	want := map[string]string{"N1": "R1.1 R2.2", "N2": "R1.2 J1.1"}
	if len(got) != len(want) {
		t.Errorf("nets = %v, want %v", got, want)
	}
	for n, pins := range want {
		if got[n] != pins {
			t.Errorf("net %s = %q, want %q", n, got[n], pins)
		}
	}
	parts := map[string]*ir.Component{}
	for _, c := range d.GetComponents() {
		parts[c.GetRefDes()] = c
	}
	if len(parts) != 3 {
		t.Fatalf("components = %d, want 3", len(parts))
	}
	r1 := parts["R1"].GetAttributes()
	// The file is ISO-8859-1, and its 0xB5 byte is the micro sign.
	if r1["part"] != "10µR" || r1["Value"] != "10R" || r1["side"] != "top" || parts["R1"].GetFootprintRef() != "R0402" {
		t.Errorf("R1 = %v on %q, want part 10µR, Value 10R, the top side and R0402", r1, parts["R1"].GetFootprintRef())
	}
	if side := parts["R2"].GetAttributes()["side"]; side != "bottom" {
		t.Errorf("R2 side = %q, want bottom", side)
	}
	var layers []string
	for _, l := range d.GetLayers() {
		layers = append(layers, l.GetName())
	}
	if strings.Join(layers, " ") != "top bottom drill" {
		t.Errorf("layers = %v, want the board layers without the component layers", layers)
	}
}

func TestReadsAnODXBoard(t *testing.T) {
	_, g := readODX(t, odxFixture)
	if g.GetOutline() == nil || len(g.GetOutline().GetPaths()) != 1 || len(g.GetOutline().GetPaths()[0].GetPoints()) != 5 {
		t.Errorf("outline = %v, want the profile's one closed rectangle", g.GetOutline())
	}

	// Each pad composes back onto the point the file draws it at, on its own side and net.
	type padWant struct {
		x, y  int64
		net   string
		layer string
		drill int64
	}
	want := map[string]padWant{
		"R1.1": {10e6, 10.5e6, "N1", "F.Cu", 0},
		"R1.2": {10e6, 9.5e6, "N2", "F.Cu", 0},
		"R2.1": {20.5e6, 10e6, "", "B.Cu", 0},
		"R2.2": {19.5e6, 10e6, "N1", "B.Cu", 0},
		"J1.1": {30e6, 10e6, "N2", "*.Cu", 1e6},
	}
	seen := 0
	for _, pl := range g.GetPlacements() {
		if (pl.GetRefDes() == "R2") != pl.GetMirror() {
			t.Errorf("%s mirror = %v, want a bottom-side part mirrored", pl.GetRefDes(), pl.GetMirror())
		}
		for _, p := range pl.GetPads() {
			key := pl.GetRefDes() + "." + p.GetNumber()
			w, ok := want[key]
			if !ok {
				t.Errorf("unexpected pad %s", key)
				continue
			}
			seen++
			at := geomath.ComposePlacement(pl.GetAt(), pl.GetRotationDeg(), pl.GetMirror(), p.GetAt())
			if at.GetX() != w.x || at.GetY() != w.y {
				t.Errorf("%s at (%d,%d), want (%d,%d)", key, at.GetX(), at.GetY(), w.x, w.y)
			}
			if p.GetNet() != w.net || strings.Join(p.GetLayers(), ",") != w.layer || p.GetDrill() != w.drill {
				t.Errorf("%s = net %q on %v drill %d, want net %q on %s drill %d", key, p.GetNet(), p.GetLayers(), p.GetDrill(), w.net, w.layer, w.drill)
			}
		}
	}
	if seen != len(want) {
		t.Errorf("read %d pads, want %d", seen, len(want))
	}

	// R1 turns 90 degrees clockwise, and ODX writes its pad's symbol already turned to the board.
	// So the pad's angle in R1's own frame undoes R1's turn, and its 0.6 x 0.5 mm box stays put.
	r1 := placement(t, g, "R1")
	if p := r1.GetPads()[0]; p.GetSize().GetX() != 600000 || p.GetSize().GetY() != 500000 || normDeg(p.GetRotationDeg()+r1.GetRotationDeg()) != 0 {
		t.Errorf("R1.1 = %v at %v deg in a part at %v deg, want 600000x500000 at 0 deg on the board", p.GetSize(), p.GetRotationDeg(), r1.GetRotationDeg())
	}

	nets := map[string]*geom.NetCopper{}
	for _, nc := range g.GetNets() {
		nets[nc.GetNet()] = nc
	}
	n1 := nets["N1"]
	if n1 == nil || len(n1.GetVias()) != 1 {
		t.Fatalf("N1 copper = %v, want one via", n1)
	}
	// The lands on both sides and the drill hit share a position, so they are one via.
	if v := n1.GetVias()[0]; v.GetAt().GetX() != 15e6 || v.GetAt().GetY() != 10.5e6 || v.GetDrill() != 300000 || v.GetSize() != 600000 {
		t.Errorf("N1 via = %v, want 0.6mm land and 0.3mm drill at (15, 10.5)", v)
	}
	var segs []string
	for _, s := range n1.GetSegments() {
		segs = append(segs, fmt.Sprintf("%s/%d", s.GetLayer(), s.GetWidth()))
	}
	if strings.Join(segs, " ") != "F.Cu/200000 B.Cu/250000" {
		t.Errorf("N1 segments = %v, want one per side, each as wide as the CAD_SET symbol before it", segs)
	}
	if n2 := nets["N2"]; n2 == nil || len(n2.GetSegments()) != 1 || n2.GetSegments()[0].GetMid().GetX() != 20e6 ||
		n2.GetSegments()[0].GetMid().GetY() != 19.5e6 {
		t.Errorf("N2 copper = %v, want one arc from (10, 9.5) to (30, 9.5) through (20, 19.5)", n2)
	}
	// The surface takes N2 from the CAD_SET before it. It is copper, so it reads as a copper shape
	// on N2 with its hole kept (agni 1357).
	if n2 := nets["N2"]; len(g.GetZones()) != 0 || n2 == nil || len(n2.GetShapes()) != 1 {
		t.Fatalf("zones = %v and N2 = %v, want the one N2 surface as a copper shape", g.GetZones(), n2)
	}
	if s := nets["N2"].GetShapes()[0]; s.GetLayer() != "B.Cu" || len(s.GetOutline().GetPoints()) != 5 || len(s.GetHoles()) != 1 {
		t.Errorf("N2 shape = %v, want a 5-point outline on B.Cu with its one hole", s)
	}
}

func TestODXRefusesWhatItCannotRead(t *testing.T) {
	cases := map[string]string{
		`<?xml version="1.0"?><IPC-2581/>`: `root element "IPC-2581"`,
		`<ODX><ODX_CAD><CAD_DATA><CAD_DATA_HEADER UNITS = "ENGLISH"/></CAD_DATA></ODX_CAD></ODX>`: "only METRIC",
		`<ODX><ODX_CAD><CAD_DATA><CAD_DATA_HEADER UNITS = "METRIC"/></CAD_DATA></ODX_CAD></ODX>`:  "no CAD_STEP",
		`<?xml version = "1.0" encoding = "EBCDIC"?><ODX/>`:                                       "EBCDIC",
	}
	for doc, wantErr := range cases {
		_, err := ReadODX(strings.NewReader(doc), "x.xml")
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("ReadODX(%.40q) = %v, want an error naming %q", doc, err, wantErr)
		}
	}
}

func TestXMLRoot(t *testing.T) {
	cases := map[string]string{
		`<?xml version = "1.0" encoding = "ISO-8859-1"?>` + "\n<ODX>":            "ODX",
		"<!-- a comment <ODX> -->\n<?pi x?>\n<!DOCTYPE x>\n<IPC-2581 rev=\"B\">": "IPC-2581",
		"<ODX/>": "ODX",
		"\ufeff<?xml version=\"1.0\"?><svg xmlns=\"x\">": "svg",
		`<?xml version="1.0"?>`:                          "",
		"":                                               "",
	}
	for head, want := range cases {
		if got := XMLRoot([]byte(head)); got != want {
			t.Errorf("XMLRoot(%q) = %q, want %q", head, got, want)
		}
	}
}
