package kicad

import (
	"fmt"
	"strings"
	"testing"
)

// mirrorProbe places one two-pin symbol at a given rotation and mirror, with four labelled wire
// stubs at the compass points around it. Whichever label a pin joins says where that pin landed, so
// the placement transform is readable straight off the netlist.
//
// Pin 1 sits at local (-3.81, 0) and pin 2 at (+3.81, 0), which is the shape of the TVS diode array
// StickHub places mirrored and rotated.
const mirrorProbe = `(kicad_sch
	(version 20250114)
	(generator "eeschema")
	(uuid "11111111-1111-1111-1111-111111111111")
	(paper "A4")
	(lib_symbols
		(symbol "test:D2"
			(pin_numbers (hide yes))
			(pin_names (offset 0))
			(property "Reference" "D")
			(symbol "D2_0_1" (rectangle (start -1.016 1.016) (end 1.016 -1.016) (fill (type background))))
			(symbol "D2_1_1"
				(pin passive line (at -3.81 0 0) (length 2.54) (name "A") (number "1"))
				(pin passive line (at 3.81 0 180) (length 2.54) (name "K") (number "2")))))
	(symbol (lib_id "test:D2") (at 100 100 %d)%s
		(unit 1) (uuid "dut")
		(property "Reference" "D1" (at 100 96 0) (effects (font (size 1.27 1.27)))))
	(wire (pts (xy 103.81 100) (xy 108 100)) (uuid "we"))
	(label "EAST" (at 106 100 0) (effects (font (size 1.27 1.27))))
	(wire (pts (xy 96.19 100) (xy 92 100)) (uuid "ww"))
	(label "WEST" (at 94 100 0) (effects (font (size 1.27 1.27))))
	(wire (pts (xy 100 103.81) (xy 100 108)) (uuid "ws"))
	(label "SOUTH" (at 100 106 0) (effects (font (size 1.27 1.27))))
	(wire (pts (xy 100 96.19) (xy 100 92)) (uuid "wn"))
	(label "NORTH" (at 100 94 0) (effects (font (size 1.27 1.27)))))`

// TestMirroredPinPlacement pins where each pin of a placed symbol lands, for every combination of
// rotation and mirror. The expectations are kicad-cli's own answers, taken by exporting a netlist
// from each of these twelve schematics; they are not values we chose.
//
// The four mirrored 90 and 270 rows are agni issue 577. KiCad applies the rotation and THEN the
// mirror, geomath.ApplyTransform applies the mirror and then the rotation, and a reflection does not
// commute with a quarter turn, so a symbol placed mirrored at 90 or 270 had its two pins swapped onto
// each other's nets. Everything else already agreed, which is why the defect survived: it needs both
// a mirror and an odd quarter turn, and it moves one connection out of a net and another in, so the
// net COUNT does not change. On StickHub that was 19 of 47 nets wrong with 47 nets reported.
func TestMirroredPinPlacement(t *testing.T) {
	cases := []struct {
		rot        int
		mirror     string
		pin1, pin2 string
	}{
		{0, "", "WEST", "EAST"},
		{90, "", "SOUTH", "NORTH"},
		{180, "", "EAST", "WEST"},
		{270, "", "NORTH", "SOUTH"},

		{0, "x", "WEST", "EAST"},
		{90, "x", "NORTH", "SOUTH"},
		{180, "x", "EAST", "WEST"},
		{270, "x", "SOUTH", "NORTH"},

		{0, "y", "EAST", "WEST"},
		{90, "y", "SOUTH", "NORTH"},
		{180, "y", "WEST", "EAST"},
		{270, "y", "NORTH", "SOUTH"},
	}
	for _, c := range cases {
		name := fmt.Sprintf("rot%d", c.rot)
		mirror := ""
		if c.mirror != "" {
			name += "-mirror-" + c.mirror
			mirror = "\n\t\t(mirror " + c.mirror + ")"
		}
		t.Run(name, func(t *testing.T) {
			src := fmt.Sprintf(mirrorProbe, c.rot, mirror)
			d, _, err := ReadSchematicHierarchyNets("probe.kicad_sch", []byte(src), nil)
			if err != nil {
				t.Fatal(err)
			}
			at := map[string]string{}
			for _, n := range d.GetNets() {
				for _, conn := range n.GetConnections() {
					at[conn.GetPinRef()] = n.GetName()
				}
			}
			if at["1"] != c.pin1 || at["2"] != c.pin2 {
				t.Errorf("pin1 at %q, pin2 at %q; kicad-cli puts pin1 at %q and pin2 at %q",
					at["1"], at["2"], c.pin1, c.pin2)
			}
		})
	}
}

// TestMirroredProbeIsWellFormed is the positive control for the table above. A probe whose pins reach
// no stub would report empty net names for both pins, and every row asserting a swap would pass
// against a design where nothing is connected at all.
func TestMirroredProbeIsWellFormed(t *testing.T) {
	src := fmt.Sprintf(mirrorProbe, 0, "")
	d, _, err := ReadSchematicHierarchyNets("probe.kicad_sch", []byte(src), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, n := range d.GetNets() {
		names = append(names, n.GetName())
	}
	for _, want := range []string{"EAST", "WEST"} {
		if !strings.Contains(strings.Join(names, " "), want) {
			t.Errorf("probe produced %v, with no %s net; the stubs are not reaching the pins", names, want)
		}
	}
}
