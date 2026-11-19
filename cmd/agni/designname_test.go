package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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

// designServer serves what `agni serve --store <store> --uploads <uploads>` serves to a client that
// checks, diffs and queries designs by name (agni issues 1695, 1739).
func designServer(t *testing.T, store, uploads string, mounted ...mountset.Mount) *httptest.Server {
	t.Helper()
	ms, err := withUploadsMount(mounted, uploads)
	if err != nil {
		t.Fatal(err)
	}
	loader := service.NewCachingLoader(&osLoader{mounts: tableOf(ms), loader: newLoader()}, service.NewDesignCache(0))
	projectStore := projects.NewFSStore(projectTrees(ms)...)
	resolver := &service.ProjectResolver{Store: projectStore, Config: &osProjectConfig{mounts: tableOf(ms)}, Uploads: uploadedHost, UploadMaxBytes: 1 << 24}
	snaps, err := newOSSnapshotStore(filepath.Join(store, storeSnapshotsDir))
	if err != nil {
		t.Fatal(err)
	}
	resolver.Snapshots = snaps
	checkSvc, reviewSvc, err := serveRuleServices(loader, nil, nil, "", nil, resolver, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	projectSvc, designSvc, packSvc, err := storeServices(tableOf(ms), store, uploads, projectStore, resolver, checkSvc, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server.API{
		Workspace: service.NewWorkspaceService(osWorkspace(tableOf(ms))).WithDesignFiles(resolver, osFiles(tableOf(ms))),
		Project:   projectSvc,
		Design:    designSvc,
		Pack:      packSvc,
		Snapshot:  checkSvc.Snapshots(),
		Check:     checkSvc,
		Diff:      service.NewDiffService(loader, resolver),
		Query:     service.NewQueryService(loader, nil, resolver),
		Review:    reviewSvc,
	}.Register(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// gatewayFolder copies the tutorial gateway's netlist and board into a folder of their own, the
// way a user holds a design they push.
func gatewayFolder(t *testing.T, netlist string) string {
	t.Helper()
	dir := t.TempDir()
	src := "../../examples/tutorial-project/designs/gateway-rev-b/"
	for from, to := range map[string]string{netlist: "gateway.edn", src + "gateway-rev-b.kicad_pcb": "gateway.kicad_pcb"} {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, to), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const (
	gatewayRevA = "../../examples/tutorial-project/designs/gateway/gateway.edn"
	gatewayRevB = "../../examples/tutorial-project/designs/gateway-rev-b/gateway-rev-b.edn"
)

func ingestByName(ctx context.Context, c webapiconnect.SnapshotServiceClient, design string) (*webapi.Snapshot, error) {
	resp, err := c.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{
		Snapshot:   &webapi.Snapshot{Design: design},
		SnapshotId: service.DesignSnapshotID(design),
	}))
	if err != nil {
		return nil, err
	}
	return resp.Msg, nil
}

// A stored design is ingested by its name, and ingesting it again under the id its name derives
// reuses the snapshot until the design's files change (agni issue 1695).
func TestASnapshotNamesItsDesign(t *testing.T) {
	ctx := context.Background()
	store, uploads := t.TempDir(), t.TempDir()
	srv := designServer(t, store, uploads)
	if out, err := runDesigns(t, srv.URL, "push", gatewayFolder(t, gatewayRevA), "--id", "gw"); err != nil {
		t.Fatalf("push: %v\n%s", err, out)
	}
	snaps := webapiconnect.NewSnapshotServiceClient(http.DefaultClient, srv.URL)
	first, err := ingestByName(ctx, snaps, "designs/gw")
	if err != nil {
		t.Fatal(err)
	}
	if first.GetName() != "snapshots/"+service.DesignSnapshotID("designs/gw") || !inUploads("gw", "netlist/gateway.edn").MatchString(first.GetTiers().GetNetlistUri()) || first.GetDesign() != "designs/gw" {
		t.Fatalf("ingest by name = %v; want the fixed id, the design named, and its netlist read from the store", first)
	}
	again, err := ingestByName(ctx, snaps, "designs/gw")
	if err != nil || again.GetRevisionId() != first.GetRevisionId() {
		t.Errorf("a second ingest = %v, %v; want the same snapshot and revision", again, err)
	}
	for name, def := range map[string]*webapi.Snapshot{
		"an unknown design": {Design: "designs/none"},
		"design and uri":    {Design: "designs/gw", Uri: first.GetTiers().GetNetlistUri()},
	} {
		_, err := snaps.Ingest(ctx, connect.NewRequest(&webapi.IngestRequest{Snapshot: def}))
		want := connect.CodeNotFound
		if name == "design and uri" {
			want = connect.CodeInvalidArgument
		}
		if connect.CodeOf(err) != want {
			t.Errorf("ingest of %s = %v, want %v", name, err, want)
		}
	}
	if out, err := runDesigns(t, srv.URL, "create", "empty"); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	if _, err := ingestByName(ctx, snaps, "designs/empty"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("ingest of a design with no artifact a reader opens = %v, want FAILED_PRECONDITION", err)
	}

	// Replacing the netlist moves the snapshot on at the next ingest under the same id.
	if out, err := runDesigns(t, srv.URL, "put", "gw", "netlist", gatewayRevB); err != nil {
		t.Fatalf("put: %v\n%s", err, out)
	}
	moved, err := ingestByName(ctx, snaps, "designs/gw")
	if err != nil {
		t.Fatal(err)
	}
	if moved.GetName() != first.GetName() || moved.GetRevisionId() == first.GetRevisionId() || !inUploads("gw", "netlist/gateway-rev-b.edn").MatchString(moved.GetTiers().GetNetlistUri()) {
		t.Errorf("ingest after replacing the netlist = %v; want the same snapshot reading gateway-rev-b.edn at a new revision", moved)
	}

	// After a restart the snapshot is rebuilt from its kept definition, by looking the name up again.
	again2 := designServer(t, store, uploads)
	checks := webapiconnect.NewCheckServiceClient(http.DefaultClient, again2.URL)
	resp, err := checks.CheckDesign(ctx, connect.NewRequest(&webapi.CheckDesignRequest{Snapshot: first.GetName()}))
	if err != nil || len(resp.Msg.GetFindings()) == 0 {
		t.Errorf("a check on the snapshot after a restart = %v, %v; want findings from the rebuilt snapshot", resp, err)
	}
}

// agni check, diff and query take a stored design's name under --remote, and answer as they do on
// the store's own folder read locally.
func TestTheCLIAsksAboutAStoredDesignByName(t *testing.T) {
	uploads := t.TempDir()
	srv := designServer(t, t.TempDir(), uploads)
	for id, netlist := range map[string]string{"gw": gatewayRevA, "gw-b": gatewayRevB} {
		if out, err := runDesigns(t, srv.URL, "push", gatewayFolder(t, netlist), "--id", id); err != nil {
			t.Fatalf("push %s: %v\n%s", id, err, out)
		}
	}
	count := func(args ...string) int {
		t.Helper()
		out, err := runAgni(t, args...)
		if err != nil {
			t.Fatalf("agni %v: %v\n%s", args, err, out)
		}
		var doc struct {
			Findings        []json.RawMessage `json:"findings"`
			ComponentStatus map[string]string `json:"componentStatus"`
			NetStatus       map[string]string `json:"netStatus"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("agni %v printed no JSON: %v\n%s", args, err, out)
		}
		changed := 0
		for _, st := range [](map[string]string){doc.ComponentStatus, doc.NetStatus} {
			for _, v := range st {
				if v != "equal" && v != "" {
					changed++
				}
			}
		}
		return len(doc.Findings) + changed
	}
	folder := currentView(t, uploads, "gw")
	byName := count("check", "designs/gw", "--remote", srv.URL, "--format", "json")
	local := count("check", folder, "--format", "json")
	if byName == 0 || byName != local {
		t.Errorf("check designs/gw --remote = %d findings, check on its folder = %d; want the same, above zero", byName, local)
	}
	diffName := count("diff", "designs/gw", "designs/gw-b", "--remote", srv.URL, "--format", "json")
	diffLocal := count("diff", folder, currentView(t, uploads, "gw-b"), "--format", "json")
	if diffName == 0 || diffName != diffLocal {
		t.Errorf("diff by name = %d changes, diff on the folders = %d; want the same, above zero", diffName, diffLocal)
	}
	out, err := runAgni(t, "query", "designs/gw", "component.class(?C, ?K)", "--remote", srv.URL)
	if err != nil || out == "" {
		t.Errorf("query designs/gw --remote = %q, %v; want rows", out, err)
	}
	if _, err := runAgni(t, "check", "designs/none", "--remote", srv.URL); err == nil {
		t.Error("check of a design the server lacks succeeded; want its not-found error")
	}
}

// inUploads matches a file's URI in a stored design's current view.
func inUploads(id, rel string) *regexp.Regexp {
	return regexp.MustCompile(`^mount://uploads/` + regexp.QuoteMeta(id) + `/[0-9a-f]{12}/` + regexp.QuoteMeta(rel) + `$`)
}

// currentView is the one view folder a stored design has, which the CLI reads as a plain design.
func currentView(t *testing.T, uploads, id string) string {
	t.Helper()
	revs, err := filepath.Glob(filepath.Join(projects.ViewsRoot(uploads), id, "*"))
	if err != nil || len(revs) != 1 {
		t.Fatalf("views of %s = %v, %v; want one", id, revs, err)
	}
	return revs[0]
}

// runAgni runs the whole CLI with args and returns what it printed to stdout.
func runAgni(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := rootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}
