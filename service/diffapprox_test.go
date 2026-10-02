package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestDiffDesignsPairsANearRenameWhenAsked covers agni issue 817. Revision C of the tutorial board
// renames PMIC_EN to PMIC_ENABLE and adds a pull-up to it, so the net is renamed and changed. A
// client asking with rename_approx gets the pairing and its evidence; without it, the default, the
// net reads as one deleted and one new, as it always has.
func TestDiffDesignsPairsANearRenameWhenAsked(t *testing.T) {
	svc := NewDiffService(fsQueryLoader{base: filepath.Join("..", "examples", "tutorial-project", "designs", "gateway")}, nil)
	ask := func(approx bool) []*webapi.DiffReport_NetChange {
		t.Helper()
		resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
			AUri: "mount://m/gateway-rev-b.edn", BUri: "mount://m/gateway-rev-c.edn", RenameApprox: approx,
		})
		if err != nil {
			t.Fatal(err)
		}
		return resp.GetReport().GetNets()
	}
	kinds := func(nets []*webapi.DiffReport_NetChange) map[string]string {
		out := map[string]string{}
		for _, n := range nets {
			out[n.GetName()] = n.GetKind()
		}
		return out
	}
	if got := kinds(ask(false)); got["PMIC_EN"] != "deleted" || got["PMIC_ENABLE"] != "new" {
		t.Errorf("without rename_approx the nets = %v, want PMIC_EN deleted and PMIC_ENABLE new", got)
	}
	var pair *webapi.DiffReport_NetChange
	for _, n := range ask(true) {
		if n.GetKind() == "renamed-approx" {
			pair = n
		}
		if n.GetKind() == "deleted" || n.GetKind() == "new" {
			t.Errorf("with rename_approx %s still reads as %s", n.GetName(), n.GetKind())
		}
	}
	if pair == nil {
		t.Fatal("with rename_approx there is no renamed-approx entry")
	}
	if pair.GetOldName() != "PMIC_EN" || pair.GetName() != "PMIC_ENABLE" || len(pair.GetAdded()) != 1 || pair.GetAdded()[0] != "R6.1" {
		t.Errorf("pairing = %s -> %s, added %v; want PMIC_EN -> PMIC_ENABLE, added [R6.1]", pair.GetOldName(), pair.GetName(), pair.GetAdded())
	}
	if pair.GetApprox().GetOldCoverageSignificant() != 1 {
		t.Errorf("evidence = %v, want every significant old endpoint kept", pair.GetApprox())
	}
}
