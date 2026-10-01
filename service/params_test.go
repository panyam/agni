package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/param"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	parampb "github.com/panyam/agni/gen/go/agni/v1/param"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// TestGetComponentParams checks that the RPC surfaces only components whose MPN joins to a seeded
// PartSpec, with that spec, and that a service built without a provider (serve without --params)
// returns no components, never an error.
func TestGetComponentParams(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{
		{RefDes: "U1", Mpn: "LM1117"},
		{RefDes: "R1"}, // no MPN -> no spec, excluded
	}}
	spec := &parampb.PartSpec{Mpn: "LM1117", Manufacturer: "TI", Parameters: []*parampb.Parameter{{Name: "VIN abs max"}}}
	provider := param.ProviderFunc(func(mpn string) *parampb.PartSpec {
		if mpn == "LM1117" {
			return spec
		}
		return nil
	})

	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), provider, "", nil, nil)
	resp, err := svc.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{Uri: "mount://m/d.pdf"})
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

	noParams := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), nil, "", nil, nil)
	empty, err := noParams.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{Uri: "mount://m/d.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.GetComponents()) != 0 {
		t.Errorf("nil provider must yield no components, got %d", len(empty.GetComponents()))
	}
}

// projectWithParams resolves every ref to one design in a project that declares its own params.
type projectWithParams struct{ ProjectStore }

func (projectWithParams) ResolveDesign(context.Context, artifact.URI) (*webapi.Design, *webapi.Project, error) {
	return &webapi.Design{Uri: "mount://m/d"}, &webapi.Project{Name: "projects/p", Config: &webapi.AnalysisConfig{ParamUris: []string{"mount://m/params"}}}, nil
}

// The params panel reads through the design's project, as a check does, so it shows the spec a verdict
// rests on: the project's for a part it seeds, the server's for the rest, each naming its corpus. It
// used the server's corpus alone, so inside a project it showed the wrong spec or none (agni 749).
func TestGetComponentParamsLayersTheProjectAndNamesTheCorpus(t *testing.T) {
	d := &ir.Design{Components: []*ir.Component{{RefDes: "U1", Mpn: "LDO"}, {RefDes: "U2", Mpn: "BUCK"}}}
	projectLDO := &parampb.PartSpec{Mpn: "LDO", Manufacturer: "project"}
	shared := param.ParamSet{"LDO": {Mpn: "LDO", Manufacturer: "shared"}, "BUCK": {Mpn: "BUCK", Manufacturer: "shared"}}
	projects := &ProjectResolver{Store: projectWithParams{}, Config: &recordingResolver{specs: param.ParamSet{"LDO": projectLDO}}}

	svc := NewCheckService(fakeLoader{design: d}, check.DefaultCatalog(), shared, "", nil, projects)
	resp, err := svc.GetComponentParams(context.Background(), &webapi.GetComponentParamsRequest{Uri: "mount://m/d/board.edn"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range resp.GetComponents() {
		got[c.GetRefDes()] = c.GetSpec().GetManufacturer() + "/" + c.GetCorpus()
	}
	want := map[string]string{"U1": "project/project", "U2": "shared/shared"}
	if len(got) != 2 || got["U1"] != want["U1"] || got["U2"] != want["U2"] {
		t.Errorf("panel = %v, want %v", got, want)
	}
}
