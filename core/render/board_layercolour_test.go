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
