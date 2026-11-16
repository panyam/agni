package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

const jetsonCutPCB = "../../examples/cross-probe-project/designs/jetson-cut/jetson-cut.kicad_pcb"

func runBoard(t *testing.T, args ...string) (string, error) {
	t.Helper()
	freshWorkspace(t)
	cmd := boardCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestBoardCountsTheCut(t *testing.T) {
	out, err := runBoard(t, jetsonCutPCB)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"layers            37", "parts             19", "tracks            397", "units             1 nm per unit"} {
		if !strings.Contains(out, want) {
			t.Errorf("agni board on the cut is missing %q:\n%s", want, out)
		}
	}
}

func TestBoardWritesTheSameMessageInJSONAndBinary(t *testing.T) {
	js, err := runBoard(t, jetsonCutPCB, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	fromJSON := &geom.BoardGeometry{}
	if err := protojson.Unmarshal([]byte(js), fromJSON); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "board.pb")
	if _, err := runBoard(t, jetsonCutPCB, "--format", "binary", "-o", file); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file) // #nosec G304 -- the test's own temp file
	if err != nil {
		t.Fatal(err)
	}
	fromBinary := &geom.BoardGeometry{}
	if err := proto.Unmarshal(raw, fromBinary); err != nil {
		t.Fatal(err)
	}
	if got := CountBoard(fromBinary); got != (BoardCounts{Layers: 37, Parts: 19, Nets: 15, Tracks: 397, Vias: 202}) {
		t.Errorf("the binary board counts %+v, want the cut's", got)
	}
	// Both forms go through a merge, which writes a back-side pad's -0 rotation as 0, before they
	// compare (see service/boardgeometry_test.go).
	a, b := &geom.BoardGeometry{}, &geom.BoardGeometry{}
	proto.Merge(a, fromJSON)
	proto.Merge(b, fromBinary)
	if !proto.Equal(a, b) {
		t.Error("the JSON and binary forms hold different boards")
	}
}

func TestBoardBinaryNeedsAFile(t *testing.T) {
	if _, err := runBoard(t, jetsonCutPCB, "--format", "binary"); err == nil || !strings.Contains(err.Error(), "-o") {
		t.Errorf("err = %v, want a refusal naming -o, since protobuf bytes would go to the terminal", err)
	}
}
