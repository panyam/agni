package main

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

const jetsonCut = "../../examples/cross-probe-project/designs/jetson-cut"

func TestDiffOfOneDesignComparesItsViews(t *testing.T) {
	freshWorkspace(t)
	out := runCLI(t, diffCmd(), jetsonCut)
	for _, want := range []string{
		"Parts: 19 of 19 schematic parts are on the board\n",
		"Pins:  51 of 51 pins on a schematic net are on a net on the board\n",
		"Nets:  17 of 17 schematic nets match a board net\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestDiffOfOneDesignAsJSONIsTheResponse(t *testing.T) {
	freshWorkspace(t)
	out := runCLI(t, diffCmd(), "--format", "json", jetsonCut)
	var resp webapi.DiffDesignsResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not a DiffDesignsResponse: %v\n%s", err, out)
	}
	v := resp.GetViews()
	if !v.GetCompared() || v.GetParts().GetMatched() != 19 || v.GetPins().GetMatched() != 51 || v.GetNets().GetMatched() != 17 {
		t.Errorf("views = %v, want the counts the text prints", v)
	}
}

func TestDiffOfASchematicAloneIsNotCompared(t *testing.T) {
	freshWorkspace(t)
	prev := readAsNamed
	readAsNamed = true
	t.Cleanup(func() { readAsNamed = prev })
	out := runCLI(t, diffCmd(), jetsonCut+"/jetson-cut.kicad_sch")
	if !strings.Contains(out, "Not compared: the design has no board to compare its schematic with.\n") || strings.Contains(out, "Parts:") {
		t.Errorf("a schematic read alone printed:\n%s", out)
	}
}

func TestDiffOfOneDesignRefusesTheRevisionFlags(t *testing.T) {
	freshWorkspace(t)
	for _, args := range [][]string{{"--format", "csv"}, {"--include-equal"}, {"--rename-approx"}} {
		cmd := diffCmd()
		cmd.SetArgs(append(args, jetsonCut))
		if err := cmd.Execute(); err == nil {
			t.Errorf("diff %v of one design answered, want it refused", args)
		}
	}
}
