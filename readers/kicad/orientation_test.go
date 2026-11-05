package kicad

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"github.com/panyam/agni/internal/geomath"
)

// orient-matrix.kicad_sch places a polarized 2-pin part and an asymmetric 3-pin part at every
// angle, plain and with each mirror. Every pin runs a short wire to a label named <ref>_<pin>, and
// orient-matrix.kicad-nets.txt is kicad-cli's netlist of it, so the labels are where KiCad puts
// the pins.
const orientMatrix = "orient-matrix.kicad_sch"

func readSheetGeometry(t *testing.T, name string) *geom.SchematicGeometry {
	t.Helper()
	g, err := ReadSchematicGeometry(bytes.NewReader(readFixture(t, name)), name)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// drawnPins maps "<ref>.<pin>" to where the renderer draws that pin's connect point.
func drawnPins(g *geom.SchematicGeometry) map[string]*geom.Point {
	syms := geomath.IndexSymbols(g)
	out := map[string]*geom.Point{}
	for _, sh := range g.Sheets {
		for _, pl := range sh.Placements {
			sym := syms.SymbolFor(pl)
			if sym == nil {
				continue
			}
			for _, pin := range sym.Pins {
				out[pl.RefDes+"."+pin.PortRef] = geomath.PlacePin(pl.Transform, pin)
			}
		}
	}
	return out
}

type pt struct{ x, y int64 }

func key(p *geom.Point) pt { return pt{p.GetX(), p.GetY()} }

// wireEnds lists every wire endpoint on the sheets.
func wireEnds(g *geom.SchematicGeometry) map[pt]bool {
	out := map[pt]bool{}
	for _, sh := range g.Sheets {
		for _, w := range sh.Wires {
			for _, pl := range w.Polylines {
				for _, p := range pl.Points {
					out[key(p)] = true
				}
			}
		}
	}
	return out
}

func TestEveryDrawnPinStartsItsOwnLabelledWire(t *testing.T) {
	g := readSheetGeometry(t, orientMatrix)
	labelAt := map[pt]string{}
	for _, sh := range g.Sheets {
		for _, l := range sh.Labels {
			at := l.Origin
			if l.Anchor != nil {
				at = l.Anchor
			}
			labelAt[key(at)] = l.Text
		}
	}
	// Each wire runs from a pin to its label, so the end that is not the label is the pin.
	pinFor := map[pt]string{}
	for _, sh := range g.Sheets {
		for _, w := range sh.Wires {
			for _, pl := range w.Polylines {
				a, b := key(pl.Points[0]), key(pl.Points[len(pl.Points)-1])
				if name, ok := labelAt[b]; ok {
					pinFor[a] = name
				}
			}
		}
	}
	drawn := drawnPins(g)
	if len(drawn) != 60 {
		t.Fatalf("drew %d pins, want 60 (12 placements of a 2-pin part, 12 of a 3-pin part)", len(drawn))
	}
	for ref, at := range drawn {
		want := strings.Replace(ref, ".", "_", 1)
		if got := pinFor[key(at)]; got != want {
			t.Errorf("%s draws at (%d, %d), where the wire is %q; want %q", ref, at.X, at.Y, got, want)
		}
	}
}

func TestMatrixPinsJoinTheNetsKiCadGivesThem(t *testing.T) {
	d, err := ReadSchematic(bytes.NewReader(readFixture(t, orientMatrix)), orientMatrix)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range d.Nets {
		for _, c := range n.Connections {
			got[c.ComponentRef+"."+c.PinRef] = n.Name
		}
	}
	rows := 0
	sc := bufio.NewScanner(bytes.NewReader(readFixture(t, "orient-matrix.kicad-nets.txt")))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		rows++
		if pin := f[0] + "." + f[1]; got[pin] != f[2] {
			t.Errorf("%s is on %q, kicad-cli puts it on %q", pin, got[pin], f[2])
		}
	}
	if rows != 60 {
		t.Fatalf("read %d kicad-cli rows, want 60", rows)
	}
}

// The Jetson cuts are agni issue 1322's symptom as it appeared. C133, C134 and C44 sit at 270
// with no mirror and drew lifted off their wires, Q5 and TP76 sit at 90 and drew onto U61, and
// C135 (mirrored at 270) drew right all along.
func TestJetsonCutsDrawEveryPinOnAWire(t *testing.T) {
	for _, c := range []struct {
		file string
		pins []string
	}{
		{"orient-jetson-caps.kicad_sch", []string{"C133.1", "C133.2", "C134.1", "C134.2", "C135.1", "C135.2", "C44.1", "C44.2"}},
		{"orient-jetson-fet.kicad_sch", []string{"Q5.1", "Q5.4", "Q5.5", "TP76.1"}},
	} {
		t.Run(c.file, func(t *testing.T) {
			g := readSheetGeometry(t, c.file)
			ends, drawn := wireEnds(g), drawnPins(g)
			for _, pin := range c.pins {
				at, ok := drawn[pin]
				if !ok {
					t.Fatalf("%s is not drawn", pin)
				}
				if !ends[key(at)] {
					t.Errorf("%s draws at (%d, %d), on no wire end", pin, at.X, at.Y)
				}
			}
		})
	}
}

// KiCad stores a field's angle relative to its symbol. C133's Reference is at 90 on a symbol at
// 270, so KiCad draws it at 0. The renderer adds the two angles, and the sum must be 0 rather than
// 180, since readable text turns a 180 run around and flips its justify onto the part's body.
func TestJetsonCapFieldsStayLevel(t *testing.T) {
	g := readSheetGeometry(t, "orient-jetson-caps.kicad_sch")
	checked := 0
	for _, ref := range []string{"C133", "C134", "C135", "C44"} {
		pl := placement(g, ref)
		if pl == nil {
			t.Fatalf("placement %s not found", ref)
		}
		for _, f := range pl.Fields {
			if !f.Visible {
				continue
			}
			checked++
			if sum := (pl.Transform.RotationDeg + f.RotationDeg) % 360; sum != 0 {
				t.Errorf("%s %s draws at %d degrees, want 0", ref, f.Name, pl.Transform.RotationDeg+f.RotationDeg)
			}
		}
	}
	if checked < 8 {
		t.Fatalf("checked %d visible fields, want at least 8 (a reference and a value per cap)", checked)
	}
}
