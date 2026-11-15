package ipc2581

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/core/stackup"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// allegroStackup is allegroShapes with the stack Allegro's export writes: a CadHeader of Specs, a
// Stackup bracketed by zero-thickness air rows that name no layer, and a blind drill layer from the
// top to the plane.
var allegroStackup = func() string {
	s := strings.Replace(allegroShapes, `<Ecad name="ecad">`, `<Ecad name="ecad">
    <CadHeader units="MILLIMETER">
      <Spec name="SPEC_LAYER_TOP">
        <General type="MATERIAL"><Property text="CU_40UM_PLAT_EXT"/></General>
        <Dielectric type="DIELECTRIC_CONSTANT"><Property value="2.8000"/></Dielectric>
      </Spec>
      <Spec name="SPEC_LAYER_DIELECTRIC_BELOW_TOP">
        <General type="MATERIAL"><Property text="HFFR4_GENERIC_PREG"/></General>
        <General type="OTHER" comment="LAYER_TYPE"><Property text="DIELECTRIC"/></General>
        <Dielectric type="DIELECTRIC_CONSTANT"><Property value="4.6000"/></Dielectric>
        <Dielectric type="LOSS_TANGENT"><Property value="0.0200"/></Dielectric>
      </Spec>
    </CadHeader>`, 1)
	s = strings.Replace(s, `<Layer name="IPC2581DRILL_1-3" layerFunction="DRILL" side="NONE" polarity="POSITIVE"/>`,
		`<Layer name="IPC2581DRILL_1-3" layerFunction="DRILL" side="NONE" polarity="POSITIVE"/>
      <Layer name="IPC2581DRILL_1-2" layerFunction="DRILL" side="NONE" polarity="POSITIVE"/>
      <Stackup name="ECAD_CROSS_SECTION" overallThickness="1.2000">
        <StackupGroup name="RootGroup" thickness="1.2000">
          <StackupLayer layerOrGroupRef="SURFACE_INDEX_1" thickness="0.0" sequence="1"/>
          <StackupLayer layerOrGroupRef="TOP" thickness="0.0400" sequence="2"><SpecRef id="SPEC_LAYER_TOP"/></StackupLayer>
          <StackupLayer layerOrGroupRef="DIELECTRIC_BELOW_TOP" thickness="0.1000" sequence="3"><SpecRef id="SPEC_LAYER_DIELECTRIC_BELOW_TOP"/></StackupLayer>
          <StackupLayer layerOrGroupRef="L2-GND" thickness="0.0350" sequence="4"/>
          <StackupLayer layerOrGroupRef="BOTTOM" sequence="5"/>
          <StackupLayer layerOrGroupRef="SURFACE_BELOW_BOTTOM" thickness="0.0" sequence="6"/>
        </StackupGroup>
      </Stackup>`, 1)
	return strings.Replace(s, `</LayerFeature>
      </Step>`, `</LayerFeature>
        <LayerFeature layerRef="IPC2581DRILL_1-2">
          <Set net="N1"><Hole name="H4" diameter="0.1500" platingStatus="VIA" x="3.0000" y="3.0000"/></Set>
        </LayerFeature>
      </Step>`, 1)
}()

func readStack(t *testing.T, doc string) *geom.BoardGeometry {
	t.Helper()
	g, err := ReadBoardGeometry(strings.NewReader(doc), "stack.xml")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func describeRows(g *geom.BoardGeometry) []string {
	var out []string
	for _, l := range g.GetStackup().GetLayers() {
		out = append(out, stackup.Describe(l, g.GetUnitNm()))
	}
	return out
}

func TestStackupFromAllegroExport(t *testing.T) {
	g := readStack(t, allegroStackup)
	want := []string{
		"F.Cu copper 0.04 mm CU_40UM_PLAT_EXT er=2.8",
		"DIELECTRIC_BELOW_TOP dielectric 0.1 mm HFFR4_GENERIC_PREG core er=4.6 tand=0.02",
		"In1.Cu copper 0.035 mm",
		"B.Cu copper thickness unknown",
	}
	if got := describeRows(g); !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	if s := g.GetStackup(); s.Thickness == nil || s.GetThickness() != 1_200_000 {
		t.Fatalf("board thickness = %v, want 1200000 nm", s.Thickness)
	}
	if msg := stackup.Check(g.GetStackup(), g.GetUnitNm()); msg != "" {
		t.Errorf("Check = %q, want no verdict while B.Cu has no thickness", msg)
	}
}

func TestStackupFromAgnisOwnExport(t *testing.T) {
	g := readStack(t, string(readTestdata(t, "board_geom.xml")))
	want := []string{"F.Cu copper 0.035 mm COPPER", "B.Cu copper 0.035 mm COPPER"}
	if got := describeRows(g); !slices.Equal(got, want) {
		t.Fatalf("stack rows:\n got %q\nwant %q", got, want)
	}
	if want := "the layers sum to 0.07 mm, and the file states a board thickness of 1.6 mm"; stackup.Check(g.GetStackup(), 1) != want {
		t.Errorf("Check = %q, want %q", stackup.Check(g.GetStackup(), 1), want)
	}
}

func TestBlindDrillLayerByAllegroName(t *testing.T) {
	g := readStack(t, allegroStackup)
	var spans []string
	for _, nc := range g.GetNets() {
		for _, v := range nc.GetVias() {
			spans = append(spans, v.GetLayerFrom()+".."+v.GetLayerTo())
		}
	}
	slices.Sort(spans)
	if want := []string{"F.Cu..B.Cu", "F.Cu..In1.Cu"}; !slices.Equal(spans, want) {
		t.Fatalf("via spans = %q, want %q", spans, want)
	}
}

func TestBoardWithNoStackupElement(t *testing.T) {
	g := readStack(t, allegroShapes)
	if g.Stackup != nil {
		t.Fatalf("a file with no <Stackup> read %v", g.Stackup)
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
