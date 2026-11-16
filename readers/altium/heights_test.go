package altium

import (
	"encoding/binary"
	"testing"
)

// body is one ComponentBodies6 record for part comp, with the given properties.
func body(comp uint16, props string) []byte {
	sub := make([]byte, 22)
	binary.LittleEndian.PutUint16(sub[7:], comp)
	binary.LittleEndian.PutUint32(sub[18:], uint32(len(props)))
	sub = append(sub, props...)
	rec := []byte{0x0C, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(rec[1:], uint32(len(sub)))
	return append(rec, sub...)
}

// A part takes its component record's HEIGHT, as Altium's ODB++ export does. A part whose HEIGHT is
// 0 or absent takes its tallest 3D body's OVERALLHEIGHT, and a body of no part is skipped (agni
// 1643).
func TestAPartTakesItsHeightThenItsTallestBody(t *testing.T) {
	stream := append(body(1, "KIND=0|OVERALLHEIGHT=20mil|STANDOFFHEIGHT=0mil"), body(1, "OVERALLHEIGHT=3.5mm")...)
	stream = append(stream, body(0xFFFF, "OVERALLHEIGHT=99mil")...)
	stream = append(stream, body(2, "OVERALLHEIGHT=10mil")...)
	p := &pcb{comps: []comp{
		{p: props{"HEIGHT": "345mil"}, height: -1},
		{p: props{"HEIGHT": "0mil"}, height: -1},
		{p: props{}, height: -1},
	}}
	err := p.readBodies(func(name string, _ bool) ([]byte, error) {
		if name == "ComponentBodies6" {
			return stream, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []int64{8_763_000, 3_500_000, 254_000} {
		h, ok := p.comps[i].partHeight()
		if !ok || h != want {
			t.Errorf("part %d height = %d (%v), want %d", i, h, ok, want)
		}
	}
	if _, ok := (comp{p: props{"HEIGHT": "0mil"}, height: -1}).partHeight(); ok {
		t.Error("a part with HEIGHT 0 and no body states a height")
	}
}
