package main

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	geom "github.com/panyam/agni/gen/go/agni/v1/geom"
)

func runStackup(t *testing.T, args ...string) (string, error) {
	t.Helper()
	freshWorkspace(t)
	cmd := stackupCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestStackupPrintsTheStackTopToBottom(t *testing.T) {
	out, err := runStackup(t, "../../readers/kicad/testdata/board_stackup.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 15 {
		t.Fatalf("got %d lines, want a header, 12 rows, a blank and the total:\n%s", len(lines), out)
	}
	for i, want := range map[int]string{
		1:  "F.SilkS       silk        unknown",
		4:  "dielectric 1  dielectric  0.1 mm     PR1080    prepreg  3.96  0.02",
		7:  "dielectric 2  dielectric  1 mm       FR4       core     4.5   0.02",
		12: "B.SilkS       silk        unknown",
	} {
		if strings.TrimRight(lines[i], " ") != want {
			t.Errorf("line %d = %q, want %q", i, lines[i], want)
		}
	}
	if want := "The file states a board thickness of 1.6 mm. The layers sum to 1.546 mm."; lines[14] != want {
		t.Errorf("last line = %q, want %q", lines[14], want)
	}
}

func TestStackupJSONIsTheStackupMessage(t *testing.T) {
	out, err := runStackup(t, "--format", "json", "../../readers/kicad/testdata/board_stackup.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	var s geom.Stackup
	if err := protojson.Unmarshal([]byte(out), &s); err != nil {
		t.Fatalf("not a Stackup: %v\n%s", err, out)
	}
	if len(s.GetLayers()) != 12 || s.GetThickness() != 1_600_000 || s.GetLayers()[2].GetName() != "F.Cu" {
		t.Fatalf("Stackup = %v", &s)
	}
}

func TestStackupSaysWhenTheFileStatesNone(t *testing.T) {
	out, err := runStackup(t, "../../readers/kicad/testdata/board.kicad_pcb")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "The board file states no stack." {
		t.Fatalf("out = %q", out)
	}
}
