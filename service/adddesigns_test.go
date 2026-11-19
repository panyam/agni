package service_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func proposal(folder, name string) *webapi.ProposedDesign {
	return &webapi.ProposedDesign{Folder: folder, Design: &webapi.Design{Name: name}}
}

// Boards of the same names in two folders get ids that differ, so adding all of them doesn't
// collide (agni issue 1756).
func TestDefaultDesignIDsDifferForBoardsOfOneNameInTwoFolders(t *testing.T) {
	all := []*webapi.ProposedDesign{
		proposal("rev-a", "gateway"), proposal("rev-a", "spare"),
		proposal("rev-b", "gateway"), proposal("rev-b", "spare"),
		proposal("labs/solo", "solo"),
		proposal("", "root-board"),
	}
	want := []string{"rev-a-gateway", "rev-a-spare", "rev-b-gateway", "rev-b-spare", "solo", "root-board"}
	for i, p := range all {
		if got := service.DefaultDesignID(p, all); got != want[i] {
			t.Errorf("DefaultDesignID(%s, %s) = %q, want %q", p.GetFolder(), p.GetDesign().GetName(), got, want[i])
		}
	}
	releases := []*webapi.ProposedDesign{proposal("jetson-1.1.2", "jetson"), proposal("jetson-1.1.3", "jetson")}
	if a, b := service.DefaultDesignID(releases[0], releases), service.DefaultDesignID(releases[1], releases); a != "jetson-1.1.2" || b != "jetson-1.1.3" {
		t.Errorf("two releases in two folders = %q and %q; want each folder's name", a, b)
	}
}

// A dropped folder whose design.yaml names a symbol folder keeps that folder as an artifact of type
// ARTIFACT_TYPE_SYMBOLS, and the design's view names it under symbols: (agni issue 1756).
func TestImportDesignFilesKeepsANamedSymbolFolder(t *testing.T) {
	ctx := context.Background()
	svc, b, _ := designService(t)
	if _, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "bb"}); err != nil {
		t.Fatal(err)
	}
	folder := map[string]string{
		"design.yaml":      "entry: bb.kicad_sch\nsymbols: [lib]\n",
		"bb.kicad_sch":     "(kicad_sch)",
		"lib/resistor.sym": "v 20200319 2\n",
		"lib/sub/cap.sym":  "v 20200319 2\n",
		"notes.txt":        "unread",
	}
	resp, err := svc.ImportDesignFiles(ctx, &webapi.ImportDesignFilesRequest{Parent: "designs/bb", Files: b.files(folder)})
	if err != nil {
		t.Fatal(err)
	}
	var lib *webapi.Artifact
	for _, a := range resp.GetArtifacts() {
		if a.GetName() == "designs/bb/artifacts/symbols" {
			lib = a
		}
	}
	if lib == nil || lib.GetType() != webapi.ArtifactType_ARTIFACT_TYPE_SYMBOLS || len(lib.GetFiles()) != 2 {
		t.Fatalf("artifacts = %v; want a symbols artifact holding the two .sym files", resp.GetArtifacts())
	}
	if slices.ContainsFunc(resp.GetUnread(), func(u string) bool { return strings.HasPrefix(u, "lib/") }) {
		t.Errorf("unread = %v; want the symbol files kept, not reported unread", resp.GetUnread())
	}
	d, err := svc.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/bb"})
	if err != nil || len(d.GetConfig().GetSymbolPathUris()) != 1 || !strings.HasSuffix(d.GetConfig().GetSymbolPathUris()[0], "/symbols") {
		t.Errorf("designs/bb's symbol folders = %v, %v; want the symbols artifact's folder", d.GetConfig().GetSymbolPathUris(), err)
	}
}

func TestADesignsPacksAreKeptAndChecked(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := designService(t)
	d, err := svc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "bb", Design: &webapi.Design{Packs: []string{"packs/house"}}})
	if err != nil || !slices.Equal(d.GetPacks(), []string{"packs/house"}) {
		t.Fatalf("CreateDesign with packs = %v, %v", d.GetPacks(), err)
	}
	up := &webapi.UpdateDesignRequest{Design: &webapi.Design{Name: "designs/bb", Packs: []string{"packs/house", "packs/extra"}}, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"packs"}}}
	if d, err = svc.UpdateDesign(ctx, up); err != nil || !slices.Equal(d.GetPacks(), []string{"packs/house", "packs/extra"}) {
		t.Errorf("UpdateDesign packs = %v, %v", d.GetPacks(), err)
	}
	up.Design.Packs = []string{"house"}
	if _, err := svc.UpdateDesign(ctx, up); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("UpdateDesign with a bare pack id = %v, want ErrInvalidArgument", err)
	}
}
