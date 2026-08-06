package foreign

import "testing"

// TestParseItemFormMatrix is the form-matrix oracle for the description grammar, the same discipline
// the EDIF name grammar earned: every shape the import claims to understand is a row here, so adding a
// shape means adding a row and a missing shape fails a test rather than a corpus file.
//
// Every input is a VERBATIM description captured from kicad-cli output over this repo's board and
// schematic fixtures. They are UI strings with no stability guarantee, so evidence is the only honest
// source for them, and a captured string is the only kind that proves the table matches reality.
func TestParseItemFormMatrix(t *testing.T) {
	cases := []struct {
		desc string
		want itemRef
	}{
		// Board.
		{"Pad 1 [VCC] of R1 on B.Cu", itemRef{RefDes: "R1", Pin: "1", Net: "VCC"}},
		{"Pad 2 [<no net>] of C1 on B.Cu", itemRef{RefDes: "C1", Pin: "2"}},
		{"PTH pad 1 [SIG] of J1", itemRef{RefDes: "J1", Pin: "1", Net: "SIG"}},
		{"PTH pad 2 [<no net>] of J1", itemRef{RefDes: "J1", Pin: "2"}},
		{"Track [VCC] on F.Cu, length 10.8000 mm", itemRef{Net: "VCC"}},
		{"Track [GND] on F.Cu, length 1.0000 mm", itemRef{Net: "GND"}},
		{"Via [GND] on F.Cu - B.Cu", itemRef{Net: "GND"}},
		{"Footprint R1", itemRef{RefDes: "R1"}},
		{"Circle of R1 on F.Silkscreen", itemRef{RefDes: "R1"}},
		{"Reference field of R1", itemRef{RefDes: "R1"}},
		// Schematic.
		{"Symbol J1 Pin 1 [VBUS, Passive, Line]", itemRef{RefDes: "J1", Pin: "1"}},
		{"Symbol U1 Pin 1 [VIN, Power input, Line]", itemRef{RefDes: "U1", Pin: "1"}},
		{"Symbol #PWR30 Pin 1 [Power input, Line]", itemRef{RefDes: "#PWR30", Pin: "1"}},
		{"Symbol #FLG01 Pin 1 [pwr, Power output, Line]", itemRef{RefDes: "#FLG01", Pin: "1"}},
		{"Symbol J1 [USB_CONN]", itemRef{RefDes: "J1"}},
		{"Symbol #PWR14 [GND]", itemRef{RefDes: "#PWR14"}},
		{"Label 'VCC'", itemRef{Net: "VCC"}},
		{"Global Label 'HARD'", itemRef{Net: "HARD"}},
		// Shapes that name no joinable entity. These are not failures: a wire's description carries
		// only its orientation and length, and board outline geometry belongs to no component or net.
		{"Horizontal Wire, length 0.1500 mm", itemRef{}},
		{"Vertical Wire, length 0.0508 mm", itemRef{}},
		{"Rectangle on Edge.Cuts", itemRef{}},
		{"Arc on Edge.Cuts", itemRef{}},
		{"", itemRef{}},
	}
	for _, tc := range cases {
		if got := parseItem(tc.desc); got != tc.want {
			t.Errorf("parseItem(%q) = %+v, want %+v", tc.desc, got, tc.want)
		}
	}
}

// TestParseItemPrefersTheMoreSpecificShape pins the ordering the table depends on. "Symbol U1 Pin 1"
// must not fall through to the bare-symbol pattern, because losing the pin silently widens a
// pin-scoped violation to the whole part.
func TestParseItemPrefersTheMoreSpecificShape(t *testing.T) {
	if got := parseItem("Symbol U1 Pin 3 [VOUT, Power output, Line]"); got.Pin != "3" {
		t.Errorf("pin lost: %+v", got)
	}
	if got := parseItem("Pad 5 [D+] of J2 on F.Cu"); got.Pin != "5" || got.Net != "D+" {
		t.Errorf("pad shape lost a field: %+v", got)
	}
}

// TestNoNetIsNotANetName pins that KiCad's literal "<no net>" never becomes a net. Carrying it through
// would invent a net by that name and join every unconnected pad on a board to it — a wrong join,
// which is worse than no join because it attaches real violations to an innocent entity.
func TestNoNetIsNotANetName(t *testing.T) {
	if got := parseItem("Pad 2 [<no net>] of C1 on B.Cu"); got.Net != "" {
		t.Errorf("net = %q, want empty", got.Net)
	}
}

// TestResidueClassSeparatesBenignFromInteresting pins that the residue is bucketed by KIND. A summary
// saying "20 not attached" is useless; one saying they were all wires tells a reader the import is
// working, and one naming an unrecognized shape tells them it is not.
func TestResidueClassSeparatesBenignFromInteresting(t *testing.T) {
	wire := residueClass("Horizontal Wire, length 0.1500 mm")
	edge := residueClass("Rectangle on Edge.Cuts")
	unknown := residueClass("Frobnicator 7 of the thing")
	if wire == edge || wire == unknown || edge == unknown {
		t.Errorf("classes collapsed: wire=%q edge=%q unknown=%q", wire, edge, unknown)
	}
	if unknown == "" {
		t.Error("an unrecognized shape must still get a class, or it vanishes from the summary")
	}
}
