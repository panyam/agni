package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/panyam/agni/core/render"
	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// The tests here cover agni issue 836: GetDesign carries the design's IR when a read_mask asks for
// it, only the parts asked for, and the summary it always had when none does.

func irDesign() *ir.Design {
	return &ir.Design{
		Name: "BOARD", SourceFormat: "edif-2.0.0",
		Components: []*ir.Component{
			{RefDes: "R1", Mpn: "RES-10K", Attributes: map[string]string{"Value": "10k"}},
			{RefDes: "U1", Mpn: "MCU-1", Attributes: map[string]string{"Value": "MCU"}},
		},
		Nets: []*ir.Net{
			{Name: "SDA", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "1"}, {ComponentRef: "U1", PinRef: "5"}}},
			{Name: "VCC", Connections: []*ir.Connection{{ComponentRef: "R1", PinRef: "2"}}},
		},
	}
}

func askDesign(t *testing.T, l fakeLoader, req *webapi.GetDesignRequest) (*webapi.GetDesignResponse, error) {
	t.Helper()
	req.Uri = "mount://m/x.edn"
	return NewDesignService(l, noNative{}, render.Style{}, nil).GetDesign(context.Background(), req)
}

func mask(paths ...string) *fieldmaskpb.FieldMask { return &fieldmaskpb.FieldMask{Paths: paths} }

func TestGetDesignWithNoMaskIsTheSummaryAlone(t *testing.T) {
	resp, err := askDesign(t, fakeLoader{design: irDesign(), geom: twoSheetGeom()}, &webapi.GetDesignRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetDesign() != nil {
		t.Error("an unmasked GetDesign carried the IR, which the viewer would pay for on every open")
	}
	if resp.GetComponentCount() != 2 || len(resp.GetSheets()) == 0 {
		t.Errorf("summary = %d components, %d sheets; want the summary as before", resp.GetComponentCount(), len(resp.GetSheets()))
	}
}

// A mask entirely under `design` reads the netlist alone. The loader here fails any geometry read,
// so the call succeeding is the proof.
func TestAMaskUnderDesignSkipsTheDrawing(t *testing.T) {
	resp, err := askDesign(t, fakeLoader{design: irDesign(), geomErr: errors.New("geometry read")}, &webapi.GetDesignRequest{ReadMask: mask("design")})
	if err != nil {
		t.Fatalf("an IR-only mask read the drawing: %v", err)
	}
	if len(resp.GetDesign().GetNets()) != 2 || len(resp.GetDesign().GetComponents()) != 2 {
		t.Errorf("design = %v, want the whole IR", resp.GetDesign())
	}
	if resp.GetComponentCount() != 0 || len(resp.GetSheets()) != 0 {
		t.Error("summary fields came back for a mask that did not ask for them")
	}
}

// A path through a repeated field applies to every element and keeps only the fields named.
func TestAMaskKeepsOnlyTheFieldsNamed(t *testing.T) {
	resp, err := askDesign(t, fakeLoader{design: irDesign()}, &webapi.GetDesignRequest{ReadMask: mask("design.components.ref_des", "design.components.mpn")})
	if err != nil {
		t.Fatal(err)
	}
	cs := resp.GetDesign().GetComponents()
	if len(cs) != 2 || cs[0].GetRefDes() != "R1" || cs[0].GetMpn() != "RES-10K" {
		t.Fatalf("components = %v, want R1 and U1 with their part numbers", cs)
	}
	if len(cs[0].GetAttributes()) != 0 || len(resp.GetDesign().GetNets()) != 0 || resp.GetDesign().GetName() != "" {
		t.Errorf("fields outside the mask survived: %v", resp.GetDesign())
	}
}

func TestAStarMaskIsEverything(t *testing.T) {
	resp, err := askDesign(t, fakeLoader{design: irDesign(), geom: twoSheetGeom()}, &webapi.GetDesignRequest{ReadMask: mask("*")})
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetComponentCount() != 2 || len(resp.GetDesign().GetNets()) != 2 || resp.GetDesign().GetComponents()[0].GetAttributes()["Value"] != "10k" {
		t.Errorf("* = %v, want the summary and the whole IR", resp)
	}
}

// The filters narrow which entities the IR carries, each its own list. Asking twice proves the
// narrowing works on a copy: the second, unfiltered answer still has every net.
func TestFiltersNarrowACopyOfTheIR(t *testing.T) {
	l := fakeLoader{design: irDesign()}
	resp, err := askDesign(t, l, &webapi.GetDesignRequest{ReadMask: mask("design"), Nets: []string{"SDA", "NOPE"}, RefDes: []string{"U1"}})
	if err != nil {
		t.Fatal(err)
	}
	if n := resp.GetDesign().GetNets(); len(n) != 1 || n[0].GetName() != "SDA" {
		t.Errorf("nets = %v, want SDA alone (NOPE is not in the design)", n)
	}
	if c := resp.GetDesign().GetComponents(); len(c) != 1 || c[0].GetRefDes() != "U1" {
		t.Errorf("components = %v, want U1 alone", c)
	}
	again, err := askDesign(t, l, &webapi.GetDesignRequest{ReadMask: mask("design.nets")})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.GetDesign().GetNets()) != 2 || len(l.design.GetNets()) != 2 {
		t.Error("filtering changed the loader's design, so a later read saw the narrowed IR")
	}
}

func TestAnUnknownMaskPathIsInvalid(t *testing.T) {
	for _, p := range []string{"design.no_such_field", "nope", "design.name.deeper", "design.*.name"} {
		_, err := askDesign(t, fakeLoader{design: irDesign()}, &webapi.GetDesignRequest{ReadMask: mask(p)})
		if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), p) {
			t.Errorf("path %q: err = %v, want an invalid argument naming it", p, err)
		}
	}
}
