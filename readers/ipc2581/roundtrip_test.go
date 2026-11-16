package ipc2581

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"google.golang.org/protobuf/proto"
)

// summarize renders the modeled IR (netlist + physical tier) as a canonical, order-independent
// string, so two designs that are semantically equal produce identical summaries regardless of
// ordering. It is the oracle's comparison key.
func summarize(d *ir.Design) string {
	var lines []string
	for _, c := range d.Components {
		lines = append(lines, fmt.Sprintf("component %s fp=%s", c.RefDes, c.FootprintRef))
	}
	for _, fp := range d.Footprints {
		lines = append(lines, "footprint "+fp.Name)
	}
	for _, n := range d.Nets {
		var conns []string
		for _, cn := range n.Connections {
			conns = append(conns, cn.ComponentRef+"."+cn.PinRef)
		}
		sort.Strings(conns)
		lines = append(lines, fmt.Sprintf("net %s = %s", n.Name, strings.Join(conns, ",")))
	}
	for _, l := range d.Layers {
		lines = append(lines, fmt.Sprintf("layer %s fn=%v", l.Name, l.Function))
	}
	for _, b := range d.Bom {
		rd := append([]string(nil), b.RefDes...)
		sort.Strings(rd)
		lines = append(lines, fmt.Sprintf("bom %s %d [%s]", b.Mpn, b.Quantity, strings.Join(rd, ",")))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestRoundTrip is the semantic round-trip oracle. It runs read -> IR -> Write -> read -> IR', and
// the two IRs must be equal on every modeled field. It confirms the reader and emitter agree.
func TestRoundTrip(t *testing.T) {
	d1, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	g1, err := ReadBoardGeometry(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatalf("ReadBoardGeometry: %v", err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, d1, g1.GetStackup()); err != nil {
		t.Fatalf("Write: %v", err)
	}

	d2, err := Read(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
	if err != nil {
		t.Fatalf("re-Read emitted XML: %v\n---emitted---\n%s", err, buf.String())
	}

	if s1, s2 := summarize(d1), summarize(d2); s1 != s2 {
		t.Errorf("round-trip changed the IR:\n--- before ---\n%s\n--- after ---\n%s", s1, s2)
	}

	// Sanity: the emitted document is a real IPC-2581 the reader accepts, with content.
	if len(d2.Components) == 0 || len(d2.Nets) == 0 || len(d2.Layers) == 0 {
		t.Errorf("round-tripped design is missing modeled content: %d comps, %d nets, %d layers",
			len(d2.Components), len(d2.Nets), len(d2.Layers))
	}
}

// TestWriteRoundTripsTheBoardsStack writes Allegro's stack, dielectrics, specs and air rows included,
// and reads the board back (agni 1660). Every row must come back the same, and the total the file
// states must stay the total its rows were written with.
func TestWriteRoundTripsTheBoardsStack(t *testing.T) {
	src := []byte(allegroStackup)
	d, err := Read(bytes.NewReader(src), "allegro.xml")
	if err != nil {
		t.Fatal(err)
	}
	g1, err := ReadBoardGeometry(bytes.NewReader(src), "allegro.xml")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, d, g1.GetStackup()); err != nil {
		t.Fatal(err)
	}
	g2, err := ReadBoardGeometry(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
	if err != nil {
		t.Fatalf("re-read: %v\n%s", err, buf.String())
	}
	s1, s2 := g1.GetStackup(), g2.GetStackup()
	dielectrics := 0
	for _, l := range s1.GetLayers() {
		if l.GetKind() == "dielectric" && l.EpsilonR != nil {
			dielectrics++
		}
	}
	if len(s1.GetLayers()) < 4 || dielectrics == 0 {
		t.Fatalf("the fixture's stack has %d rows and %d dielectrics with a constant", len(s1.GetLayers()), dielectrics)
	}
	if !proto.Equal(s1, s2) {
		t.Errorf("the stack changed through Write:\n%v\n%v", s1, s2)
	}
}

// TestWriteWithNoStackWritesNone holds that a design written with no board stack states no
// Stackup, so it never states a total no rows sum to.
func TestWriteWithNoStackWritesNone(t *testing.T) {
	d, err := Read(bytes.NewReader(readFixture(t, "board.xml")), "board.xml")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Write(&buf, d, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Stackup") {
		t.Errorf("a design with no stack wrote one:\n%s", buf.String())
	}
}

// TestWriteDeclaresTheLayersAStackNames writes a stack whose dielectrics the design declares no
// layer for, as a KiCad or ODB++ board's are (agni 1660). Each must come back with its kind and its
// dielectric type, so the writer declares a layer of the right function for it.
func TestWriteDeclaresTheLayersAStackNames(t *testing.T) {
	d := &ir.Design{Layers: []*ir.Layer{
		{Name: "TOP", Function: ir.LayerFunction_LAYER_FUNCTION_SIGNAL},
		{Name: "BOTTOM", Function: ir.LayerFunction_LAYER_FUNCTION_SIGNAL},
	}}
	cu, core, mask := int64(35_000), int64(1_530_000), int64(10_000)
	rows := []*geom.StackLayer{
		{Name: "F.Mask", Kind: "mask", Layer: "F.Mask", Thickness: &mask},
		{Name: "F.Cu", Kind: "copper", Layer: "F.Cu", Thickness: &cu},
		{Name: "Core", Kind: "dielectric", DielectricType: "core", Thickness: &core},
		{Name: "B.Cu", Kind: "copper", Layer: "B.Cu", Thickness: &cu},
	}
	var buf bytes.Buffer
	if err := Write(&buf, d, &geom.Stackup{Layers: rows}); err != nil {
		t.Fatal(err)
	}
	g, err := ReadBoardGeometry(bytes.NewReader(buf.Bytes()), "roundtrip.xml")
	if err != nil {
		t.Fatal(err)
	}
	got := g.GetStackup().GetLayers()
	if len(got) != len(rows) {
		t.Fatalf("%d rows came back, want %d:\n%s", len(got), len(rows), buf.String())
	}
	for i, w := range rows {
		if g := got[i]; g.GetKind() != w.GetKind() || g.GetDielectricType() != w.GetDielectricType() || g.GetThickness() != w.GetThickness() {
			t.Errorf("row %d came back %v, want %v", i, g, w)
		}
	}
}
