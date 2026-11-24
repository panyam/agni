package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"
)

// jetsonRepo is goapplib's fake GitHub holding antmicro/jetson-orin-baseboard: a README and two
// schematics on main, and a link that leads out. It returns the fake and the commit main is at. A
// tarball path that climbs out of the repo fails the whole snapshot in goapplib, whose own tests
// hold that.
func jetsonRepo(t *testing.T) (*githubtest.Server, string) {
	t.Helper()
	gh := githubtest.New(t)
	repo := gh.AddRepo("antmicro", "jetson-orin-baseboard", false, map[string]string{
		"README.md":          "# baseboard\n",
		"hw/board.kicad_sch": "(kicad_sch (version 20231120))\n",
		"hw/copy.kicad_sch":  "(kicad_sch (version 20231120))\n",
	})
	return gh, repo.Symlink("main", "hw/escape", "../../../etc/passwd")
}

// runtimeMounts is the parts of a server a runtime mount touches: the table and the
// WorkspaceService over it, composed as runViewer composes them.
type runtimeMounts struct {
	table *mountset.Table
	ws    *service.WorkspaceService
}

func startRuntimeMounts(t *testing.T, store, uploads, api string, limits mounts.Limits, fixed ...mounts.Mount) runtimeMounts {
	t.Helper()
	table, err := mountset.Open(mountset.Config{Fixed: fixed, Uploads: uploads, Records: filepath.Join(store, storeMountsDir), GitHubAPI: api, Limits: limits, Log: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	return runtimeMounts{table: table, ws: service.NewWorkspaceService(osWorkspace(table)).WithMountMaker(table)}
}

func listNames(t *testing.T, ws *service.WorkspaceService, uri string) []string {
	t.Helper()
	resp, err := ws.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uri})
	if err != nil {
		t.Fatalf("ListDir %s: %v", uri, err)
	}
	var names []string
	for _, e := range resp.GetEntries() {
		names = append(names, e.GetName())
	}
	return names
}

func TestAGitHubMountIsServedAfterARestart(t *testing.T) {
	gh, fakeCommit := jetsonRepo(t)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, store, uploads, gh.URL, mounts.Limits{})
	m, err := s.ws.CreateMount(context.Background(), &webapi.CreateMountRequest{Name: "jetson", Source: "github://antmicro/jetson-orin-baseboard", Ref: fakeCommit})
	if err != nil {
		t.Fatal(err)
	}
	if m.GetKind() != webapi.MountKind_MOUNT_KIND_GITHUB || m.GetCommit() != fakeCommit || m.GetRef() != fakeCommit || m.GetFixed() || m.GetWritable() || m.GetUri() != "mount://jetson" {
		t.Fatalf("CreateMount = %v; want a GitHub mount at %s, not fixed, not writable", m, fakeCommit)
	}
	if got := strings.Join(listNames(t, s.ws, "mount://jetson/hw"), ","); got != "board.kicad_sch,copy.kicad_sch" {
		t.Fatalf("mount://jetson/hw lists %q; want the two schematics and no link", got)
	}
	// The store holds the record and no file bytes.
	if err := filepath.WalkDir(store, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasSuffix(p, ".yaml") {
			t.Errorf("the store holds %s, which is not a record", p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(uploads, "outside.txt")); err == nil {
		t.Fatal("a tarball path with .. was written outside the view")
	}

	// A restart reads the record back. The view is removed first, so the restart has to rebuild it
	// from the blobs.
	if err := os.RemoveAll(filepath.Join(uploads, mountset.SnapshotsDir)); err != nil {
		t.Fatal(err)
	}
	again := startRuntimeMounts(t, store, uploads, "http://127.0.0.1:1", mounts.Limits{})
	got, err := again.ws.GetMount(context.Background(), &webapi.GetMountRequest{Name: "jetson"})
	if err != nil || got.GetCommit() != fakeCommit || got.GetSource() != "github://antmicro/jetson-orin-baseboard" {
		t.Fatalf("after a restart, GetMount = %v, %v; want the GitHub mount at %s", got, err, fakeCommit)
	}
	if names := listNames(t, again.ws, "mount://jetson/hw"); len(names) != 2 {
		t.Fatalf("after a restart, mount://jetson/hw lists %v; want the two schematics", names)
	}
	b, err := os.ReadFile(filepath.Join(got.GetRoot(), "README.md"))
	if err != nil || string(b) != "# baseboard\n" {
		t.Fatalf("the rebuilt view's README = %q, %v", b, err)
	}

	if _, err := again.ws.DeleteMount(context.Background(), &webapi.DeleteMountRequest{Name: "jetson"}); err != nil {
		t.Fatal(err)
	}
	if _, err := again.ws.GetMount(context.Background(), &webapi.GetMountRequest{Name: "jetson"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("GetMount after DeleteMount = %v; want ErrNotFound", err)
	}
	if _, err := os.Stat(filepath.Join(uploads, mountset.SnapshotsDir, "jetson")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("DeleteMount left the view: %v", err)
	}
}

func TestCreateMountRefusesWhatItCannotServe(t *testing.T) {
	gh, _ := jetsonRepo(t)
	dir := t.TempDir()
	fixed := mountset.Folder("boards", t.TempDir())
	s := startRuntimeMounts(t, filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), gh.URL, mounts.Limits{}, fixed)
	ctx := context.Background()
	for _, tc := range []struct {
		name, source, ref string
		want              error
	}{
		{"boards", "github://antmicro/jetson-orin-baseboard", "", service.ErrAlreadyExists},
		{"etc", "file:///etc", "", service.ErrInvalidArgument},
		// A bucket or a drive mounts now (agni issues 1729, 1730), and these are refused before any
		// request leaves the server.
		{"ftp", "ftp://host/boards", "", service.ErrInvalidArgument},
		{"s3", "s3://bucket/boards", "main", service.ErrInvalidArgument},
		{"sp", "msgraph://drives/d1/root/hw", "", service.ErrInvalidArgument},
		{"Bad Name", "github://antmicro/jetson-orin-baseboard", "", service.ErrInvalidArgument},
		{"x", "github://antmicro/../etc", "", service.ErrInvalidArgument},
		{"x", "github://antmicro/no-such-repo", "", service.ErrNotFound},
		{"x", "github://antmicro/jetson-orin-baseboard", "no-such-tag", service.ErrNotFound},
	} {
		_, err := s.ws.CreateMount(ctx, &webapi.CreateMountRequest{Name: tc.name, Source: tc.source, Ref: tc.ref})
		if !errors.Is(err, tc.want) {
			t.Errorf("CreateMount(%q, %q, %q) = %v; want %v", tc.name, tc.source, tc.ref, err, tc.want)
		}
	}
	if _, err := s.ws.DeleteMount(ctx, &webapi.DeleteMountRequest{Name: "boards"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("DeleteMount of a --mount = %v; want ErrFailedPrecondition", err)
	}
	if _, err := s.ws.DeleteMount(ctx, &webapi.DeleteMountRequest{Name: "nothing"}); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("DeleteMount of an unknown name = %v; want ErrNotFound", err)
	}
	if names := s.table.List(); len(names) != 1 {
		t.Errorf("after the refusals the table serves %v; want the one fixed mount", names)
	}

	// A server with no --store and --uploads makes no mounts, and says so in ListMounts so a client
	// offers no form (agni issue 1749).
	plain := service.NewWorkspaceService(osWorkspace(mountset.NewTable(fixed)))
	for _, tc := range []struct {
		ws   *service.WorkspaceService
		want bool
	}{{s.ws, true}, {plain, false}} {
		if resp, err := tc.ws.ListMounts(ctx, &webapi.ListMountsRequest{}); err != nil || resp.GetCanCreateMounts() != tc.want {
			t.Errorf("ListMounts.can_create_mounts = %v, %v; want %v", resp.GetCanCreateMounts(), err, tc.want)
		}
	}
	if _, err := plain.CreateMount(ctx, &webapi.CreateMountRequest{Name: "x", Source: "github://antmicro/jetson-orin-baseboard"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateMount with no store = %v; want ErrFailedPrecondition", err)
	}
}

func TestASnapshotOverTheCapIsRefusedAndLeavesNoMount(t *testing.T) {
	gh, _ := jetsonRepo(t)
	dir := t.TempDir()
	uploads := filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, filepath.Join(dir, "store"), uploads, gh.URL, mounts.Limits{MaxBytes: 20})
	_, err := s.ws.CreateMount(context.Background(), &webapi.CreateMountRequest{Name: "jetson", Source: "github://antmicro/jetson-orin-baseboard"})
	if !errors.Is(err, service.ErrResourceExhausted) {
		t.Fatalf("CreateMount over the cap = %v; want ErrResourceExhausted", err)
	}
	if _, ok := s.table.Find("jetson"); ok {
		t.Fatal("a refused snapshot is still served")
	}
	views, _ := os.ReadDir(filepath.Join(uploads, mountset.SnapshotsDir, "jetson"))
	if len(views) != 0 {
		t.Fatalf("a refused snapshot left %d folders in the uploads", len(views))
	}
}

func TestTheUploadsSweepKeepsASnapshotsFiles(t *testing.T) {
	gh, _ := jetsonRepo(t)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, store, uploads, gh.URL, mounts.Limits{})
	if _, err := s.ws.CreateMount(context.Background(), &webapi.CreateMountRequest{Name: "jetson", Source: "github://antmicro/jetson-orin-baseboard"}); err != nil {
		t.Fatal(err)
	}
	// Age every blob past the sweep's grace, as a restart a day later sees them.
	old := time.Now().Add(-48 * time.Hour)
	blobs := filepath.Join(uploads, "blobs")
	var count int
	if err := filepath.WalkDir(blobs, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			count++
			return os.Chtimes(p, old, old)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("the snapshot stored %d blobs; want 3 (README, one schematic held once, and the manifest)", count)
	}
	if _, _, _, err := storeServices(t.Context(), spawnFor(t), s.table, testStore(t, store), uploads, nil, nil, &service.ProjectResolver{}, nil, nil, io.Discard); err != nil {
		t.Fatal(err)
	}
	var left int
	_ = filepath.WalkDir(blobs, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			left++
		}
		return nil
	})
	if left != count {
		t.Fatalf("the sweep left %d of the snapshot's %d blobs", left, count)
	}
}

// TestServeMakesAGitHubMountOverConnect runs the served wiring: runViewer with --store, --uploads
// and --github-api, and the Connect routes a client calls. A second runViewer over the same folders
// is the restart.
func TestServeMakesAGitHubMountOverConnect(t *testing.T) {
	gh, fakeCommit := jetsonRepo(t)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	serve := func() string {
		return serveViewer(t, viewerOpts{store: testStore(t, store), uploadsDir: uploads, githubAPI: gh.URL})
	}
	call := func(base, rpc, body string) (int, string) {
		resp, err := http.Post(base+"/agni.v1.webapi.WorkspaceService/"+rpc, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	first := serve()
	if code, body := call(first, "CreateMount", `{"name":"jetson","source":"github://antmicro/jetson-orin-baseboard","ref":"`+fakeCommit+`"}`); code != 200 || !strings.Contains(body, fakeCommit) {
		t.Fatalf("CreateMount = %d %s", code, body)
	}
	if code, body := call(first, "ListDir", `{"uri":"mount://jetson/hw"}`); code != 200 || !strings.Contains(body, "board.kicad_sch") {
		t.Fatalf("ListDir = %d %s", code, body)
	}

	second := serve()
	code, body := call(second, "ListMounts", `{}`)
	if code != 200 || !strings.Contains(body, `"jetson"`) || !strings.Contains(body, "MOUNT_KIND_GITHUB") || !strings.Contains(body, `"uploads"`) {
		t.Fatalf("after a restart, ListMounts = %d %s; want jetson beside the uploads mount", code, body)
	}
	if code, body := call(second, "DeleteMount", `{"name":"uploads"}`); code != 400 || !strings.Contains(body, "failed_precondition") {
		t.Fatalf("DeleteMount of the uploads mount = %d %s; want failed_precondition", code, body)
	}
}

// serveViewer runs runViewer with o on a free port until the test ends, and returns its base URL
// once /healthz answers. A runViewer that stops before answering fails the test with its error.
func serveViewer(t *testing.T, o viewerOpts) string {
	t.Helper()
	// A test that must know the server's URL first, such as for a redirect URI it registers
	// elsewhere, opens the listener itself.
	ln := o.listener
	if ln == nil {
		var err error
		if ln, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			t.Fatal(err)
		}
		o.listener = ln
	}
	cmd := rootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() {
		done <- runViewer(cmd, o)
	}()
	t.Cleanup(func() { ln.Close(); <-done })
	base := "http://" + ln.Addr().String()
	client := http.Client{Timeout: time.Second}
	for range 200 {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("runViewer stopped before answering: %v", err)
		default:
		}
		if resp, err := client.Get(base + "/healthz"); err == nil {
			resp.Body.Close()
			return base
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s never answered", base)
	return ""
}
