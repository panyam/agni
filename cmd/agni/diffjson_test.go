package main

import (
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

const tutorialGateway = "../../examples/tutorial-project/designs/gateway/"

func diffJSON(t *testing.T, args ...string) *webapi.DiffDesignsResponse {
	t.Helper()
	out := runCLI(t, diffCmd(), append([]string{"--format", "json"}, args...)...)
	var resp webapi.DiffDesignsResponse
	if err := protojson.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("not a DiffDesignsResponse: %v\n%s", err, out)
	}
	return &resp
}

// TestDiffJSONCarriesTheSheetMaps covers agni issue 737: `diff --format json` is the message the
// DiffDesigns rpc returns, sheet and placement maps included, because the command now calls the
// service rather than converting a diff of its own.
func TestDiffJSONCarriesTheSheetMaps(t *testing.T) {
	resp := diffJSON(t, tutorialGateway+"gateway.edn", tutorialGateway+"gateway-rev-b.edn")
	if len(resp.GetNetSheetsA()) == 0 || len(resp.GetComponentSheetsB()) == 0 {
		t.Errorf("net_sheets_a has %d entries and component_sheets_b %d, want both filled", len(resp.GetNetSheetsA()), len(resp.GetComponentSheetsB()))
	}
	if len(resp.GetSharedPlacementsA()) == 0 {
		t.Error("shared_placements_a is empty, want the placements both revisions share")
	}
}

// TestDiffJSONAsksTheServiceForNearRenames covers agni issue 817 from the CLI side: --rename-approx
// reaches the service as rename_approx, so the json carries the pairing.
func TestDiffJSONAsksTheServiceForNearRenames(t *testing.T) {
	count := func(resp *webapi.DiffDesignsResponse) int {
		n := 0
		for _, c := range resp.GetReport().GetNets() {
			if c.GetKind() == "renamed-approx" {
				n++
			}
		}
		return n
	}
	b, c := tutorialGateway+"gateway-rev-b.edn", tutorialGateway+"gateway-rev-c.edn"
	if n := count(diffJSON(t, b, c)); n != 0 {
		t.Errorf("without --rename-approx: %d renamed-approx entries, want 0", n)
	}
	if n := count(diffJSON(t, "--rename-approx", b, c)); n != 1 {
		t.Errorf("with --rename-approx: %d renamed-approx entries, want 1", n)
	}
}
