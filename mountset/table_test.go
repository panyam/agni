package mountset_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/blobs"
	"github.com/panyam/goapplib/storage/mounts"

	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func uri(t *testing.T, s string) mounts.URI {
	t.Helper()
	u, err := mounts.ParseURI(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestATableResolvesFolderPathsArchivesAndLinks(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	write(t, filepath.Join(root, "designs", "top.kicad_sch"), "(kicad_sch)")
	write(t, filepath.Join(root, "jobs", "board.tgz"), "not really a tarball")
	write(t, filepath.Join(root, "libs", "real.kicad_sym"), "(sym)")
	write(t, filepath.Join(outside, "secret"), "no")
	if err := os.Symlink("real.kicad_sym", filepath.Join(root, "libs", "linked.kicad_sym")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := os.Symlink(filepath.Join("..", "..", filepath.Base(outside), "secret"), filepath.Join(root, "libs", "out")); err != nil {
		t.Fatal(err)
	}
	table := mountset.NewTable(mountset.Folder("Boards Lab", root))

	for in, want := range map[string]string{
		"mount://Boards Lab":                          root,
		"mount://Boards Lab/designs/top.kicad_sch":    filepath.Join(root, "designs", "top.kicad_sch"),
		"mount://Boards Lab/jobs/board.tgz/steps/pcb": filepath.Join(root, "jobs", "board.tgz", "steps", "pcb"),
		"mount://Boards Lab/libs/linked.kicad_sym":    filepath.Join(root, "libs", "linked.kicad_sym"),
		"mount://Boards Lab/designs/new.kicad_sch":    filepath.Join(root, "designs", "new.kicad_sch"),
	} {
		got, err := table.Resolve(uri(t, in))
		if err != nil || got != want {
			t.Errorf("Resolve(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := table.Resolve(uri(t, "mount://Boards Lab/libs/out")); !errors.Is(err, service.ErrInvalidPath) {
		t.Errorf("Resolve of a link out of the mount = %v, want service.ErrInvalidPath", err)
	}
	if _, err := table.Resolve(uri(t, "mount://designs/top.kicad_sch/x")); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Resolve in an unknown mount = %v, want service.ErrNotFound", err)
	}
	if _, err := table.Resolve(uri(t, "mount://Boards Lab/designs/top.kicad_sch/x")); !errors.Is(err, service.ErrInvalidPath) {
		t.Errorf("Resolve through a file that isn't an archive = %v, want service.ErrInvalidPath", err)
	}
}

// oldRecord writes a runtime mount record the way agni wrote one before goapplib storage, with its
// files' blobs in uploads, and returns the commit.
func oldRecord(t *testing.T, records, uploads, name string, files map[string]string, keepBlobs bool) string {
	t.Helper()
	commit := strings.Repeat("ab", 20)
	b, err := blobs.NewDir(filepath.Join(uploads, mountset.BlobsDir))
	if err != nil {
		t.Fatal(err)
	}
	var list strings.Builder
	for p, body := range files {
		h := blobs.HashOf([]byte(body))
		if keepBlobs {
			if _, err := b.Put(context.Background(), strings.NewReader(body)); err != nil {
				t.Fatal(err)
			}
		}
		fmt.Fprintf(&list, "  - path: %s\n    sha256: %s\n", p, h)
	}
	write(t, filepath.Join(records, name+".yaml"), fmt.Sprintf("name: %s\nkind: github\nsource: github://acme/%s\nref: main\ncommit: %s\ncreated: 2026-11-18T00:00:00Z\nfiles:\n%s", name, name, commit, list.String()))
	return commit
}

func TestOpenRewritesAnOlderAgnisMountRecords(t *testing.T) {
	dir := t.TempDir()
	records, uploads := filepath.Join(dir, "store", "mounts"), filepath.Join(dir, "uploads")
	commit := oldRecord(t, records, uploads, "jetson", map[string]string{"hw/board.kicad_pcb": "(kicad_pcb)", "README.md": "hi"}, true)
	oldRecord(t, records, uploads, "lost", map[string]string{"a.txt": "gone"}, false)

	write(t, filepath.Join(uploads, "links", "git", strings.Repeat("c", 40)), "sha256:old")
	var log bytes.Buffer
	table, err := mountset.Open(mountset.Config{Uploads: uploads, Records: records, Log: &log})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := table.Find("jetson")
	if !ok || m.Kind != mounts.KindGitHub || m.Version != commit || m.Mode != mounts.ModeSnapshot || m.Manifest == "" {
		t.Fatalf("the rewritten mount = %+v, %v; want a github snapshot at %s with a manifest", m, ok, commit)
	}
	got, err := table.Resolve(uri(t, "mount://jetson/hw/board.kicad_pcb"))
	if err != nil || got != filepath.Join(uploads, mountset.SnapshotsDir, "jetson", commit, "hw", "board.kicad_pcb") {
		t.Fatalf("Resolve = %q, %v; want the file in the snapshot's view", got, err)
	}
	if b, err := os.ReadFile(got); err != nil || string(b) != "(kicad_pcb)" {
		t.Errorf("the view's file = %q, %v", b, err)
	}
	if _, ok := table.Find("lost"); ok {
		t.Errorf("a record whose blobs are gone is still served")
	}
	if _, err := os.Stat(filepath.Join(records, "lost.yaml.old")); err != nil || !strings.Contains(log.String(), "lost.yaml") {
		t.Errorf("the record with lost blobs wasn't kept aside and reported: %v; log %q", err, log.String())
	}

	if _, err := os.Stat(filepath.Join(uploads, "links")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open left #1758's git index in the uploads: %v", err)
	}
	again, err := mountset.Open(mountset.Config{Uploads: uploads, Records: records})
	if err != nil {
		t.Fatal(err)
	}
	if m2, ok := again.Find("jetson"); !ok || m2.Manifest != m.Manifest {
		t.Errorf("a second Open = %+v, %v; want the rewritten record read as it is", m2, ok)
	}
}

func TestAReadOnlyTableNeitherRewritesNorNeedsTheRecordsFolder(t *testing.T) {
	dir := t.TempDir()
	records, uploads := filepath.Join(dir, "store", "mounts"), filepath.Join(dir, "uploads")
	if _, err := mountset.Open(mountset.Config{Uploads: uploads, Records: records, ReadOnly: true}); err != nil {
		t.Fatalf("a read-only Open with no records folder = %v, want no mounts and no error", err)
	}
	if _, err := os.Stat(records); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a read-only Open made the records folder: %v", err)
	}
	oldRecord(t, records, uploads, "jetson", map[string]string{"a.txt": "a"}, true)
	before, _ := os.ReadFile(filepath.Join(records, "jetson.yaml"))
	mountset.Open(mountset.Config{Uploads: uploads, Records: records, ReadOnly: true})
	after, _ := os.ReadFile(filepath.Join(records, "jetson.yaml"))
	if !bytes.Equal(before, after) {
		t.Errorf("a read-only Open rewrote a record")
	}
}

// A read-only reader opens over --uploads and --store folders it can't create, as agnids does in its
// image when nothing is mounted at /var/lib/agni, and serves no runtime mounts rather than failing to
// start. It makes neither folder.
func TestAReadOnlyTableNeedsNoUploadsFolderItCannotMake(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("needs a folder this user can't write into")
	}
	parent := filepath.Join(t.TempDir(), "agni")
	if err := os.Mkdir(parent, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	records, uploads := filepath.Join(parent, "meta", "mounts"), filepath.Join(parent, "files")
	table, err := mountset.Open(mountset.Config{Uploads: uploads, Records: records, ReadOnly: true})
	if err != nil {
		t.Fatalf("a read-only Open over uploads it can't create = %v, want no mounts and no error", err)
	}
	if got := table.List(); len(got) != 0 {
		t.Errorf("List = %v, want no mounts", got)
	}
	if _, err := os.Stat(uploads); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a read-only Open made the uploads folder: %v", err)
	}
}

func TestErrMapsTheLibrarysErrorsOntoServiceErrors(t *testing.T) {
	for lib, svc := range map[error]error{
		mounts.ErrNotFound:    service.ErrNotFound,
		mounts.ErrExists:      service.ErrAlreadyExists,
		mounts.ErrInvalidPath: service.ErrInvalidPath,
		mounts.ErrFixed:       service.ErrFailedPrecondition,
		mounts.ErrTooLarge:    service.ErrResourceExhausted,
		mounts.ErrUnavailable: service.ErrUnavailable,
		mounts.ErrLFS:         service.ErrFailedPrecondition,
	} {
		got := mountset.Err(fmt.Errorf("%w: detail", lib))
		if !errors.Is(got, svc) || !errors.Is(got, lib) || !strings.Contains(got.Error(), "detail") {
			t.Errorf("Err(%v) = %v; want it to wrap %v and keep the message", lib, got, svc)
		}
	}
	plain := errors.New("plain")
	if mountset.Err(plain) != plain || mountset.Err(nil) != nil {
		t.Errorf("Err changed an error that isn't the library's")
	}
}
