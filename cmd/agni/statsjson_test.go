package main

import (
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

func statsJSONOf(t *testing.T, args ...string) *webapi.GetDesignResponse {
	t.Helper()
	out := runCLI(t, statsCmd(), append([]string{tutorialGateway + "gateway.edn", "--format", "json"}, args...)...)
	var resp webapi.GetDesignResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not a GetDesignResponse: %v\n%s", err, out)
	}
	return &resp
}

// TestStatsJSONIsGetDesign covers agni issue 836 from the CLI: `stats --format json` is the message
// GetDesign returns, the summary alone by default and the IR when --mask asks for it.
func TestStatsJSONIsGetDesign(t *testing.T) {
	sum := statsJSONOf(t)
	if sum.GetComponentCount() == 0 || sum.GetDesign() != nil {
		t.Errorf("default = %d components, design %v; want the summary and no IR", sum.GetComponentCount(), sum.GetDesign() != nil)
	}
	ir := statsJSONOf(t, "--mask", "design.nets", "--net", "PMIC_EN")
	nets := ir.GetDesign().GetNets()
	if len(nets) != 1 || nets[0].GetName() != "PMIC_EN" || len(nets[0].GetConnections()) == 0 {
		t.Errorf("--mask design.nets --net PMIC_EN gave %v, want that one net with its connections", nets)
	}
	if len(ir.GetDesign().GetComponents()) != 0 {
		t.Error("components came back for a mask that named only nets")
	}
}

func TestStatsRefusesAMaskOnText(t *testing.T) {
	cmd := statsCmd()
	cmd.SetArgs([]string{tutorialGateway + "gateway.edn", "--mask", "design"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--format json") {
		t.Errorf("err = %v, want the mask refused without --format json", err)
	}
}
