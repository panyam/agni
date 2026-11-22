package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/internal/server"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
)

// storeServer serves the project, design and snapshot services `agni serve --store --uploads`
// builds over dir/store and dir/uploads, with the tutorial project mounted beside them.
func storeServer(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	ms, err := withUploadsMount([]mounts.Mount{mountset.Folder("tut", "../../examples/tutorial-project")}, filepath.Join(dir, "uploads"))
	if err != nil {
		t.Fatal(err)
	}
	projectStore := projects.NewFSStore(projectTrees(ms)...)
	resolver := &service.ProjectResolver{Store: projectStore, Uploads: uploadedHost, UploadMaxBytes: 1 << 20}
	projectSvc, designSvc, _, err := storeServices(t.Context(), tableOf(ms), testStore(t, filepath.Join(dir, "store")), filepath.Join(dir, "uploads"), projectStore, resolver, nil, nil, io.Discard)
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
	// No design was added from the tutorial's folder, so the mounted project lists none, rather than
	// the names its folders' design.yaml files extend it with (agni issue 1756).
	if err != nil || len(gw.Msg.GetDesigns()) != 0 {
		t.Errorf("the mounted gateway project = %v, %v; want it to list no designs, since none was added from its folder", gw.Msg.GetDesigns(), err)
	}
}

// A mounted project lists the stored designs added from its folder, and only those (agni issue
// 1756).
func TestAMountedProjectListsTheDesignsAddedFromItsFolder(t *testing.T) {
	ctx := context.Background()
	srv := storeServer(t, t.TempDir())
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	projectsClient := webapiconnect.NewProjectServiceClient(http.DefaultClient, srv.URL)
	if _, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://tut/designs", Designs: []*webapi.DesignToAdd{{Folder: "designs/gateway", DesignId: "gw"}}})); err != nil {
		t.Fatal(err)
	}
	if _, err := designs.CreateDesign(ctx, connect.NewRequest(&webapi.CreateDesignRequest{DesignId: "elsewhere"})); err != nil {
		t.Fatal(err)
	}
	gw, err := projectsClient.GetProject(ctx, connect.NewRequest(&webapi.GetProjectRequest{Name: "projects/gateway"}))
	if err != nil || !slices.Equal(gw.Msg.GetDesigns(), []string{"designs/gw"}) {
		t.Errorf("the mounted gateway project lists %v, %v; want the design added from its folder alone", gw.Msg.GetDesigns(), err)
	}
	list, err := projectsClient.ListProjects(ctx, connect.NewRequest(&webapi.ListProjectsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list.Msg.GetProjects() {
		if p.GetName() == "projects/gateway" && !slices.Equal(p.GetDesigns(), []string{"designs/gw"}) {
			t.Errorf("ListProjects shows the gateway project with %v; want designs/gw", p.GetDesigns())
		}
	}
}

// A running server sweeps --uploads on a timer as well as at startup, so a replaced artifact's
// files go without a restart (agni issue 1742).
func TestARunningServerSweepsAReplacedArtifactsFiles(t *testing.T) {
	old := sweepInterval
	sweepInterval = 20 * time.Millisecond
	t.Cleanup(func() { sweepInterval = old })
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	ms, err := withUploadsMount(nil, uploads)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &service.ProjectResolver{Uploads: uploadedHost, UploadMaxBytes: 1 << 20}
	_, designSvc, _, err := storeServices(t.Context(), tableOf(ms), testStore(t, store), uploads, nil, resolver, nil, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := designSvc.CreateDesign(ctx, &webapi.CreateDesignRequest{DesignId: "mem"}); err != nil {
		t.Fatal(err)
	}
	put := func(body string) string {
		t.Helper()
		h, err := resolver.Blobs.Put([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := designSvc.UpdateArtifact(ctx, &webapi.UpdateArtifactRequest{AllowMissing: true,
			Artifact: &webapi.Artifact{Name: "designs/mem/artifacts/netlist", Files: []*webapi.SnapshotFile{{Path: "mem.edn", Sha256: h, Size: int64(len(body))}}}}); err != nil {
			t.Fatal(err)
		}
		return h
	}
	first := put("(edif mem 1)")
	second := put("(edif mem 2)")
	blobPath := func(h string) string {
		hex := strings.TrimPrefix(h, "sha256:")
		return filepath.Join(uploads, "blobs", "sha256", hex[:2], hex[2:])
	}
	// Age everything past the grace, as a server up for a day sees it. The sweeper is already
	// running, so it can remove a file aged earlier in this walk before the walk reaches it.
	aged := time.Now().Add(-48 * time.Hour)
	if err := filepath.WalkDir(uploads, func(p string, _ os.DirEntry, err error) error {
		if err == nil {
			err = os.Chtimes(p, aged, aged)
		}
		if _, gone := os.Stat(p); errors.Is(gone, os.ErrNotExist) {
			return nil
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := os.Stat(blobPath(first))
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the replaced artifact's blob %s is still there after the server ran a while", first)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(blobPath(second)); err != nil {
		t.Errorf("the sweep removed the current artifact's blob: %v", err)
	}
	views, _ := os.ReadDir(filepath.Join(uploads, "designs", "mem"))
	if len(views) != 1 {
		t.Errorf("the design keeps %d views after the sweep; want only the current one", len(views))
	}
}
