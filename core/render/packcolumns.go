package render

import (
	"encoding/binary"

	"github.com/panyam/agni/core/num"
	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// A packed sheet sends its keys and labels as columns rather than one message each (agni issue
// 1532). Jetson's board holds 37,320 keys and 5,732 labels, and a browser took about 35 ms to
// decode them as messages. As a string table and fixed-width records they decode in about 1 ms.
// web/src/packed.ts decodes the same layout, and testdata/packed_columns_twin.binpb holds one sheet
// that both sides' tests decode.

const (
	keyRecordWords   = 7  // primitive, ref_des, net, net_id, pin, bus_id, placement
	labelRecordWords = 11 // x, y, height, rotation_deg, max_width, width, text, justify, color, group, mirror
)

// stringTable interns strings for a column message. Index 0 is always "".
type stringTable struct {
	index   map[string]uint32
	strings []string
}

func newStringTable() *stringTable {
	return &stringTable{index: map[string]uint32{"": 0}, strings: []string{""}}
}

func (t *stringTable) of(s string) uint32 {
	if i, ok := t.index[s]; ok {
		return i
	}
	i := num.Uint32(len(t.strings))
	t.index[s] = i
	t.strings = append(t.strings, s)
	return i
}

// packKeys writes keys as PackedKeys columns, in the order given.
func packKeys(keys []*geom.PrimitiveKey) *geom.PackedKeys {
	t := newStringTable()
	rec := make([]byte, 0, len(keys)*keyRecordWords*4)
	for _, k := range keys {
		for _, w := range [keyRecordWords]uint32{k.GetPrimitive(), t.of(k.GetRefDes()), t.of(k.GetNet()), t.of(k.GetNetId()), t.of(k.GetPin()), t.of(k.GetBusId()), k.GetPlacement()} {
			rec = binary.LittleEndian.AppendUint32(rec, w)
		}
	}
	return &geom.PackedKeys{Strings: t.strings, Records: rec}
}

// packLabels writes labels as PackedLabels columns, in the order given.
func packLabels(labels []*geom.PackedLabel) *geom.PackedLabels {
	t := newStringTable()
	out := &geom.PackedLabels{}
	rec := make([]byte, 0, len(labels)*labelRecordWords*4)
	for i, l := range labels {
		mirror := uint32(0)
		if l.GetMirror() {
			mirror = 1
		}
		// The signed fields go out as their int32's two's-complement bits, which a reader decodes as
		// an int32.
		for _, w := range [labelRecordWords]uint32{
			uint32(l.GetX()), uint32(l.GetY()), uint32(l.GetHeight()), uint32(l.GetRotationDeg()), // #nosec G115 -- two's-complement bits
			uint32(l.GetMaxWidth()), uint32(l.GetWidth()), // #nosec G115 -- two's-complement bits
			t.of(l.GetText()), t.of(l.GetJustify()), t.of(l.GetColor()), l.GetGroup(), mirror,
		} {
			rec = binary.LittleEndian.AppendUint32(rec, w)
		}
		if len(l.GetRuns()) > 0 {
			out.Runs = append(out.Runs, &geom.PackedLabelRuns{Label: num.Uint32(i), Runs: l.GetRuns()})
		}
	}
	out.Strings, out.Records = t.strings, rec
	return out
}

// PackedKeys decodes a packed sheet's key columns into one PrimitiveKey per keyed primitive, in
// primitive order. A sheet with no key columns has no keys. A string index past the table reads as
// "", and a trailing partial record is ignored, so a malformed sheet loses keys rather than panics.
func PackedKeys(sheet *geom.PackedSheet) []*geom.PrimitiveKey {
	c := sheet.GetKeyColumns()
	strs, rec := c.GetStrings(), c.GetRecords()
	str := func(i uint32) string {
		if int(i) < len(strs) {
			return strs[i]
		}
		return ""
	}
	n := len(rec) / (keyRecordWords * 4)
	out := make([]*geom.PrimitiveKey, n)
	for i := range out {
		w := func(j int) uint32 { return binary.LittleEndian.Uint32(rec[(i*keyRecordWords+j)*4:]) }
		out[i] = &geom.PrimitiveKey{Primitive: w(0), RefDes: str(w(1)), Net: str(w(2)), NetId: str(w(3)), Pin: str(w(4)), BusId: str(w(5)), Placement: w(6)}
	}
	return out
}

// PackedLabels decodes a packed sheet's label columns into one PackedLabel per text, in draw order,
// with each label's markup runs. It reads a malformed sheet as PackedKeys does.
func PackedLabels(sheet *geom.PackedSheet) []*geom.PackedLabel {
	c := sheet.GetLabelColumns()
	strs, rec := c.GetStrings(), c.GetRecords()
	str := func(i uint32) string {
		if int(i) < len(strs) {
			return strs[i]
		}
		return ""
	}
	n := len(rec) / (labelRecordWords * 4)
	out := make([]*geom.PackedLabel, n)
	for i := range out {
		w := func(j int) uint32 { return binary.LittleEndian.Uint32(rec[(i*labelRecordWords+j)*4:]) }
		s := func(j int) int32 { return int32(w(j)) } // #nosec G115 -- the int32's two's-complement bits
		out[i] = &geom.PackedLabel{
			X: s(0), Y: s(1), Height: s(2), RotationDeg: s(3), MaxWidth: s(4), Width: s(5),
			Text: str(w(6)), Justify: str(w(7)), Color: str(w(8)), Group: w(9), Mirror: w(10) != 0,
		}
	}
	for _, r := range c.GetRuns() {
		if int(r.GetLabel()) < n {
			out[r.GetLabel()].Runs = r.GetRuns()
		}
	}
	return out
}
