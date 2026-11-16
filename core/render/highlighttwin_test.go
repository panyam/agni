package render

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

// highlightTwin is the fixture web/src/highlights.test.ts replays (C27). keys are the packed
// board's PrimitiveKeys with each primitive's packed group, as
// [primitive, group, ref_des, net, net_id, pin, bus_id, placement]. Each case is a spec and the
// primitives HighlightPacked lights for it, sorted.
type highlightTwin struct {
	Sheet string          `json:"sheet"`
	Keys  [][]any         `json:"keys"`
	Cases []highlightCase `json:"cases"`
}

type highlightCase struct {
	Name       string          `json:"name"`
	Spec       json.RawMessage `json:"spec"`
	Primitives []uint32        `json:"primitives"`
}

// TestHighlightTwin writes what HighlightPacked lights on the Jetson cut's packed board, for a part
// with a pick area, a pin, a busy net, a net id and a spec naming nothing, and holds the committed
// file to it (agni issue 1635). The viewer resolves the same specs against the same keys, so its
// resolveHighlights cannot drift from HighlightPacked. `go test ./core/render -run
// TestHighlightTwin -update` rewrites it.
func TestHighlightTwin(t *testing.T) {
	sheet := PackBoard(jetsonCutBoard(t))
	keys := PackedKeys(sheet)
	if len(keys) < 100 {
		t.Fatalf("the cut's packed board has %d keys, so the twin checks almost nothing", len(keys))
	}
	twin := highlightTwin{Sheet: "the Jetson cut's packed board (examples/cross-probe-project)"}
	group := func(p uint32) int {
		i := int(p)*primRecordBytes + 1
		if i >= len(sheet.GetPrimitives()) {
			return 0
		}
		return int(sheet.GetPrimitives()[i])
	}
	nets := map[string]int{}
	var pin *geom.PrimitiveKey
	netID, picked := "", ""
	for _, k := range keys {
		twin.Keys = append(twin.Keys, []any{k.GetPrimitive(), group(k.GetPrimitive()), k.GetRefDes(), k.GetNet(), k.GetNetId(), k.GetPin(), k.GetBusId(), k.GetPlacement()})
		if k.GetNet() != "" {
			nets[k.GetNet()]++
		}
		if pin == nil && k.GetPin() != "" {
			pin = k
		}
		if netID == "" && k.GetNetId() != "" {
			netID = k.GetNetId()
		}
		if picked == "" && k.GetRefDes() != "" && pickOnly(sheet, k.GetPrimitive()) {
			picked = k.GetRefDes()
		}
	}
	busiest := ""
	for n, c := range nets {
		if c > nets[busiest] || (c == nets[busiest] && n < busiest) {
			busiest = n
		}
	}
	if picked == "" || pin == nil || busiest == "" {
		t.Fatalf("the cut's board lacks a part with a pick area (%q), a pin (%v) or a net (%q)", picked, pin, busiest)
	}
	specs := []struct {
		name string
		spec *geom.HighlightSpec
	}{
		{"a part, without its pick area", &geom.HighlightSpec{Components: []string{picked}}},
		{"one pin of a part", &geom.HighlightSpec{Pins: []*geom.PinRef{{RefDes: pin.GetRefDes(), Pin: pin.GetPin()}}}},
		{"the busiest net", &geom.HighlightSpec{Nets: []string{busiest}}},
		{"a part and a net together", &geom.HighlightSpec{Components: []string{picked}, Nets: []string{busiest}}},
		{"a name the board lacks", &geom.HighlightSpec{Components: []string{"NOPE"}, Nets: []string{"NOPE"}, BusIds: []string{"NOPE"}}},
	}
	if netID != "" {
		specs = append(specs, struct {
			name string
			spec *geom.HighlightSpec
		}{"a net by its id alone", &geom.HighlightSpec{NetIds: []string{netID}}})
	}
	for _, s := range specs {
		got := HighlightPacked(sheet, []*geom.HighlightSpec{s.spec}).GetGroups()[0].GetPrimitives()
		prims := append([]uint32{}, got...)
		sort.Slice(prims, func(i, j int) bool { return prims[i] < prims[j] })
		spec, err := protojson.Marshal(s.spec)
		if err != nil {
			t.Fatal(err)
		}
		twin.Cases = append(twin.Cases, highlightCase{Name: s.name, Spec: spec, Primitives: prims})
	}
	out, err := twinLines(twin)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "highlight_twin.json")
	if *update {
		if err := os.WriteFile(path, out, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path) // #nosec G304 -- the package's own testdata
	if err != nil {
		t.Fatalf("%v (run with -update to write it)", err)
	}
	if !bytes.Equal(have, out) {
		t.Errorf("%s is not what HighlightPacked lights; run `go test ./core/render -run TestHighlightTwin -update`", path)
	}
}

// twinLines writes the twin with one key and one case per line, so a change to what a highlight
// lights reads as a short diff.
func twinLines(tw highlightTwin) ([]byte, error) {
	var out bytes.Buffer
	sheet, err := json.Marshal(tw.Sheet)
	if err != nil {
		return nil, err
	}
	out.WriteString("{\"sheet\":" + string(sheet) + ",\n\"keys\":[\n")
	for i, k := range tw.Keys {
		line, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		out.Write(line)
		if i < len(tw.Keys)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("],\n\"cases\":[\n")
	for i, c := range tw.Cases {
		line, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, line); err != nil {
			return nil, err
		}
		out.Write(compact.Bytes())
		if i < len(tw.Cases)-1 {
			out.WriteByte(',')
		}
		out.WriteByte('\n')
	}
	out.WriteString("]}\n")
	return out.Bytes(), nil
}
