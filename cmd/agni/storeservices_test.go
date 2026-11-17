package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// storeServer serves the project, design and snapshot services `agni serve` builds over dir, with
// the tutorial project mounted beside it.
func storeServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	ms, err := withStoreMount([]mounts.Mount{{Name: "tut", Root: "../../examples/tutorial-project"}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	projectStore := projects.NewFSStore(projectTrees(ms)...)
	resolver := &service.ProjectResolver{Store: projectStore, Uploads: uploadedHost, UploadMaxBytes: 1 << 20}
	projectSvc, designSvc, err := storeServices(ms, dir, projectStore, resolver)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.API{Project: projectSvc, Design: designSvc, Snapshot: service.NewSnapshotService(nil, resolver, nil, "")}.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// A client uploads a design's files, keeps it, groups it in a project, and finds both again after
// the server restarts over the same --store-dir (agni issue 1694).
func TestAServerKeepsADesignAndAProjectAcrossARestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srv := storeServer(t, dir)
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	projectsClient := webapiconnect.NewProjectServiceClient(http.DefaultClient, srv.URL)

	var files []*webapi.SnapshotFile
	var contents [][]byte
	for _, f := range []struct{ path, body string }{{"design.yaml", "title: Memory unit\nentry: mem.edn\nseries: memory_unit\n"}, {"mem.edn", "(edif mem)"}} {
		sum := sha256.Sum256([]byte(f.body))
		files = append(files, &webapi.SnapshotFile{Path: f.path, Sha256: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(f.body))})
		contents = append(contents, []byte(f.body))
	}
	_, err := designs.CreateDesign(ctx, connect.NewRequest(&webapi.CreateDesignRequest{DesignId: "memory_unit", Files: files}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeFailedPrecondition || len(ce.Details()) != 1 {
		t.Fatalf("CreateDesign before the contents are sent = %v, want FAILED_PRECONDITION with a BlobsMissing detail", err)
	}
	if _, err := snaps.UploadBlobs(ctx, connect.NewRequest(&webapi.UploadBlobsRequest{Contents: contents})); err != nil {
		t.Fatal(err)
	}
	if _, err := designs.CreateDesign(ctx, connect.NewRequest(&webapi.CreateDesignRequest{DesignId: "memory_unit", Files: files})); err != nil {
		t.Fatal(err)
	}
	if _, err := projectsClient.CreateProject(ctx, connect.NewRequest(&webapi.CreateProjectRequest{ProjectId: "b200", Project: &webapi.Project{Title: "B200", Designs: []string{"designs/memory_unit"}}})); err != nil {
		t.Fatal(err)
	}
	if _, err := designs.UpdateDesign(ctx, connect.NewRequest(&webapi.UpdateDesignRequest{Name: "designs/gateway", Files: files})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("UpdateDesign of the mounted gateway = %v, want FAILED_PRECONDITION", err)
	}

	again := storeServer(t, dir)
	designs = webapiconnect.NewDesignServiceClient(http.DefaultClient, again.URL)
	projectsClient = webapiconnect.NewProjectServiceClient(http.DefaultClient, again.URL)
	list, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{Filter: `project = "projects/b200"`}))
	if err != nil || len(list.Msg.GetDesigns()) != 1 || list.Msg.GetDesigns()[0].GetTitle() != "Memory unit" || !list.Msg.GetDesigns()[0].GetSeriesLatest() {
		t.Fatalf("b200's designs after a restart = %v, %v; want memory_unit, latest in its series", list.Msg.GetDesigns(), err)
	}
	all, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{}))
	if err != nil || len(all.Msg.GetDesigns()) != 3 {
		t.Errorf("every design after a restart = %v, %v; want the mounted gateway and gateway-rev-b, and memory_unit", all.Msg.GetDesigns(), err)
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
