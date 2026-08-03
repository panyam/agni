package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/core/check"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/datasheet/param"
)

// TestGetComponentParams: the RPC surfaces only components whose MPN joins to a seeded PartSpec, with
// that spec; and a service built without a provider (serve without --params) returns no components,
// never an error.
func TestGetComponentParams(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Attributes: map[string]string{"MPN": "LM1117"}},
		{RefDes: "R1"}, // no MPN -> no spec, excluded
	}}
	spec := &parampb.PartSpec{Mpn: "LM1117", Manufacturer: "TI", Parameters: []*parampb.Parameter{{Name: "VIN abs max"}}}
	provider := param.ProviderFunc(func(mpn string) *parampb.PartSpec {
		if mpn == "LM1117" {
			return spec
		}
		return nil
	})

	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), provider)
	resp, err := svc.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetComponents()) != 1 {
		t.Fatalf("components with a joined spec = %d, want 1 (only U1)", len(resp.GetComponents()))
	}
	cp := resp.GetComponents()[0]
	if cp.GetRefDes() != "U1" || cp.GetMpn() != "LM1117" {
		t.Errorf("component = {ref:%q mpn:%q}, want U1 / LM1117", cp.GetRefDes(), cp.GetMpn())
	}
	if got := cp.GetSpec().GetParameters(); len(got) != 1 || got[0].GetName() != "VIN abs max" {
		t.Errorf("spec parameters not carried through: %v", got)
	}

	noParams := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), nil)
	empty, err := noParams.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.GetComponents()) != 0 {
		t.Errorf("nil provider must yield no components, got %d", len(empty.GetComponents()))
	}
}
