package odb

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

// gatewayJob is the gateway board's ODB++ export, which the odb-verification example reads too.
const gatewayJob = "../../examples/common/designs/odb/gateway-layout"

func readJob(t *testing.T, fsys fs.FS) (*ir.Design, *geom.BoardGeometry) {
	t.Helper()
	d, err := Read(fsys, "job")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	g, err := ReadBoardGeometry(fsys, "job")
	if err != nil {
		t.Fatalf("ReadBoardGeometry: %v", err)
	}
	return d, g
}

func netPins(d *ir.Design) map[string]string {
	out := map[string]string{}
	for _, n := range d.GetNets() {
		var pins []string
		for _, c := range n.GetConnections() {
			pins = append(pins, c.GetComponentRef()+"."+c.GetPinRef())
		}
		out[n.GetName()] = strings.Join(pins, " ")
	}
	return out
}

func placement(t *testing.T, g *geom.BoardGeometry, ref string) *geom.ComponentPlacement {
	t.Helper()
	for _, pl := range g.GetPlacements() {
		if pl.GetRefDes() == ref {
			return pl
		}
	}
	t.Fatalf("no placement %s", ref)
	return nil
}

// TestReadsAnInchJob reads the hand-written job in inches, which carries the grammar KiCad's export
// never writes: an inner plane, a legacy orient code, a quoted property with a space, a plane with a
// hole, and silkscreen text.
func TestReadsAnInchJob(t *testing.T) {
	d, g := readJob(t, os.DirFS("testdata/inch-job"))

	if d.GetName() != "inchjob" || d.GetSourceFormat() != "odb++" {
		t.Errorf("design = %q from %q, want inchjob from odb++", d.GetName(), d.GetSourceFormat())
	}
	got := netPins(d)
	want := map[string]string{"VIN": "U1.1 R1.1", "GND": "U1.2 R1.2"}
	if len(got) != len(want) {
		t.Errorf("nets = %v, want %v", got, want)
	}
	for n, pins := range want {
		if got[n] != pins {
			t.Errorf("net %s = %q, want %q", n, got[n], pins)
		}
	}
	vals := map[string]string{}
	for _, c := range d.GetComponents() {
		vals[c.GetRefDes()] = c.GetAttributes()["Value"] + "/" + c.GetAttributes()["side"]
	}
	if vals["U1"] != "LDO 3V3/top" || vals["R1"] != "10k/bottom" {
		t.Errorf("values/sides = %v, want U1 LDO 3V3/top and R1 10k/bottom", vals)
	}

	var names []string
	for _, l := range g.GetLayers() {
		names = append(names, l.GetName())
	}
	if got, want := strings.Join(names, " "), "F.SilkS F.Cu In1.Cu B.Cu DRILL"; got != want {
		t.Errorf("layers = %q, want %q", got, want)
	}

	const mil = 25400 // nanometres
	u1 := placement(t, g, "U1")
	if u1.GetAt().GetX() != 500*mil || u1.GetAt().GetY() != 500*mil || u1.GetRotationDeg() != -90 || u1.GetMirror() || u1.GetLayer() != "F.Cu" {
		t.Errorf("U1 at %v rot %v mirror %v layer %s, want (12.7mm,12.7mm) rot -90 front", u1.GetAt(), u1.GetRotationDeg(), u1.GetMirror(), u1.GetLayer())
	}
	p1 := u1.GetPads()[0]
	if p1.GetNumber() != "1" || p1.GetAt().GetX() != -50*mil || p1.GetAt().GetY() != 0 || p1.GetNet() != "VIN" {
		t.Errorf("U1.1 = %v, want pin 1 at (-50mil, 0) on VIN", p1)
	}
	if p1.GetShape() != "rect" || p1.GetSize().GetX() != 60*mil || p1.GetSize().GetY() != 40*mil || p1.GetRotationDeg() != -90 {
		t.Errorf("U1.1 shape %s %v rot %v, want a 60x40mil rect at -90 (legacy orient 1)", p1.GetShape(), p1.GetSize(), p1.GetRotationDeg())
	}
	if p2 := u1.GetPads()[1]; p2.GetRotationDeg() != -90 {
		t.Errorf("U1.2 rot = %v, want -90 (orient 8 at 90 clockwise)", p2.GetRotationDeg())
	}
	r1 := placement(t, g, "R1")
	if !r1.GetMirror() || r1.GetLayer() != "B.Cu" || r1.GetPads()[0].GetAt().GetX() != -30*mil || r1.GetPads()[0].GetLayers()[0] != "B.Cu" {
		t.Errorf("R1 = mirror %v layer %s pad1 %v, want a mirrored back part with pad 1 at (-30mil, 0)", r1.GetMirror(), r1.GetLayer(), r1.GetPads()[0])
	}

	var vin *geom.NetCopper
	for _, nc := range g.GetNets() {
		if nc.GetNet() == "VIN" {
			vin = nc
		}
	}
	if vin == nil {
		t.Fatalf("no VIN copper in %v", g.GetNets())
	}
	// One straight run and one arc of arcSegments pieces on the front, one run on the back.
	if got, want := len(vin.GetSegments()), 1+arcSegments+1; got != want {
		t.Errorf("VIN segments = %d, want %d", got, want)
	}
	if len(vin.GetVias()) != 1 {
		t.Fatalf("VIN vias = %d, want 1", len(vin.GetVias()))
	}
	v := vin.GetVias()[0]
	if v.GetDrill() != 12*mil || v.GetSize() != 10*mil || v.GetLayerFrom() != "F.Cu" || v.GetLayerTo() != "B.Cu" {
		t.Errorf("via = %v, want 12mil drill, 10mil land, F.Cu to B.Cu", v)
	}
	if len(g.GetZones()) != 1 || g.GetZones()[0].GetNet() != "GND" || g.GetZones()[0].GetLayer() != "In1.Cu" || len(g.GetZones()[0].GetOutline().GetPoints()) != 5 {
		t.Errorf("zones = %v, want the GND plane's outline on In1.Cu", g.GetZones())
	}
	if len(g.GetTexts()) != 1 || g.GetTexts()[0].GetText() != "U1" || g.GetTexts()[0].GetLayer() != "F.SilkS" || g.GetTexts()[0].GetHeight() != 60*mil {
		t.Errorf("texts = %v, want U1 on F.SilkS at 60mil", g.GetTexts())
	}
	if len(g.GetGraphics()) != 1 {
		t.Errorf("graphics = %d, want the one silkscreen line", len(g.GetGraphics()))
	}
	if pts := g.GetOutline().GetPaths(); len(pts) != 1 || len(pts[0].GetPoints()) != 5 {
		t.Errorf("outline = %v, want one closed rectangle", g.GetOutline())
	}
}

// archive builds a tar.gz or zip of a directory, the job one level down as producers wrap it.
func archive(t *testing.T, dir, kind string) []byte {
	t.Helper()
	var buf bytes.Buffer
	var add func(name string, b []byte)
	var closeAll func()
	switch kind {
	case "tgz":
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		add = func(name string, b []byte) {
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write(b); err != nil {
				t.Fatal(err)
			}
		}
		closeAll = func() { tw.Close(); gz.Close() }
	case "zip":
		zw := zip.NewWriter(&buf)
		add = func(name string, b []byte) {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(b); err != nil {
				t.Fatal(err)
			}
		}
		closeAll = func() { zw.Close() }
	}
	n := 0
	err := fs.WalkDir(os.DirFS(dir), ".", func(p string, de fs.DirEntry, err error) error {
		if err != nil || de.IsDir() {
			return err
		}
		b, err := os.ReadFile(path.Join(dir, p))
		if err != nil {
			return err
		}
		add("wrapped/"+p, b)
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("%s holds no files", dir)
	}
	closeAll()
	return buf.Bytes()
}

// TestArchivesReadAsTheirDirectory holds a .tgz and a .zip of a job to the job's directory read,
// whichever folder the archive wraps it in.
func TestArchivesReadAsTheirDirectory(t *testing.T) {
	wantD, wantG := readJob(t, os.DirFS(gatewayJob))
	if len(wantD.GetNets()) == 0 {
		t.Fatal("the gateway job reads no nets")
	}
	for _, kind := range []string{"tgz", "zip"} {
		fsys, err := FromArchive(archive(t, gatewayJob, kind))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		d, g := readJob(t, fsys)
		if !proto.Equal(d, wantD) || !proto.Equal(g, wantG) {
			t.Errorf("%s reads differently from the job's directory", kind)
		}
	}
	if _, err := FromArchive([]byte("not an archive")); err == nil {
		t.Error("FromArchive accepted bytes that are no archive")
	}
	if IsArchiveJob(archive(t, "testdata/lzw", "zip")) {
		t.Error("a zip with no matrix/matrix was taken for an ODB++ job")
	}
}

// TestCompressedMembersReadAsPlain reads the board job with every features file compressed, as
// Valor-lineage tools write a job, and holds it to the plain one.
func TestCompressedMembersReadAsPlain(t *testing.T) {
	wantD, wantG := readJob(t, os.DirFS("testdata/board"))
	d, g := readJob(t, os.DirFS("testdata/board-compressed"))
	if len(wantG.GetPlacements()) == 0 {
		t.Fatal("the board job reads no placements")
	}
	if !proto.Equal(d, wantD) || !proto.Equal(g, wantG) {
		t.Error("the compressed job reads differently from the plain one")
	}
}

// TestUncompressMatchesCompress decodes one input compress(1) wrote at three maximum code widths. At
// 9 bits the table fills every few hundred codes, so the clears and the group padding after each run
// many times, and at 16 the width grows through every step.
func TestUncompressMatchesCompress(t *testing.T) {
	const want = "5e1512f3f06181b66a4905c77eeb96069c016cbae2d7fdbbb58cca59a988eeb5"
	for _, bits := range []string{"9", "12", "16"} {
		z, err := os.ReadFile("testdata/lzw/features.b" + bits + ".Z")
		if err != nil {
			t.Fatal(err)
		}
		out, err := uncompress(z)
		if err != nil {
			t.Fatalf("%s bits: %v", bits, err)
		}
		sum := sha256.Sum256(out)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s bits: decoded %d bytes hashing %s, want the original's %s", bits, len(out), got, want)
		}
	}
	if _, err := uncompress([]byte("plain")); err == nil {
		t.Error("uncompress accepted data with no compress(1) header")
	}
}

func TestParseSymbol(t *testing.T) {
	for _, c := range []struct {
		name  string
		w, h  int64
		shape string
	}{
		{"r250", 250000, 250000, "circle"},
		{"s100", 100000, 100000, "rect"},
		{"rect600.0x700.0", 600000, 700000, "rect"},
		{"rect1200.0x1400.0xr300.0", 1200000, 1400000, "roundrect"},
		{"rect100x50xc10x13", 100000, 50000, "chamfered_rect"},
		{"oval1000x500", 1000000, 500000, "oval"},
		{"di40x20", 40000, 20000, "diamond"},
		{"donut_r100x50", 100000, 100000, "circle"},
		{"my_user_pad", 0, 0, "my_user_pad"},
	} {
		got := parseSymbol(symbolRef{name: c.name, nm: 1000})
		if got.w != c.w || got.h != c.h || got.shape != c.shape {
			t.Errorf("parseSymbol(%q) = %+v, want %dx%d %s", c.name, got, c.w, c.h, c.shape)
		}
	}
}

func TestOrient(t *testing.T) {
	for _, c := range []struct {
		in     []string
		rot    float64
		mirror bool
	}{
		{[]string{"0"}, 0, false},
		{[]string{"1"}, 90, false},
		{[]string{"6"}, 180, true},
		{[]string{"8", "37.5"}, 37.5, false},
		{[]string{"9", "270"}, 270, true},
	} {
		rot, mirror := orient(c.in)
		if rot != c.rot || mirror != c.mirror {
			t.Errorf("orient(%v) = %v, %v, want %v, %v", c.in, rot, mirror, c.rot, c.mirror)
		}
	}
}

// TestArchiveRefusesEscapingMembers keeps a member named ../x or an AppleDouble shadow out of the
// job's file system.
func TestArchiveRefusesEscapingMembers(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"job/matrix/matrix", "job/matrix/matrix", true},
		{"../../etc/passwd", "etc/passwd", true},
		{"/abs/matrix/matrix", "abs/matrix/matrix", true},
		{"job/._matrix", "", false},
		{"__MACOSX/job/matrix/matrix", "", false},
	} {
		got, ok := cleanMember(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("cleanMember(%q) = %q, %v, want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// TestFindRootPrefersTheShallowestJob picks the job an archive holds at its top over one nested
// inside it (a job carrying a copy of another in its user/ folder).
func TestFindRootPrefersTheShallowestJob(t *testing.T) {
	fsys := fstest.MapFS{
		"outer/matrix/matrix":                {Data: []byte("STEP {\nNAME=PCB\n}\n")},
		"outer/user/old/inner/matrix/matrix": {Data: []byte("x")},
		"outer/steps/pcb/eda/data":           {Data: []byte("")},
	}
	root, err := FindRoot(fsys)
	if err != nil || root != "outer" {
		t.Errorf("FindRoot = %q, %v, want outer", root, err)
	}
}
