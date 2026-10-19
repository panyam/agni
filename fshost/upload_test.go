package fshost

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// The tests here are agni issue 1061's step 4b: a design a client sends as files, named by a
// manifest of paths and hashes with the contents uploaded apart.

// uploadFixture is a host serving the tutorial project as mount m, that also takes uploads, counting
// the upload hosts it builds.
type uploadFixture struct {
	res    *service.ProjectResolver
	snaps  *service.SnapshotService
	check  *service.CheckService
	query  *service.QueryService
	builds atomic.Int64
}

func newUploadFixture(t *testing.T, maxBytes int64) *uploadFixture {
	t.Helper()
	ms := []Mount{{Name: "m", FS: os.DirFS(filepath.Join("..", "examples", "tutorial-project"))}}
	f := &uploadFixture{}
	f.res = &service.ProjectResolver{
		Store: projects.NewFSStore(projects.Tree{Mount: "m", FS: ms[0].FS}), Config: &ConfigResolver{Mounts: ms},
		UploadMaxBytes: maxBytes,
		Uploads: func(mount string, files map[string][]byte) (service.CacheableLoader, *service.ProjectResolver) {
			f.builds.Add(1)
			fsys := MemFS(files)
			um := []Mount{{Name: mount, FS: fsys}}
			return New(um...), &service.ProjectResolver{Store: projects.NewFSStore(projects.Tree{Mount: mount, FS: fsys}), Config: &ConfigResolver{Mounts: um}}
		},
	}
	loader := service.NewCachingLoader(New(ms...), service.NewDesignCache(32))
	f.check = service.NewCheckService(loader, check.DefaultCatalog(), nil, "", nil, f.res)
	f.snaps = f.check.Snapshots()
	f.query = service.NewQueryService(loader, nil, f.res)
	return f
}

// tutorialFiles is the tutorial project's files by path, as a client holding it would send them.
func tutorialFiles(t *testing.T) map[string][]byte {
	t.Helper()
	root := filepath.Join("..", "examples", "tutorial-project")
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil || len(out) == 0 {
		t.Fatalf("read the tutorial project: %d files, %v", len(out), err)
	}
	return out
}

func manifestOf(files map[string][]byte) []*webapi.SnapshotFile {
	var out []*webapi.SnapshotFile
	for p, b := range files {
		sum := sha256.Sum256(b)
		out = append(out, &webapi.SnapshotFile{Path: p, Sha256: "sha256:" + hex.EncodeToString(sum[:]), Size: int64(len(b))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// send ingests files as mount m's design at uri, uploading what the host asks for.
func (f *uploadFixture) send(t *testing.T, uri string, files map[string][]byte) (*webapi.Snapshot, []string) {
	t.Helper()
	req := &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri, Files: manifestOf(files)}}
	s, err := f.snaps.Ingest(context.Background(), req)
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) {
		if err != nil {
			t.Fatal(err)
		}
		return s, nil
	}
	byHash := map[string][]byte{}
	for _, b := range files {
		sum := sha256.Sum256(b)
		byHash["sha256:"+hex.EncodeToString(sum[:])] = b
	}
	var contents [][]byte
	for _, h := range bm.Hashes {
		contents = append(contents, byHash[h])
	}
	if _, err := f.snaps.UploadBlobs(context.Background(), &webapi.UploadBlobsRequest{Contents: contents}); err != nil {
		t.Fatal(err)
	}
	if s, err = f.snaps.Ingest(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	return s, bm.Hashes
}

func (f *uploadFixture) findings(t *testing.T, snap string) map[string]bool {
	t.Helper()
	resp, err := f.check.CheckDesign(context.Background(), &webapi.CheckDesignRequest{Snapshot: snap})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, x := range resp.GetFindings() {
		out[x.GetRule()+"|"+x.GetSubject().GetRef()] = true
	}
	return out
}

func TestAnUploadedDesignIsReadFromItsFilesAlone(t *testing.T) {
	f := newUploadFixture(t, 0)
	files := tutorialFiles(t)
	up, missing := f.send(t, "mount://m/designs/gateway", files)
	if len(missing) != len(manifestOf(files)) {
		t.Errorf("the first ingest asked for %d contents, want every one of %d", len(missing), len(files))
	}
	served := f.ingest(t, "mount://m/designs/gateway")
	if len(f.findings(t, up.GetName())) == 0 {
		t.Fatal("the uploaded gateway has no findings, so agreeing with the served one proves nothing")
	}
	// The upload is the project the host serves as m, so the two answer alike, project config
	// included, which shows the project inside the upload was resolved.
	if !equalSets(f.findings(t, up.GetName()), f.findings(t, served)) {
		t.Error("the uploaded gateway and the served one answer differently")
	}
	if up.GetProject() == "" {
		t.Error("the uploaded design names no project, so its own project.yaml was not read")
	}
	// Now the same paths with a part removed from the netlist: the upload answers from its files,
	// and neither its reads nor the served mount's reach the other's cache.
	edited := map[string][]byte{}
	for p, b := range files {
		edited[p] = b
	}
	netlist := "designs/gateway/gateway.edn"
	edited[netlist] = bytes.Replace(files[netlist], []byte(`(designator "U1")`), []byte(`(designator "U91")`), 1)
	if bytes.Equal(edited[netlist], files[netlist]) {
		t.Fatal("the edit changed nothing in the netlist, so the comparison below proves nothing")
	}
	changed, missing := f.send(t, "mount://m/designs/gateway", edited)
	if len(missing) != 1 {
		t.Errorf("the edited upload asked for %d contents, want only the edited netlist", len(missing))
	}
	if got := f.parts(t, changed.GetName()); !got["U91"] || got["U1"] {
		t.Errorf("the edited upload's parts are %v; want U91 in place of U1, so a read crossed from the served design", got)
	}
	if got := f.parts(t, served); !got["U1"] || got["U91"] {
		t.Errorf("the served design's parts are %v after the edited upload, so a read crossed from the upload", got)
	}
	if got := f.parts(t, up.GetName()); !got["U1"] || got["U91"] {
		t.Errorf("the first upload's parts are %v after the edited one, so a read crossed between uploads", got)
	}
}

func (f *uploadFixture) ingest(t *testing.T, uri string) string {
	t.Helper()
	s, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: uri}})
	if err != nil {
		t.Fatal(err)
	}
	return s.GetName()
}

// parts is the reference designators a snapshot's design has, asked of the query service.
func (f *uploadFixture) parts(t *testing.T, snap string) map[string]bool {
	t.Helper()
	resp, err := f.query.RunQuery(context.Background(), &webapi.RunQueryRequest{Snapshot: snap, Query: "component.class(?r, ?c) => ?r"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, r := range resp.GetRows() {
		out[r.GetCells()[0]] = true
	}
	return out
}

func equalSets(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func TestOneManifestIsReadThroughOneHost(t *testing.T) {
	f := newUploadFixture(t, 0)
	files := tutorialFiles(t)
	a, _ := f.send(t, "mount://m/designs/gateway", files)
	b, missing := f.send(t, "mount://m/designs/gateway", files)
	if len(missing) != 0 {
		t.Errorf("a second ingest of the same files asked for %d contents again", len(missing))
	}
	if a.GetName() == b.GetName() {
		t.Error("two ingests of one manifest are one snapshot; each ingest is its own")
	}
	if n := f.builds.Load(); n != 1 {
		t.Errorf("one manifest built %d hosts, want one shared by both snapshots", n)
	}
}

func TestAnUploadIsRefusedPastItsLimitsAndOutsideItsMount(t *testing.T) {
	f := newUploadFixture(t, 64)
	if _, err := f.snaps.UploadBlobs(context.Background(), &webapi.UploadBlobsRequest{Contents: [][]byte{bytes.Repeat([]byte("x"), 65)}}); !errors.Is(err, service.ErrResourceExhausted) {
		t.Errorf("a blob over the host's limit answered %v; want resource_exhausted", err)
	}
	big := []*webapi.SnapshotFile{{Path: "a.edn", Sha256: "sha256:00", Size: service.DesignFilesMaxBytes + 1}}
	if _, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/a.edn", Files: big}}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a manifest over the design limit answered %v; want invalid_argument", err)
	}
	escaping := []*webapi.SnapshotFile{{Path: "../a.edn", Sha256: "sha256:00", Size: 1}}
	if _, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/a.edn", Files: escaping}}); !errors.Is(err, service.ErrInvalidPath) {
		t.Errorf("a path leaving the mount answered %v; want invalid_path", err)
	}
	unclean := []*webapi.SnapshotFile{{Path: "a/../b.edn", Sha256: "sha256:00", Size: 1}}
	if _, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/b.edn", Files: unclean}}); !errors.Is(err, service.ErrInvalidPath) {
		t.Errorf("a path spelled two ways answered %v; want invalid_path", err)
	}
	// control: a host that takes no files refuses a manifest outright.
	f.res.Uploads = nil
	small := []*webapi.SnapshotFile{{Path: "a.edn", Sha256: "sha256:00", Size: 1}}
	if _, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/a.edn", Files: small}}); err == nil || !strings.Contains(err.Error(), "takes no files") {
		t.Errorf("a host taking no files answered %v", err)
	}
}

func TestAnEvictedBlobIsAskedForAgain(t *testing.T) {
	f := newUploadFixture(t, 10)
	one, two := []byte("0123456789"), []byte("abcdefghij")
	if _, err := f.snaps.UploadBlobs(context.Background(), &webapi.UploadBlobsRequest{Contents: [][]byte{one, two}}); err != nil {
		t.Fatal(err)
	}
	// The limit holds one of the two, so the first is gone and the ingest names it.
	files := map[string][]byte{"a.edn": one}
	_, err := f.snaps.Ingest(context.Background(), &webapi.IngestRequest{Snapshot: &webapi.Snapshot{Uri: "mount://m/a.edn", Files: manifestOf(files)}})
	var bm *service.BlobsMissingError
	if !errors.As(err, &bm) || len(bm.Hashes) != 1 || bm.Hashes[0] != manifestOf(files)[0].GetSha256() {
		t.Errorf("an ingest naming an evicted blob answered %v; want that blob named as missing", err)
	}
}

func TestASnapshotKeepsItsUploadPastLaterOnes(t *testing.T) {
	f := newUploadFixture(t, 0)
	files := tutorialFiles(t)
	first, _ := f.send(t, "mount://m/designs/gateway", files)
	want := f.findings(t, first.GetName())
	// More manifests than the host keeps, each one byte apart in a file nothing reads.
	for i := 0; i < 20; i++ {
		other := map[string][]byte{"designs/gateway/gateway.edn": files["designs/gateway/gateway.edn"], "pad.txt": []byte{byte(i)}}
		f.send(t, "mount://m/designs/gateway/gateway.edn", other)
	}
	if got := f.findings(t, first.GetName()); !equalSets(got, want) {
		t.Error("the first snapshot answers differently once its upload was evicted from the host's list")
	}
}

func TestAnUpdateReplacesTheFilesASnapshotReads(t *testing.T) {
	f := newUploadFixture(t, 0)
	files := tutorialFiles(t)
	s, _ := f.send(t, "mount://m/designs/gateway", files)
	if !f.parts(t, s.GetName())["U1"] {
		t.Fatal("control: the uploaded gateway has no U1")
	}
	netlist := "designs/gateway/gateway.edn"
	edited := map[string][]byte{}
	for p, b := range files {
		edited[p] = b
	}
	edited[netlist] = bytes.Replace(files[netlist], []byte(`(designator "U1")`), []byte(`(designator "U91")`), 1)
	if _, err := f.snaps.UploadBlobs(context.Background(), &webapi.UploadBlobsRequest{Contents: [][]byte{edited[netlist]}}); err != nil {
		t.Fatal(err)
	}
	u, err := f.snaps.UpdateSnapshot(context.Background(), &webapi.UpdateSnapshotRequest{
		Snapshot:   &webapi.Snapshot{Name: s.GetName(), Files: manifestOf(edited)},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"files"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if u.GetName() != s.GetName() || u.GetRevisionId() == s.GetRevisionId() {
		t.Errorf("the update answered %s at %s; want the same snapshot at a new revision than %s", u.GetName(), u.GetRevisionId(), s.GetRevisionId())
	}
	if got := f.parts(t, s.GetName()); !got["U91"] || got["U1"] {
		t.Errorf("after the update the snapshot's parts are %v; want U91 in place of U1", got)
	}
}
