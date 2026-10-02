package service

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/panyam/agni/core/diff"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestDiffDesignsPairsANearRenameWhenAsked covers agni issue 817. Revision C of the tutorial board
// renames PMIC_EN to PMIC_ENABLE and adds a pull-up to it, so the net is renamed and changed. A
// client asking with near_renames gets the pairing and its evidence; without it, the default, the
// net reads as one deleted and one new, as it always has.
func TestDiffDesignsPairsANearRenameWhenAsked(t *testing.T) {
	svc := NewDiffService(fsQueryLoader{base: filepath.Join("..", "examples", "tutorial-project", "designs", "gateway")}, nil)
	ask := func(near *webapi.NearRenameOptions) []*webapi.DiffReport_NetChange {
		t.Helper()
		resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
			AUri: "mount://m/gateway-rev-b.edn", BUri: "mount://m/gateway-rev-c.edn", NearRenames: near,
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
	if got := kinds(ask(nil)); got["PMIC_EN"] != "deleted" || got["PMIC_ENABLE"] != "new" {
		t.Errorf("without near_renames the nets = %v, want PMIC_EN deleted and PMIC_ENABLE new", got)
	}
	var pair *webapi.DiffReport_NetChange
	for _, n := range ask(&webapi.NearRenameOptions{}) {
		if n.GetKind() == "renamed-approx" {
			pair = n
		}
		if n.GetKind() == "deleted" || n.GetKind() == "new" {
			t.Errorf("with near_renames %s still reads as %s", n.GetName(), n.GetKind())
		}
	}
	if pair == nil {
		t.Fatal("with near_renames there is no renamed-approx entry")
	}
	if pair.GetOldName() != "PMIC_EN" || pair.GetName() != "PMIC_ENABLE" || len(pair.GetAdded()) != 1 || pair.GetAdded()[0] != "R6.1" {
		t.Errorf("pairing = %s -> %s, added %v; want PMIC_EN -> PMIC_ENABLE, added [R6.1]", pair.GetOldName(), pair.GetName(), pair.GetAdded())
	}
	if pair.GetApprox().GetOldCoverageSignificant() != 1 {
		t.Errorf("evidence = %v, want every significant old endpoint kept", pair.GetApprox())
	}
}

// A threshold set on the request overrides the calibrated one. PMIC_ENABLE's significant endpoints
// are two thirds old ones, so requiring nine tenths refuses the pairing the defaults make, and the
// nets read as deleted and new again.
func TestANearRenameThresholdOverridesTheDefault(t *testing.T) {
	svc := NewDiffService(fsQueryLoader{base: filepath.Join("..", "examples", "tutorial-project", "designs", "gateway")}, nil)
	strict := 0.9
	resp, err := svc.DiffDesigns(context.Background(), &webapi.DiffDesignsRequest{
		AUri: "mount://m/gateway-rev-b.edn", BUri: "mount://m/gateway-rev-c.edn",
		NearRenames: &webapi.NearRenameOptions{MinNewCoverageSignificant: &strict},
	})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, n := range resp.GetReport().GetNets() {
		kinds[n.GetName()] = n.GetKind()
	}
	if kinds["PMIC_ENABLE"] != "new" || kinds["PMIC_EN"] != "deleted" {
		t.Errorf("with min_new_coverage_significant 0.9 the nets = %v, want PMIC_EN deleted and PMIC_ENABLE new", kinds)
	}
}

func TestRenameOptionsFromProto(t *testing.T) {
	if RenameOptionsFromProto(nil).Enabled {
		t.Error("no options turned the pass on")
	}
	def := RenameOptionsFromProto(&webapi.NearRenameOptions{})
	want := diff.DefaultRenameOptions()
	want.Enabled = true
	if !reflect.DeepEqual(def, want) {
		t.Errorf("empty options = %+v, want the defaults with the pass on", def)
	}
	floor := int32(5)
	got := RenameOptionsFromProto(&webapi.NearRenameOptions{MaxAddedSignificantFloor: &floor, InsignificantClasses: []string{"test_point", "mounting_hole"}})
	if got.MaxAddedSignificantFloor != 5 || len(got.InsignificantClasses) != 2 || got.MinOldCoverage != want.MinOldCoverage {
		t.Errorf("overrides = %+v, want floor 5, two insignificant classes, other thresholds unchanged", got)
	}
}
