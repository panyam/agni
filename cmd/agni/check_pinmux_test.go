package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/panyam/agni/core/param"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/internal/yamlpb"
)

const pinMuxFixture = "testdata/pin-mux"

// pinMuxDesign lays the pin-functions design out in a temp dir with spec as its project's only
// datasheet entry, so one test can run the same map before and after a merge.
func pinMuxDesign(t *testing.T, spec []byte) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"demo_ble.edn", "design.yaml"} {
		b, err := os.ReadFile(filepath.Join(pinFunctionFixture, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "params"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "params", "kw47.textproto"), spec, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A mux table kept as its own file (agni issue 1200) reaches io-map-function once it is merged into the
// published spec. Before the merge the spec's pins list no alternates, so every declared function on
// U1 is not-considered; after it, the rows are the ones the hand-written spec gives.
func TestAMergedPinMuxReachesIOMapFunction(t *testing.T) {
	freshWorkspace(t)
	baseText, err := os.ReadFile(filepath.Join(pinMuxFixture, "kw47.textproto"))
	if err != nil {
		t.Fatal(err)
	}
	muxText, err := os.ReadFile(filepath.Join(pinMuxFixture, "kw47.pin_mux.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := param.Load(strings.NewReader(string(baseText)))
	if err != nil {
		t.Fatal(err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(muxText, &doc); err != nil {
		t.Fatal(err)
	}
	mux := &parampb.PartSpec{}
	if err := yamlpb.Decode(&doc, mux); err != nil {
		t.Fatal(err)
	}
	merged, err := param.MergePinMux(base, mux, param.PinMuxMerge{})
	if err != nil {
		t.Fatal(err)
	}
	mergedText, err := param.MarshalSpecText(merged)
	if err != nil {
		t.Fatal(err)
	}

	const functionCol = 10
	before := pinMapRows(t, pinMuxDesign(t, baseText), ioMapRules...)
	if len(before) != 4 {
		t.Fatalf("%d pin_map rows before the merge, want 4: %v", len(before), before)
	}
	for _, r := range before[:3] {
		if r[functionCol] != "not-considered" {
			t.Errorf("before the merge %s's function check is %q, want not-considered: %v", r[1], r[functionCol], r)
		}
	}

	after := pinMapRows(t, pinMuxDesign(t, mergedText), ioMapRules...)
	want := pinMapRows(t, pinFunctionFixture, ioMapRules...)
	if len(after) != 4 || len(want) != 4 {
		t.Fatalf("want 4 rows from each run, got %d merged and %d hand-written", len(after), len(want))
	}
	for i := range want {
		if strings.Join(after[i], ",") != strings.Join(want[i], ",") {
			t.Errorf("row %d:\n merged      %v\n hand-written %v", i, after[i], want[i])
		}
	}
	if after[2][1] != "PTB5" || after[2][functionCol] != "fail" {
		t.Errorf("SDA declared on PTB5, which offers SCL, must fail: %v", after[2])
	}
}
