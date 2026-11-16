package render

import (
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/readers/odb"
)

func rectGraphic(ref, layer string, x0, y0, x1, y1 int64) *geom.BoardGraphic {
	return &geom.BoardGraphic{RefDes: ref, Layer: layer, Shape: &geom.Shape{
		Kind: geom.Shape_KIND_RECT, Points: []*geom.Point{{X: x0, Y: y0}, {X: x1, Y: y1}}}}
}

func bodiesBoard() *geom.BoardGeometry {
	h := int64(2_500_000)
	pad := func(x int64) *geom.Pad {
		return &geom.Pad{Number: "1", At: &geom.Point{X: x}, Size: &geom.Point{X: 1_000_000, Y: 1_000_000}}
	}
	return &geom.BoardGeometry{
		UnitNm:  1,
		Stackup: &geom.Stackup{Layers: []*geom.StackLayer{{Name: "F.Cu", Kind: "copper", Layer: "F.Cu"}, {Name: "B.Cu", Kind: "copper", Layer: "B.Cu"}}},
		Placements: []*geom.ComponentPlacement{
			{RefDes: "CY", At: &geom.Point{X: 10_000_000, Y: 10_000_000}, Layer: "F.Cu", Height: &h, Pads: []*geom.Pad{pad(0)}},
			{RefDes: "FAB", At: &geom.Point{X: 30_000_000, Y: 10_000_000}, Layer: "B.Cu", Mirror: true, Pads: []*geom.Pad{pad(0)}},
			{RefDes: "PADS", At: &geom.Point{X: 50_000_000, Y: 10_000_000}, Layer: "F.Cu", Pads: []*geom.Pad{pad(-2_000_000), pad(2_000_000)}},
		},
		Graphics: []*geom.BoardGraphic{
			rectGraphic("CY", "F.CrtYd", 7_000_000, 8_000_000, 13_000_000, 12_000_000),
			rectGraphic("CY", "F.Fab", 9_000_000, 9_000_000, 11_000_000, 11_000_000),
			rectGraphic("FAB", "B.Fab", 28_000_000, 9_000_000, 32_000_000, 11_000_000),
			rectGraphic("FAB", "B.SilkS", 0, 0, 90_000_000, 90_000_000),
		},
	}
}

func bodyOf(t *testing.T, v *geom.BoardStackView, ref string) *geom.BoardPartBody {
	t.Helper()
	for _, b := range v.GetBodies() {
		if b.GetRefDes() == ref {
			return b
		}
	}
	t.Fatalf("no body for %s among %d", ref, len(v.GetBodies()))
	return nil
}

// ringBox is the ring's bounding box and its signed area, positive when it runs counter-clockwise.
func ringBox(xy []float64) (x0, y0, x1, y1, area float64) {
	x0, y0, x1, y1 = xy[0], xy[1], xy[0], xy[1]
	n := len(xy) / 2
	for i := 0; i < n; i++ {
		x, y := xy[2*i], xy[2*i+1]
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
		nx, ny := xy[2*((i+1)%n)], xy[2*((i+1)%n)+1]
		area += x*ny - nx*y
	}
	return x0, y0, x1, y1, area / 2
}

func TestABodyTakesItsCourtyardThenItsFabThenItsPads(t *testing.T) {
	v := BoardStack(bodiesBoard())
	if got := len(v.GetBodies()); got != 3 {
		t.Fatalf("%d bodies, want one per placement, 3", got)
	}
	for ref, want := range map[string][4]float64{
		"CY":   {7e6, 8e6, 13e6, 12e6},
		"FAB":  {28e6, 9e6, 32e6, 11e6},
		"PADS": {47.5e6, 9.5e6, 52.5e6, 10.5e6},
	} {
		x0, y0, x1, y1, area := ringBox(bodyOf(t, v, ref).GetOutline())
		if [4]float64{x0, y0, x1, y1} != want {
			t.Errorf("%s outline box = %v, want %v", ref, [4]float64{x0, y0, x1, y1}, want)
		}
		if area <= 0 {
			t.Errorf("%s outline area = %v, want a counter-clockwise ring", ref, area)
		}
	}
}

func TestABodyStatesItsSideAndOnlyTheHeightTheFileGives(t *testing.T) {
	v := BoardStack(bodiesBoard())
	cy, fab := bodyOf(t, v, "CY"), bodyOf(t, v, "FAB")
	if cy.GetSide() != "front" || cy.HeightMm == nil || cy.GetHeightMm() != 2.5 {
		t.Errorf("CY = side %q height %v, want front at 2.5 mm", cy.GetSide(), cy.HeightMm)
	}
	if fab.GetSide() != "back" || fab.HeightMm != nil {
		t.Errorf("FAB = side %q height %v, want back with no height", fab.GetSide(), fab.HeightMm)
	}
}

func TestJetsonCutHasABodyForEveryPart(t *testing.T) {
	b := jetsonCutBoard(t)
	v := BoardStack(b)
	if len(b.GetPlacements()) < 10 || len(v.GetBodies()) != len(b.GetPlacements()) {
		t.Fatalf("%d bodies for %d placements, want one each", len(v.GetBodies()), len(b.GetPlacements()))
	}
	for _, body := range v.GetBodies() {
		if len(body.GetOutline()) < 6 || body.HeightMm != nil {
			t.Errorf("%s = %d outline floats, height %v; want a ring and no height, as KiCad states none", body.GetRefDes(), len(body.GetOutline()), body.HeightMm)
		}
	}
}

func TestBodiesTakeTheirColoursFromTheStyle(t *testing.T) {
	s := DefaultStyle
	s.PartBody, s.PartBodyDefault = "#010203", "#040506"
	v := BoardStack(bodiesBoard(), WithStyle(s))
	if v.GetBodyColor() != "#010203" || v.GetDefaultBodyColor() != "#040506" {
		t.Errorf("body colours = %q, %q; want the style's", v.GetBodyColor(), v.GetDefaultBodyColor())
	}
}

func TestABodyTakesTheColourOfItsFirstClassThatHasOne(t *testing.T) {
	classes := map[string][]string{"CY": {"tvs", "diode"}, "FAB": {"widget"}}
	v := BoardStack(bodiesBoard(), WithPartClasses(func(ref string) []string { return classes[ref] }))
	for ref, want := range map[string][2]string{
		"CY":   {"tvs", DefaultStyle.PartClass.Diode},
		"FAB":  {"widget", DefaultStyle.PartBody},
		"PADS": {"", DefaultStyle.PartBody},
	} {
		b := bodyOf(t, v, ref)
		if b.GetPartClass() != want[0] || b.GetColor() != want[1] {
			t.Errorf("%s = class %q colour %q, want %q %q", ref, b.GetPartClass(), b.GetColor(), want[0], want[1])
		}
	}
	if c := bodyOf(t, BoardStack(bodiesBoard(), WithPartClasses(nil)), "CY").GetColor(); c != DefaultStyle.PartBody {
		t.Errorf("with no class function CY draws %q, want PartBody %q", c, DefaultStyle.PartBody)
	}
}

func TestEveryPartClassColourDiffers(t *testing.T) {
	for _, s := range []Style{DefaultStyle, DarkStyle} {
		seen := map[string]string{}
		for _, class := range []string{"resistor", "capacitor", "inductor", "diode", "led", "ic", "connector", "transistor", "clock", "test_point", "fuse"} {
			c := s.PartClass.of(class)
			if c == "" || c == s.PartBody {
				t.Errorf("%s has no colour of its own", class)
			}
			if prev, ok := seen[c]; ok {
				t.Errorf("%s and %s share %s", prev, class, c)
			}
			seen[c] = class
		}
	}
}

func TestTheBoardLookTakesItsColoursFromTheStyle(t *testing.T) {
	s := DefaultStyle
	s.BoardMask, s.MaskOverCopper, s.PadFinish, s.BoardSilk = "#010101", "#020202", "#030303", "#040404"
	l := BoardStack(bodiesBoard(), WithStyle(s)).GetBoardLook()
	if l.GetMask() != "#010101" || l.GetMaskOverCopper() != "#020202" || l.GetPadFinish() != "#030303" || l.GetSilk() != "#040404" {
		t.Errorf("board look = %v, want the style's four colours", l)
	}
}

// An ODB++ part stands on its package outline, at the height its .comp_height gives (agni 1656).
// inch-job's U1 has an RC outline wider than its pads, so a block on its pads would be narrower.
func TestAnODBPartStandsOnItsPackageOutline(t *testing.T) {
	b, err := odb.ReadBoardGeometry(os.DirFS(filepath.Join("..", "..", "readers", "odb", "testdata", "inch-job")), "inch-job")
	if err != nil {
		t.Fatal(err)
	}
	const mil = 25_400
	x0, y0, x1, y1, _ := ringBox(bodyOf(t, BoardStack(b), "U1").GetOutline())
	if got, want := [4]float64{x0, y0, x1, y1}, [4]float64{470 * mil, 420 * mil, 530 * mil, 580 * mil}; got != want {
		t.Errorf("U1's block = %v, want its package outline %v", got, want)
	}
}

// A mounting hole stands no block, and a part with drilled pins does (agni 1607). MH is one pin on a
// 3.5 mm drill ringed by 0.5 mm vias, as Glasgow's are. TP is one pin on a 1 mm drill, and J two
// pins on 3.5 mm drills.
func TestAMountingHoleStandsNoBlock(t *testing.T) {
	pad := func(num string, x, drill int64) *geom.Pad {
		return &geom.Pad{Number: num, At: &geom.Point{X: x}, Size: &geom.Point{X: drill + 500_000, Y: drill + 500_000}, Drill: drill}
	}
	b := &geom.BoardGeometry{UnitNm: 1, Stackup: bodiesBoard().GetStackup(), Placements: []*geom.ComponentPlacement{
		{RefDes: "MH", At: &geom.Point{}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", 0, 3_500_000), pad("1", 2_600_000, 500_000), pad("1", -2_600_000, 500_000)}},
		{RefDes: "TP", At: &geom.Point{X: 20_000_000}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", 0, 1_000_000)}},
		{RefDes: "J", At: &geom.Point{X: 40_000_000}, Layer: "F.Cu", Pads: []*geom.Pad{pad("1", 0, 3_500_000), pad("2", 5_000_000, 3_500_000)}},
	}}
	got := map[string]bool{}
	for _, body := range BoardStack(b).GetBodies() {
		got[body.GetRefDes()] = true
	}
	if want := map[string]bool{"TP": true, "J": true}; len(got) != len(want) || !got["TP"] || !got["J"] {
		t.Errorf("bodies for %v, want TP and J and not the mounting hole MH", got)
	}
}
