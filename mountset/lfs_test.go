package mountset_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/goapplib/storage/mounts"
	"github.com/panyam/goapplib/storage/mounts/github/githubtest"

	"github.com/panyam/agni/mountset"
	"github.com/panyam/agni/service"
)

// lfsTable opens a table with uploads, a GitHub mount "gh" over repo acme/lfs, and a folder mount
// "lab" over lab.
func lfsTable(t *testing.T, gh *githubtest.Server, lab string) *mountset.Table {
	t.Helper()
	dir := t.TempDir()
	table, err := mountset.Open(mountset.Config{
		Uploads: filepath.Join(dir, "uploads"), Records: filepath.Join(dir, "store", "mounts"),
		GitHubAPI: gh.URL, GitHubRaw: gh.RawURL(), Fixed: []mounts.Mount{mountset.Folder("lab", lab)},
	})
	if err != nil {
		t.Fatal(err)
	}
	ops := service.WithCaller(context.Background(), service.Caller{Subject: "ops"})
	if _, err := table.CreateMount(ops, "gh", "github://acme/lfs", "main", ""); err != nil {
		t.Fatal(err)
	}
	return table
}

// fetchAll reads every file under p at ref through the mount's reader, the way LinkArtifact does,
// and returns each file's content hash and listed size by path, "sha256:<hex> <size>".
func fetchAll(t *testing.T, table *mountset.Table, mount, ref, p string) (map[string]string, error) {
	t.Helper()
	ctx := context.Background()
	r, err := table.Readers(ctx, mount)
	if err != nil {
		t.Fatal(err)
	}
	version, _, err := r.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	files, err := r.List(ctx, version, p)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range files {
		h, _, err := r.Fetch(ctx, version, f)
		if err != nil {
			return nil, err
		}
		out[f.Path] = fmt.Sprintf("%s %d", h, f.Size)
	}
	return out, nil
}

// sha gives s's hash and size as fetchAll reports them.
func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("sha256:%s %d", hex.EncodeToString(sum[:]), len(s))
}

func TestALinkFromGitHubReadsAGitLFSFileAsItsObject(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "lfs", false, map[string]string{"hw/bb.kicad_pro": "{}", ".gitattributes": "*.kicad_sch filter=lfs diff=lfs merge=lfs -text\n"})
	// An LFS object is larger than its pointer in practice. goapplib#218 tracks the case of one
	// smaller than its pointer.
	schematic := strings.Repeat("(kicad_sch (version 20231120) (generator eeschema))\n", 40)
	repo.CommitLFS("main", "hw/bb.kicad_sch", schematic)
	table := lfsTable(t, gh, t.TempDir())

	got, err := fetchAll(t, table, "gh", "main", "hw")
	if err != nil {
		t.Fatalf("read hw at main: %v", err)
	}
	pointer, _ := githubtest.LFSPointer(schematic)
	if got["hw/bb.kicad_sch"] != sha(schematic) {
		t.Errorf("hw/bb.kicad_sch read as %s; want the object's hash and size %s, not the pointer's %s", got["hw/bb.kicad_sch"], sha(schematic), sha(pointer))
	}
}

func TestAnLFSObjectGitHubLacksFailsTheReadNamingThePath(t *testing.T) {
	gh := githubtest.New(t)
	repo := gh.AddRepo("acme", "lfs", false, map[string]string{"hw/bb.kicad_pro": "{}"})
	table := lfsTable(t, gh, t.TempDir())
	_, oid := repo.CommitLFS("main", "hw/bb.kicad_sch", "(kicad_sch)\n")
	gh.RemoveLFSObject(oid)

	_, err := fetchAll(t, table, "gh", "main", "hw")
	if !errors.Is(err, service.ErrFailedPrecondition) || !errors.Is(err, mounts.ErrLFS) || !strings.Contains(err.Error(), "hw/bb.kicad_sch") {
		t.Errorf("a link's read of a pointer whose object is gone = %v; want FAILED_PRECONDITION wrapping ErrLFS and naming hw/bb.kicad_sch", err)
	}
	ops := service.WithCaller(context.Background(), service.Caller{Subject: "ops"})
	_, err = table.CreateMount(ops, "again", "github://acme/lfs", "main", "")
	if !errors.Is(err, service.ErrFailedPrecondition) || !errors.Is(err, mounts.ErrLFS) || !strings.Contains(err.Error(), "hw/bb.kicad_sch") {
		t.Errorf("mounting a commit with a pointer whose object is gone = %v; want FAILED_PRECONDITION wrapping ErrLFS and naming hw/bb.kicad_sch", err)
	}
}

func TestAFolderMountRefusesAGitLFSPointerNamingThePath(t *testing.T) {
	lab := t.TempDir()
	pointer, _ := githubtest.LFSPointer("(kicad_sch)\n")
	write(t, filepath.Join(lab, "hw", "bb.kicad_sch"), pointer)
	write(t, filepath.Join(lab, "hw", "bb.kicad_pro"), "{}")
	gh := githubtest.New(t)
	gh.AddRepo("acme", "lfs", false, map[string]string{"README.md": "x"})
	table := lfsTable(t, gh, lab)

	_, err := fetchAll(t, table, "lab", "", "hw")
	if !errors.Is(err, service.ErrFailedPrecondition) || !errors.Is(err, mounts.ErrLFS) || !strings.Contains(err.Error(), "hw/bb.kicad_sch") {
		t.Errorf("reading a checkout without git lfs pull = %v; want FAILED_PRECONDITION wrapping ErrLFS and naming hw/bb.kicad_sch", err)
	}
}
