package render

import (
	"reflect"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// TestPackSheetPalette checks that the geometry palette and background travel on the wire from
// the Style, so the WebGL renderer colors geometry from the same Style as the SVG backend.
func TestPackSheetPalette(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{Id: "P1"}}}

	ps := PackSheet(g, g.Sheets[0])
	want := []string{DefaultStyle.Symbol, DefaultStyle.Wire, DefaultStyle.Pin, DefaultStyle.Free, DefaultStyle.Frame}
	if !reflect.DeepEqual(ps.GroupColors, want) {
		t.Errorf("group colors = %v, want %v", ps.GroupColors, want)
	}
	if ps.BackgroundColor != DefaultStyle.Page {
		t.Errorf("background = %q, want %q", ps.BackgroundColor, DefaultStyle.Page)
	}

	dark := PackSheet(g, g.Sheets[0], WithStyle(DarkStyle))
	if dark.GroupColors[groupWire] != DarkStyle.Wire {
		t.Errorf("dark wire = %q, want %q", dark.GroupColors[groupWire], DarkStyle.Wire)
	}
	if dark.BackgroundColor != DarkStyle.Page {
		t.Errorf("dark background = %q, want %q", dark.BackgroundColor, DarkStyle.Page)
	}
}

// TestWithStyleOverrides checks that a WithStyle override reaches both renderers (SVG colors +
// font, packed label colors + font), and that a plain render is unaffected by it.
func TestWithStyleOverrides(t *testing.T) {
	g := &geom.SchematicGeometry{Sheets: []*geom.SheetGeometry{{
		Id: "P1",
		Labels: []*geom.Label{
			{Text: "NET", Origin: &geom.Point{X: 10, Y: 10}, Height: 5},
		},
	}}}
	custom := DefaultStyle
	custom.Label = "#ff0000"
	custom.Font = "serif"

	svg := SheetSVG(g, g.Sheets[0], WithStyle(custom))
	if !strings.Contains(svg, "#ff0000") {
		t.Error("SVG did not use the overridden label color")
	}
	if !strings.Contains(svg, `font-family="serif"`) {
		t.Error("SVG did not use the overridden font")
	}

	// A plain render is unaffected: it uses the default label color, not the override.
	if plain := SheetSVG(g, g.Sheets[0]); strings.Contains(plain, "#ff0000") {
		t.Error("default SVG picked up the override color")
	}

	ps := PackSheet(g, g.Sheets[0], WithStyle(custom))
	if ps.FontFamily != "serif" {
		t.Errorf("packed font = %q, want serif", ps.FontFamily)
	}
	var found bool
	for _, l := range ps.Labels {
		if l.Text == "NET" {
			found = l.Color == "#ff0000"
		}
	}
	if !found {
		t.Error("packed NET label did not use the overridden color")
	}

	// The default pack still uses the default font.
	if ps := PackSheet(g, g.Sheets[0]); ps.FontFamily != DefaultStyle.Font {
		t.Errorf("default packed font = %q, want %q", ps.FontFamily, DefaultStyle.Font)
	}
}
