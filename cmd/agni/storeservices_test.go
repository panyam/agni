package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/internal/mountset"
	"github.com/panyam/agni/service"
)

// storeServer serves the project, design and snapshot services `agni serve --store --uploads`
// builds over dir/store and dir/uploads, with the tutorial project mounted beside them.
func storeServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	ms, err := withUploadsMount([]mountset.Mount{{Name: "tut", Root: "../../examples/tutorial-project"}}, filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	projectStore := projects.NewFSStore(projectTrees(ms)...)
	resolver := &service.ProjectResolver{Store: projectStore, Uploads: uploadedHost, UploadMaxBytes: 1 << 20}
	projectSvc, designSvc, _, err := storeServices(tableOf(ms), filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), projectStore, resolver, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.API{Workspace: service.NewWorkspaceService(osWorkspace(tableOf(ms))), Project: projectSvc, Design: designSvc, Snapshot: service.NewSnapshotService(nil, resolver, nil, "")}.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// A client makes a design, puts its netlist artifact, keeps it, groups it in a project, and finds both again after
// the server restarts over the same --store and --uploads (agni issues 1694, 1739).
func TestAServerKeepsADesignAndAProjectAcrossARestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srv := storeServer(t, dir)
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	projectsClient := webapiconnect.NewProjectServiceClient(http.DefaultClient, srv.URL)

	var files []*webapi.SnapshotFile
	var contents [][]byte
	for _, f := range []struct{ path, body string }{{"mem.edn", "(edif mem)"}, {"mem.kicad_pcb", "(kicad_pcb)"}} {
		sum := sha256.Sum256([]byte(f.body))
		files = append(files, &webapi.SnapshotFile{Path: f.path, Sha256: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(f.body))})
		contents = append(contents, []byte(f.body))
	}
	if _, err := designs.CreateDesign(ctx, connect.NewRequest(&webapi.CreateDesignRequest{DesignId: "memory_unit", Design: &webapi.Design{Title: "Memory unit", Series: "memory_unit"}})); err != nil {
		t.Fatal(err)
	}
	put := connect.NewRequest(&webapi.BatchUpdateArtifactsRequest{Parent: "designs/memory_unit", Requests: []*webapi.UpdateArtifactRequest{
		{Artifact: &webapi.Artifact{Name: "designs/memory_unit/artifacts/netlist", Files: files[:1]}, AllowMissing: true},
		{Artifact: &webapi.Artifact{Name: "designs/memory_unit/artifacts/board", Files: files[1:]}, AllowMissing: true},
	}})
	_, err := designs.BatchUpdateArtifacts(ctx, put)
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition || len(ce.Details()) != 1 {
		t.Fatalf("BatchUpdateArtifacts before the contents are sent = %v, want FAILED_PRECONDITION with a BlobsMissing detail", err)
	}
	if _, err := snaps.UploadBlobs(ctx, connect.NewRequest(&webapi.UploadBlobsRequest{Contents: contents})); err != nil {
		t.Fatal(err)
	}
	if _, err := designs.BatchUpdateArtifacts(ctx, put); err != nil {
		t.Fatal(err)
	}
	if _, err := projectsClient.CreateProject(ctx, connect.NewRequest(&webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Title: "B200", Designs: []string{"designs/memory_unit"}}})); err != nil {
		t.Fatal(err)
	}
	// The mounted folder's design.yaml makes no design (agni issue 1736), so there is no gateway to
	// write to.
	if _, err := designs.UpdateArtifact(ctx, connect.NewRequest(&webapi.UpdateArtifactRequest{Artifact: &webapi.Artifact{Name: "designs/gateway/artifacts/netlist", Files: files[:1]}, AllowMissing: true})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("UpdateArtifact of the mounted folder's gateway = %v, want NOT_FOUND", err)
	}

	again := storeServer(t, dir)
	designs = webapiconnect.NewDesignServiceClient(http.DefaultClient, again.URL)
	projectsClient = webapiconnect.NewProjectServiceClient(http.DefaultClient, again.URL)
	list, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{Filter: `project = "projects/b200"`}))
	if err != nil || len(list.Msg.GetDesigns()) != 1 || list.Msg.GetDesigns()[0].GetTitle() != "Memory unit" || !list.Msg.GetDesigns()[0].GetSeriesLatest() {
		t.Fatalf("b200's designs after a restart = %v, %v; want memory_unit, latest in its series", list.Msg.GetDesigns(), err)
	}
	all, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{}))
	if err != nil || len(all.Msg.GetDesigns()) != 1 || all.Msg.GetDesigns()[0].GetName() != "designs/memory_unit" {
		t.Errorf("every design after a restart = %v, %v; want memory_unit alone, since a mounted folder makes no design", all.Msg.GetDesigns(), err)
	}
	d, err := designs.GetDesign(ctx, connect.NewRequest(&webapi.GetDesignRequest{Name: "designs/memory_unit"}))
	if err != nil || !regexp.MustCompile(`^mount://uploads/memory_unit/[0-9a-f]{12}/netlist/mem.edn$`).MatchString(d.Msg.GetEntryUri()) || len(d.Msg.GetCompanionUris()) != 1 || len(d.Msg.GetArtifacts()) != 2 {
		t.Errorf("memory_unit after a restart = %v, %v; want its netlist as entry, its board as companion, and both artifacts", d, err)
	}
	p, err := projectsClient.GetProject(ctx, connect.NewRequest(&webapi.GetProjectRequest{Name: "projects/b200"}))
	if err != nil || p.Msg.GetTitle() != "B200" {
		t.Errorf("project b200 after a restart = %v, %v; want B200", p, err)
	}
	gw, err := projectsClient.GetProject(ctx, connect.NewRequest(&webapi.GetProjectRequest{Name: "projects/gateway"}))
	if err != nil || len(gw.Msg.GetDesigns()) != 2 || gw.Msg.GetDesigns()[0] != "designs/gateway" || gw.Msg.GetDesigns()[1] != "designs/gateway-rev-b" {
		t.Errorf("the mounted gateway project = %v, %v; want it to list designs/gateway and designs/gateway-rev-b", gw, err)
	}
}
