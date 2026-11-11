package render

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/panyam/agni/readers/kicad"
)

// columnFixture sets every PrimitiveKey and PackedLabel field to a distinct non-zero value on more
// than one element, with negative coordinates, a shared string, and runs on two labels, so a field
// the columns drop fails the round trip (C26).
func columnFixture() *geom.PackedSheet {
	return &geom.PackedSheet{
		SheetId:       "twin",
		LayoutVersion: packLayoutVersion,
		KeyColumns: packKeys([]*geom.PrimitiveKey{
			{Primitive: 3, RefDes: "U1", Net: "SDA", NetId: "n-7", Pin: "4", BusId: "D[0..7]", Placement: 2},
			{Primitive: 9, RefDes: "R12", Net: "SDA", NetId: "n-8", Pin: "1", BusId: "A[0..3]", Placement: 5},
			{Primitive: 11, Net: "GND"},
		}),
		LabelColumns: packLabels([]*geom.PackedLabel{
			{X: -120, Y: 340, Text: "U1", Height: 900, RotationDeg: 90, Justify: "left bottom", Color: "#112233", MaxWidth: 4000, Group: 11, Mirror: true, Width: 2500,
				Runs: []*geom.TextRun{{Text: "U", Kind: geom.TextRun_KIND_OVERBAR}, {Text: "1"}}},
			{X: 50, Y: -60, Text: "~RESET", Height: 700, RotationDeg: 270, Justify: "center center", Color: "#445566", MaxWidth: 1, Group: 3, Mirror: true, Width: 1,
				Runs: []*geom.TextRun{{Text: "RESET", Kind: geom.TextRun_KIND_SUBSCRIPT}}},
			{X: 7, Y: 8, Text: "U1", Height: 1},
		}),
	}
}

// fixtureKeys and fixtureLabels are the values columnFixture packs, decoded the long way, which a
// broken decoder cannot have produced.
func fixtureKeys() []*geom.PrimitiveKey {
	return []*geom.PrimitiveKey{
		{Primitive: 3, RefDes: "U1", Net: "SDA", NetId: "n-7", Pin: "4", BusId: "D[0..7]", Placement: 2},
		{Primitive: 9, RefDes: "R12", Net: "SDA", NetId: "n-8", Pin: "1", BusId: "A[0..3]", Placement: 5},
		{Primitive: 11, Net: "GND"},
	}
}

func TestPackedKeysRoundTrip(t *testing.T) {
	got := PackedKeys(columnFixture())
	want := fixtureKeys()
	if len(got) != len(want) {
		t.Fatalf("keys = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !proto.Equal(got[i], want[i]) {
			t.Errorf("key %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestPackedLabelsRoundTrip(t *testing.T) {
	in := []*geom.PackedLabel{
		{X: -120, Y: 340, Text: "U1", Height: 900, RotationDeg: 90, Justify: "left bottom", Color: "#112233", MaxWidth: 4000, Group: 11, Mirror: true, Width: 2500,
			Runs: []*geom.TextRun{{Text: "U", Kind: geom.TextRun_KIND_OVERBAR}, {Text: "1"}}},
		{X: 50, Y: -60, Text: "~RESET", Height: 700, RotationDeg: 270, Justify: "center center", Color: "#445566", MaxWidth: 1, Group: 3, Mirror: true, Width: 1,
			Runs: []*geom.TextRun{{Text: "RESET", Kind: geom.TextRun_KIND_SUBSCRIPT}}},
		{X: 7, Y: 8, Text: "U1", Height: 1},
	}
	got := PackedLabels(&geom.PackedSheet{LabelColumns: packLabels(in)})
	if len(got) != len(in) {
		t.Fatalf("labels = %d, want %d", len(got), len(in))
	}
	for i := range in {
		if !proto.Equal(got[i], in[i]) {
			t.Errorf("label %d = %v, want %v", i, got[i], in[i])
		}
	}
}

// TestColumnFixtureSetsEveryField is the census beside the round trips. A field added to
// PrimitiveKey or PackedLabel fails here until the columns carry it and the fixture sets it.
func TestColumnFixtureSetsEveryField(t *testing.T) {
	sheet := columnFixture()
	census := func(name string, msgs []protoreflect.Message) {
		fields := msgs[0].Descriptor().Fields()
		for i := 0; i < fields.Len(); i++ {
			f := fields.Get(i)
			set := 0
			for _, m := range msgs {
				if m.Has(f) {
					set++
				}
			}
			if set < 2 {
				t.Errorf("%s.%s is set on %d fixture elements, want at least 2", name, f.Name(), set)
			}
		}
	}
	var keys, labels []protoreflect.Message
	for _, k := range PackedKeys(sheet) {
		keys = append(keys, k.ProtoReflect())
	}
	for _, l := range PackedLabels(sheet) {
		labels = append(labels, l.ProtoReflect())
	}
	census("PrimitiveKey", keys)
	census("PackedLabel", labels)
}

// TestPackedColumnsTwin regenerates the twin fixture that web/src/packed.test.ts decodes and holds
// the committed files to it (C27). The .binpb is the packed sheet, and the two JSON files are what
// it decodes to, which both suites assert. `go test ./core/render -run TestPackedColumnsTwin
// -update` rewrites them.
func TestPackedColumnsTwin(t *testing.T) {
	sheet := columnFixture()
	bin, err := proto.MarshalOptions{Deterministic: true}.Marshal(sheet)
	if err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join("testdata", "packed_columns_twin.binpb")
	keysPath := filepath.Join("testdata", "packed_columns_twin_keys.json")
	labelsPath := filepath.Join("testdata", "packed_columns_twin_labels.json")
	keys, labels := PackedKeys(sheet), PackedLabels(sheet)
	if *update {
		writeJSONList(t, keysPath, keys)
		writeJSONList(t, labelsPath, labels)
		if err := os.WriteFile(binPath, bin, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if !bytes.Equal(have, bin) {
		t.Errorf("%s is not what the packer writes; run `go test ./core/render -run TestPackedColumnsTwin -update`", binPath)
	}
	wantKeys := readJSONList(t, keysPath, func() *geom.PrimitiveKey { return &geom.PrimitiveKey{} })
	wantLabels := readJSONList(t, labelsPath, func() *geom.PackedLabel { return &geom.PackedLabel{} })
	if len(wantKeys) != len(keys) || len(wantLabels) != len(labels) {
		t.Fatalf("twin lists %d keys and %d labels, the packer %d and %d", len(wantKeys), len(wantLabels), len(keys), len(labels))
	}
	for i := range keys {
		if !proto.Equal(keys[i], wantKeys[i]) {
			t.Errorf("twin key %d = %v, the packer decodes %v", i, wantKeys[i], keys[i])
		}
	}
	for i := range labels {
		if !proto.Equal(labels[i], wantLabels[i]) {
			t.Errorf("twin label %d = %v, the packer decodes %v", i, wantLabels[i], labels[i])
		}
	}
}

// writeJSONList writes messages as a JSON array, one protojson object per element.
func writeJSONList[M proto.Message](t *testing.T, path string, ms []M) {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, m := range ms {
		j, err := protojson.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("  ")
		b.Write(j)
		if i < len(ms)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readJSONList reads a JSON array of protojson objects.
func readJSONList[M proto.Message](t *testing.T, path string, mk func() M) []M {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := make([]M, len(raw))
	for i, r := range raw {
		out[i] = mk()
		if err := protojson.Unmarshal(r, out[i]); err != nil {
			t.Fatalf("%s[%d]: %v", path, i, err)
		}
	}
	return out
}

// TestJetsonKeysAndLabelsTravelSmall packs Jetson's board and holds its key and label columns to
// their size and string count (agni issue 1532). As messages they were 1.19 MB. The columns come to
// 1.35 MB, since a fixed word takes more room than a varint, and they decode about 30 times faster.
func TestJetsonKeysAndLabelsTravelSmall(t *testing.T) {
	path := "../../tools/samples/boards/jetson-agx-thor-baseboard/jetson-agx-thor-baseboard.kicad_pcb"
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("%v (make samples-oracle fetches the boards)", err)
	}
	defer f.Close()
	b, err := kicad.ReadBoardGeometry(f, filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	p := PackBoard(b)
	keys, labels := PackedKeys(p), PackedLabels(p)
	if len(keys) < 30_000 || len(labels) < 5_000 {
		t.Fatalf("Jetson packed %d keys and %d labels, want at least 30,000 and 5,000", len(keys), len(labels))
	}
	size := proto.Size(p.GetKeyColumns()) + proto.Size(p.GetLabelColumns())
	if size > 1_400_000 {
		t.Errorf("key and label columns are %d bytes, want at most 1.4 MB", size)
	}
	if n := len(p.GetKeyColumns().GetStrings()) + len(p.GetLabelColumns().GetStrings()); n > 12_000 {
		t.Errorf("the columns hold %d strings, want at most 12,000, one per distinct name", n)
	}
}
