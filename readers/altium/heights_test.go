package altium

import (
	"encoding/binary"
	"math"
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

// A body's outline follows its properties as a vertex count and x, y doubles, and becomes the part's
// outline (agni 1668). A count under three draws nothing.
func TestABodyKeepsItsOutline(t *testing.T) {
	text := "OVERALLHEIGHT=10mil"
	rec := body(0, text)
	ring := []float64{0, 0, 1000, 0, 1000, 500}
	tail := make([]byte, 4+16*3)
	binary.LittleEndian.PutUint32(tail, 3)
	for i, v := range ring {
		binary.LittleEndian.PutUint64(tail[4+8*i:], math.Float64bits(v))
	}
	rec = append(rec, tail...)
	binary.LittleEndian.PutUint32(rec[1:], uint32(len(rec)-5))
	short := body(0, text)
	short = append(short, 2, 0, 0, 0)
	binary.LittleEndian.PutUint32(short[1:], uint32(len(short)-5))
	p := &pcb{comps: []comp{{p: props{}, height: -1}}}
	err := p.readBodies(func(name string, _ bool) ([]byte, error) {
		if name == "ComponentBodies6" {
			return append(rec, short...), nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := p.comps[0].bodies; len(got) != 1 || len(got[0]) != 3 || got[0][1] != [2]float64{1000, 0} {
		t.Errorf("bodies = %v, want the one three-point outline", got)
	}
}
