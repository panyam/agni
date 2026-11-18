package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/mounts"
	"github.com/panyam/agni/service"
)

// fakeCommit is the commit the fake GitHub resolves every known ref to.
const fakeCommit = "0123456789abcdef0123456789abcdef01234567"

// tarEntry is one entry of a fake tarball. A non-empty link makes it a symlink.
type tarEntry struct {
	name, body, link string
}

// fakeGitHub serves owner/repo the way GitHub's REST API does: the repo's default branch, a ref's
// commit, and the commit's tarball with every path under one top folder. Other paths are 404.
func fakeGitHub(t *testing.T, owner, repo string, entries []tarEntry) *httptest.Server {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	top := owner + "-" + repo + "-" + fakeCommit[:7] + "/"
	if err := tw.WriteHeader(&tar.Header{Name: top, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		h := &tar.Header{Name: top + e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: tar.TypeReg}
		if e.link != "" {
			h = &tar.Header{Name: top + e.name, Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: e.link}
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.link == "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	tarball := buf.Bytes()
	base := "/repos/" + owner + "/" + repo
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"default_branch":"main"}`)
	})
	mux.HandleFunc("GET "+base+"/commits/{ref}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("ref") {
		case "main", "1.1.2", fakeCommit:
			_, _ = io.WriteString(w, `{"sha":"`+fakeCommit+`"}`)
		default:
			http.Error(w, `{"message":"No commit found"}`, http.StatusUnprocessableEntity)
		}
	})
	// GitHub answers the tarball with a redirect to codeload, so the fake does too.
	mux.HandleFunc("GET "+base+"/tarball/"+fakeCommit, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/codeload/"+fakeCommit, http.StatusFound)
	})
	mux.HandleFunc("GET /codeload/"+fakeCommit, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tarball)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// jetsonTree is a small stand-in for a board repo, with a link and a climbing path GitHub would
// never send but a hostile tarball could.
var jetsonTree = []tarEntry{
	{name: "README.md", body: "# baseboard\n"},
	{name: "hw/board.kicad_sch", body: "(kicad_sch (version 20231120))\n"},
	{name: "hw/copy.kicad_sch", body: "(kicad_sch (version 20231120))\n"},
	{name: "hw/escape", link: "../../../etc/passwd"},
	{name: "../outside.txt", body: "no"},
}

// runtimeMounts is the parts of a server a runtime mount touches: the table, the maker, and the
// WorkspaceService over both, composed as runViewer composes them.
type runtimeMounts struct {
	table *mounts.Table
	maker *mountMaker
	ws    *service.WorkspaceService
}

func startRuntimeMounts(t *testing.T, store, uploads, api string, fixed ...mounts.Mount) runtimeMounts {
	t.Helper()
	table := mounts.NewTable(fixed...)
	maker, err := newMountMaker(table, filepath.Join(store, storeMountsDir), uploads, api)
	if err != nil {
		t.Fatal(err)
	}
	if err := maker.restore(io.Discard); err != nil {
		t.Fatal(err)
	}
	return runtimeMounts{table: table, maker: maker, ws: service.NewWorkspaceService(osWorkspace(table)).WithMountMaker(maker)}
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
	gh := fakeGitHub(t, "antmicro", "jetson-orin-baseboard", jetsonTree)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, store, uploads, gh.URL)
	m, err := s.ws.CreateMount(context.Background(), &webapi.CreateMountRequest{Name: "jetson", Source: "github://antmicro/jetson-orin-baseboard", Ref: "1.1.2"})
	if err != nil {
		t.Fatal(err)
	}
	if m.GetKind() != webapi.MountKind_MOUNT_KIND_GITHUB || m.GetCommit() != fakeCommit || m.GetRef() != "1.1.2" || m.GetFixed() || m.GetWritable() || m.GetUri() != "mount://jetson" {
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
	if err := os.RemoveAll(filepath.Join(uploads, mounts.SnapshotsDir)); err != nil {
		t.Fatal(err)
	}
	again := startRuntimeMounts(t, store, uploads, "http://127.0.0.1:1")
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
	if _, err := os.Stat(filepath.Join(uploads, mounts.SnapshotsDir, "jetson")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("DeleteMount left the view: %v", err)
	}
}

func TestCreateMountRefusesWhatItCannotServe(t *testing.T) {
	gh := fakeGitHub(t, "antmicro", "jetson-orin-baseboard", jetsonTree)
	dir := t.TempDir()
	fixed := mounts.Mount{Name: "boards", Root: t.TempDir()}
	s := startRuntimeMounts(t, filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), gh.URL, fixed)
	ctx := context.Background()
	for _, tc := range []struct {
		name, source, ref string
		want              error
	}{
		{"boards", "github://antmicro/jetson-orin-baseboard", "", service.ErrAlreadyExists},
		{"etc", "file:///etc", "", service.ErrInvalidArgument},
		{"s3", "s3://bucket/boards", "", service.ErrInvalidArgument},
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

	// A server with no --store and --uploads makes no mounts.
	plain := service.NewWorkspaceService(osWorkspace(mounts.NewTable(fixed)))
	if _, err := plain.CreateMount(ctx, &webapi.CreateMountRequest{Name: "x", Source: "github://antmicro/jetson-orin-baseboard"}); !errors.Is(err, service.ErrFailedPrecondition) {
		t.Errorf("CreateMount with no store = %v; want ErrFailedPrecondition", err)
	}
}

func TestASnapshotOverTheCapIsRefusedAndLeavesNoMount(t *testing.T) {
	gh := fakeGitHub(t, "antmicro", "jetson-orin-baseboard", jetsonTree)
	dir := t.TempDir()
	uploads := filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, filepath.Join(dir, "store"), uploads, gh.URL)
	s.maker.maxBytes = 20
	_, err := s.ws.CreateMount(context.Background(), &webapi.CreateMountRequest{Name: "jetson", Source: "github://antmicro/jetson-orin-baseboard"})
	if !errors.Is(err, service.ErrResourceExhausted) {
		t.Fatalf("CreateMount over the cap = %v; want ErrResourceExhausted", err)
	}
	if _, ok := s.table.Find("jetson"); ok {
		t.Fatal("a refused snapshot is still served")
	}
	views, _ := os.ReadDir(filepath.Join(uploads, mounts.SnapshotsDir, "jetson"))
	if len(views) != 0 {
		t.Fatalf("a refused snapshot left %d folders in the uploads", len(views))
	}
}

func TestTheUploadsSweepKeepsASnapshotsFiles(t *testing.T) {
	gh := fakeGitHub(t, "antmicro", "jetson-orin-baseboard", jetsonTree)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	s := startRuntimeMounts(t, store, uploads, gh.URL)
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
	if count != 2 {
		t.Fatalf("the snapshot stored %d blobs; want 2 (README and one schematic, held once)", count)
	}
	if _, _, _, err := storeServices(s.table, store, uploads, nil, &service.ProjectResolver{}, nil, io.Discard); err != nil {
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
	gh := fakeGitHub(t, "antmicro", "jetson-orin-baseboard", jetsonTree)
	dir := t.TempDir()
	store, uploads := filepath.Join(dir, "store"), filepath.Join(dir, "uploads")
	serve := func() string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		cmd := rootCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		done := make(chan error, 1)
		go func() {
			done <- runViewer(cmd, viewerOpts{listener: ln, storeDir: store, uploadsDir: uploads, githubAPI: gh.URL})
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
	if code, body := call(first, "CreateMount", `{"name":"jetson","source":"github://antmicro/jetson-orin-baseboard","ref":"1.1.2"}`); code != 200 || !strings.Contains(body, fakeCommit) {
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
