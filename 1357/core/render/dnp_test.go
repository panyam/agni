package render

import (
	"strings"
	"testing"
)

// kicad-cli crosses out the fet cut's U61 and Q5, both (dnp yes), and leaves TP76 alone. Both
// backends draw the two diagonals per part, keyed to the part so a click on the cross selects it.
func TestEachBackendCrossesOutADoNotPopulatePart(t *testing.T) {
	g := readKiCadSheet(t, "orient-jetson-fet.kicad_sch")
	sheet := g.Sheets[0]

	crossed := map[string]int{}
	for _, line := range strings.Split(SheetSVG(g, sheet), "\n") {
		if !strings.Contains(line, `stroke="`+DefaultStyle.DNP+`"`) {
			continue
		}
		for _, ref := range []string{"U61", "Q5", "TP76"} {
			if strings.Contains(line, `data-ref="`+ref+`"`) {
				crossed[ref]++
			}
		}
	}
	if crossed["U61"] != 2 || crossed["Q5"] != 2 || crossed["TP76"] != 0 {
		t.Errorf("SVG crosses %v, want U61 and Q5 twice each and TP76 never", crossed)
	}

	ps := PackSheet(g, sheet)
	if len(ps.GroupColors) <= int(groupDNP) || ps.GroupColors[groupDNP] != DefaultStyle.DNP {
		t.Fatalf("packed palette %v has no DNP color at %d", ps.GroupColors, groupDNP)
	}
	refOf := map[uint32]string{}
	for _, k := range ps.Keys {
		refOf[k.Primitive] = k.RefDes
	}
	packed := map[string]int{}
	for i, r := range decodeRecords(ps.Primitives) {
		if r.group == groupDNP {
			packed[refOf[uint32(i)]]++
		}
	}
	if packed["U61"] != 2 || packed["Q5"] != 2 || len(packed) != 2 {
		t.Errorf("WebGL crosses %v, want U61 and Q5 twice each and nothing else", packed)
	}
}
