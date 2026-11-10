package render

import (
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// Each side's silkscreen, fab and courtyard layer draws in its own colour, as KiCad's do, so the
// back's legend reads apart from the front's. The SVG, the packed groups, the packed labels and the
// layer rows all agree, under both styles.
func TestEachSidesLayersTakeTheirOwnColour(t *testing.T) {
	line := &geom.Shape{Kind: geom.Shape_KIND_POLYLINE, Points: []*geom.Point{{X: 0, Y: 0}, {X: 1_000_000, Y: 0}}}
	for name, style := range Themes {
		want := map[string]string{
			"F.SilkS": style.SilkFront, "B.SilkS": style.SilkBack,
			"F.Fab": style.FabFront, "B.Fab": style.FabBack,
			"F.CrtYd": style.CourtyardFront, "B.CrtYd": style.CourtyardBack,
		}
		b := &geom.BoardGeometry{Outline: &geom.BoardOutline{Paths: []*geom.Polyline{{Points: []*geom.Point{{X: 0, Y: 0}, {X: 50_000_000, Y: 0}, {X: 50_000_000, Y: 50_000_000}}}}}}
		seen := map[string]bool{}
		for ly, c := range want {
			if seen[c] {
				t.Errorf("%s: %s shares its colour %s with another layer", name, ly, c)
			}
			seen[c] = true
			b.Graphics = append(b.Graphics, &geom.BoardGraphic{Layer: ly, Shape: line})
			b.Texts = append(b.Texts, &geom.BoardText{Text: "T" + ly, At: &geom.Point{X: 10_000_000, Y: 10_000_000}, Height: 1_000_000, Layer: ly})
		}
		doc := BoardSVG(b, WithStyle(style))
		packed := PackBoard(b, WithStyle(style))
		rows := BoardLayers(b, WithStyle(style))
		for ly, c := range want {
			if !strings.Contains(doc, `data-layer="`+ly+`">`+"\n"+`<polyline fill="none" stroke="`+c+`"`) {
				t.Errorf("%s: the SVG draws no %s outline in %s", name, ly, c)
			}
			if !strings.Contains(doc, `fill="`+c+`"`) || !strings.Contains(doc, ">T"+ly+"</text>") {
				t.Errorf("%s: the SVG draws no %s text in %s", name, ly, c)
			}
			row := boardRow(t, rows, ly)
			if row.GetColor() != c {
				t.Errorf("%s: the %s row reads %s, want %s", name, ly, row.GetColor(), c)
			}
			if got := packed.GetGroupColors()[row.GetGroups()[0]]; got != c {
				t.Errorf("%s: the packed %s group draws in %s, want %s", name, ly, got, c)
			}
			for _, l := range packed.GetLabels() {
				if l.GetText() == "T"+ly && l.GetColor() != c {
					t.Errorf("%s: the packed %s label draws in %s, want %s", name, ly, l.GetColor(), c)
				}
			}
		}
	}
}

// An inner layer takes KiCad's colour for its number, whichever inner layers the board uses, as
// kicad-cli draws In5 lavender on any board. An inner layer named otherwise takes the colour for its
// place among the inner layers.
func TestAnInnerLayerTakesTheColourOfItsNumber(t *testing.T) {
	seg := func(layer string) *geom.NetCopper {
		return &geom.NetCopper{Net: layer, Segments: []*geom.TrackSegment{{A: &geom.Point{X: 0, Y: 0}, B: &geom.Point{X: 1_000_000, Y: 0}, Width: 200_000, Layer: layer}}}
	}
	b := &geom.BoardGeometry{
		Layers: []*geom.BoardLayer{{Name: "F.Cu", Kind: "signal", Number: 0}, {Name: "In5.Cu", Kind: "signal", Number: 12}, {Name: "GND_PLANE", Kind: "signal", Number: 20}, {Name: "B.Cu", Kind: "signal", Number: 2}},
		Nets:   []*geom.NetCopper{seg("F.Cu"), seg("In5.Cu"), seg("GND_PLANE"), seg("B.Cu")},
	}
	for name, style := range Themes {
		colour := copperColors(style, b)
		for layer, want := range map[string]string{"In5.Cu": style.CopperInners[4], "GND_PLANE": style.CopperInners[1], "F.Cu": style.CopperFront, "B.Cu": style.CopperBack} {
			if got := colour(layer); got != want {
				t.Errorf("%s: %s draws in %s, want %s", name, layer, got, want)
			}
		}
	}
	if got := DarkStyle.CopperInners[4]; got != "#a7a5c6" {
		t.Errorf("In5.Cu is %s in the dark style, want KiCad's #a7a5c6", got)
	}
}
