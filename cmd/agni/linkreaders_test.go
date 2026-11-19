package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/panyam/agni/internal/mountset"
	"github.com/panyam/agni/service"
)

// The fake repo's commits. linkV1 is the tag v1, and the branch main starts there.
const (
	linkV1 = "1111111111111111111111111111111111111111"
	linkV2 = "2222222222222222222222222222222222222222"
)

// fakeRepo serves one repo's commits the way GitHub does for a link: its default branch, a ref's
// commit, which refs are branches, a commit's file list with git blob ids, each file from the raw
// host under /raw, and the tarball a mount snapshots. It counts the raw downloads by path.
type fakeRepo struct {
	owner, repo string
	mu          sync.Mutex
	refs        map[string]string            // ref -> commit
	branches    map[string]bool              // refs that are branches
	commits     map[string]map[string]string // commit -> path -> contents
	tamper      bool                         // send each raw file with a byte changed
	downloads   []string
}

func (f *fakeRepo) setRef(ref, commit string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refs[ref] = commit
}

func (f *fakeRepo) took() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.downloads)
	f.downloads = nil
	return out
}

func (f *fakeRepo) serve(t *testing.T) *httptest.Server {
	t.Helper()
	base := "/repos/" + f.owner + "/" + f.repo
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"default_branch":"main"}`)
	})
	commitOf := func(ref string) (string, bool) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if _, ok := f.commits[ref]; ok {
			return ref, true
		}
		c, ok := f.refs[ref]
		return c, ok
	}
	mux.HandleFunc("GET "+base+"/commits/{ref}", func(w http.ResponseWriter, r *http.Request) {
		c, ok := commitOf(r.PathValue("ref"))
		if !ok {
			http.Error(w, `{"message":"No commit found"}`, http.StatusUnprocessableEntity)
			return
		}
		_, _ = io.WriteString(w, `{"sha":"`+c+`"}`)
	})
	mux.HandleFunc("GET "+base+"/branches/{ref}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		ok := f.branches[r.PathValue("ref")]
		f.mu.Unlock()
		if !ok {
			http.Error(w, `{"message":"Branch not found"}`, http.StatusNotFound)
			return
		}
		_, _ = io.WriteString(w, `{"name":"`+r.PathValue("ref")+`"}`)
	})
	mux.HandleFunc("GET "+base+"/git/trees/{commit}", func(w http.ResponseWriter, r *http.Request) {
		files, ok := f.commits[r.PathValue("commit")]
		if !ok || r.URL.Query().Get("recursive") != "1" {
			http.NotFound(w, r)
			return
		}
		var entries []string
		for p, c := range files {
			entries = append(entries, `{"path":"`+p+`","mode":"100644","type":"blob","sha":"`+gitBlobSHA([]byte(c))+`","size":`+strconv.Itoa(len(c))+`}`)
		}
		// A folder entry and a symlink, which a link leaves out.
		entries = append(entries, `{"path":"hw","mode":"040000","type":"tree","sha":"`+strings.Repeat("3", 40)+`"}`)
		entries = append(entries, `{"path":"hw/escape","mode":"120000","type":"blob","sha":"`+strings.Repeat("4", 40)+`","size":3}`)
		_, _ = io.WriteString(w, `{"tree":[`+strings.Join(entries, ",")+`],"truncated":false}`)
	})
	mux.HandleFunc("GET /raw/"+f.owner+"/"+f.repo+"/{commit}/{path...}", func(w http.ResponseWriter, r *http.Request) {
		c, ok := f.commits[r.PathValue("commit")][r.PathValue("path")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		f.downloads = append(f.downloads, r.PathValue("path"))
		tamper := f.tamper
		f.mu.Unlock()
		if tamper {
			c += "!"
		}
		_, _ = io.WriteString(w, c)
	})
	mux.HandleFunc("GET "+base+"/tarball/{commit}", func(w http.ResponseWriter, r *http.Request) {
		files, ok := f.commits[r.PathValue("commit")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(tarballOf(t, f.owner+"-"+f.repo+"-top/", files))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func tarballOf(t *testing.T, top string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for p, c := range files {
		if err := tw.WriteHeader(&tar.Header{Name: top + p, Mode: 0o644, Size: int64(len(c)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// boardRepo is a KiCad design under hw/ at v1, and at v2 with one child sheet changed.
func boardRepo() *fakeRepo {
	v1 := map[string]string{
		"README.md":          "# baseboard\n",
		"hw/bb.kicad_pro":    "{}",
		"hw/bb.kicad_sch":    `(kicad_sch (sheet (property "Sheetfile" "power.kicad_sch")))`,
		"hw/power.kicad_sch": "(kicad_sch)",
		"hw/bb.kicad_pcb":    "(kicad_pcb)",
	}
	v2 := map[string]string{}
	for p, c := range v1 {
		v2[p] = c
	}
	v2["hw/power.kicad_sch"] = "(kicad_sch (power))"
	return &fakeRepo{
		owner: "antmicro", repo: "baseboard",
		refs:     map[string]string{"v1": linkV1, "main": linkV1},
		branches: map[string]bool{"main": true},
		commits:  map[string]map[string]string{linkV1: v1, linkV2: v2},
	}
}

// githubReaderOver is the reader linkReaders gives for a github mount over repo served at srv.
func githubReaderOver(t *testing.T, repo *fakeRepo, srv *httptest.Server) service.MountReader {
	t.Helper()
	dir := t.TempDir()
	table := mountset.NewTable(mountset.Mount{Name: "bb", Kind: mountset.KindGitHub, Source: "github://" + repo.owner + "/" + repo.repo})
	maker, err := newMountMaker(table, filepath.Join(dir, "store", storeMountsDir), filepath.Join(dir, "uploads"), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	r, err := newLinkReaders(table, maker, srv.URL+"/raw").reader(context.Background(), "bb")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAGitHubReaderResolvesRefsAndDownloadsOnlyFilesItLacks(t *testing.T) {
	ctx := context.Background()
	repo := boardRepo()
	srv := repo.serve(t)
	r := githubReaderOver(t, repo, srv)

	for _, c := range []struct {
		ref    string
		follow bool
	}{{"main", true}, {"", true}, {"v1", false}, {linkV1, false}} {
		v, follow, err := r.Resolve(ctx, c.ref)
		if err != nil || v != linkV1 || follow != c.follow {
			t.Errorf("Resolve(%q) = %s, %v, %v; want %s, follow %v", c.ref, v, follow, err, linkV1, c.follow)
		}
	}
	if _, _, err := r.Resolve(ctx, "v9"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("Resolve of a ref GitHub lacks = %v, want ErrNotFound", err)
	}

	hw, err := r.List(ctx, linkV1, "hw")
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, f := range hw {
		listed = append(listed, f.Path)
	}
	if want := []string{"hw/bb.kicad_pcb", "hw/bb.kicad_pro", "hw/bb.kicad_sch", "hw/power.kicad_sch"}; !slices.Equal(listed, want) {
		t.Errorf("List(hw) = %v, want %v, sorted, with the symlink and the folder left out", listed, want)
	}
	for _, f := range hw {
		if _, fetched, err := r.Fetch(ctx, linkV1, f); err != nil || !fetched {
			t.Fatalf("Fetch(%s) at v1 = %v, %v; want a download", f.Path, fetched, err)
		}
	}
	if got := repo.took(); len(got) != 4 {
		t.Errorf("v1 downloaded %v, want its four files", got)
	}

	v2, err := r.List(ctx, linkV2, "hw")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range v2 {
		if _, _, err := r.Fetch(ctx, linkV2, f); err != nil {
			t.Fatal(err)
		}
	}
	if got := repo.took(); !slices.Equal(got, []string{"hw/power.kicad_sch"}) {
		t.Errorf("v2 downloaded %v, want only the sheet whose git blob id changed", got)
	}

	repo.tamper = true
	if _, _, err := r.Fetch(ctx, linkV1, service.MountFile{Path: "README.md", ID: gitBlobSHA([]byte("# baseboard\n")), Size: 12}); !errors.Is(err, service.ErrUnavailable) {
		t.Errorf("a download that doesn't match its git blob id = %v, want ErrUnavailable", err)
	}
}

func TestAFolderReaderReadsAMountsFilesAndLeavesLinksOut(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for p, c := range map[string]string{"job/a.txt": "a", "job/sub/b.txt": "b", "top.txt": "t"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "top.txt"), filepath.Join(root, "job", "link.txt")); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	dir := t.TempDir()
	table := mountset.NewTable(mountset.Mount{Name: "boards", Root: root})
	maker, err := newMountMaker(table, filepath.Join(dir, "store", storeMountsDir), filepath.Join(dir, "uploads"), "http://127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r, err := newLinkReaders(table, maker, "").reader(ctx, "boards")
	if err != nil {
		t.Fatal(err)
	}
	v, follow, err := r.Resolve(ctx, "")
	if err != nil || v != "" || !follow {
		t.Errorf("Resolve on a folder = %q, %v, %v; want no version, followed", v, follow, err)
	}
	job, err := r.List(ctx, "", "job")
	if err != nil {
		t.Fatal(err)
	}
	var listed []string
	for _, f := range job {
		listed = append(listed, f.Path)
	}
	if want := []string{"job/a.txt", "job/sub/b.txt"}; !slices.Equal(listed, want) {
		t.Errorf("List(job) = %v, want %v with the link left out", listed, want)
	}
	if _, fetched, err := r.Fetch(ctx, "", job[0]); err != nil || !fetched {
		t.Errorf("a first Fetch = %v, %v; want it kept", fetched, err)
	}
	if _, fetched, err := r.Fetch(ctx, "", job[0]); err != nil || fetched {
		t.Errorf("a second Fetch = %v, %v; want nothing new kept", fetched, err)
	}
	if _, err := r.List(ctx, "", "../outside"); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("List of a path climbing out = %v, want ErrInvalidArgument", err)
	}
	if _, err := newLinkReaders(table, maker, "").reader(ctx, "nope"); !errors.Is(err, service.ErrNotFound) {
		t.Errorf("the reader for an unknown mount = %v, want ErrNotFound", err)
	}
}

func TestTheDesignsCommandLinksSyncsAndUnlinksAGitHubArtifact(t *testing.T) {
	repo := boardRepo()
	srv := repo.serve(t)
	dir := t.TempDir()
	opts := viewerOpts{storeDir: filepath.Join(dir, "store"), uploadsDir: filepath.Join(dir, "uploads"), githubAPI: srv.URL, githubRaw: srv.URL + "/raw"}
	url := serveViewer(t, opts)
	must := func(out string, err error) string {
		t.Helper()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return out
	}
	must(runMounts(t, "add", "bb", "github://antmicro/baseboard", "--remote", url))
	must(runDesigns(t, url, "create", "bb"))
	out := must(runDesigns(t, url, "link", "bb", "schematic", "bb/hw/bb.kicad_sch@main"))
	if !strings.Contains(out, "3 files") || !strings.Contains(out, "from bb/hw/bb.kicad_sch@main at "+linkV1+", followed") {
		t.Errorf("designs link = %q; want three files from main at v1, followed", out)
	}
	out = must(runDesigns(t, url, "link", "bb", "board", "bb/hw/bb.kicad_pcb@v1"))
	if !strings.Contains(out, "from bb/hw/bb.kicad_pcb@v1 at "+linkV1+", pinned") {
		t.Errorf("designs link of the board = %q; want it pinned at v1", out)
	}
	repo.took()

	repo.setRef("main", linkV2)
	out = must(runDesigns(t, url, "sync", "bb"))
	if want := "designs/bb/artifacts/board\tunchanged\t" + linkV1[:12] + " -> " + linkV1[:12] + "\tfetched 0 files, 0 bytes\n" +
		"designs/bb/artifacts/schematic\tchanged\t" + linkV1[:12] + " -> " + linkV2[:12] + "\tfetched 1 files, 19 bytes\n"; out != want {
		t.Errorf("designs sync =\n%s\nwant\n%s", out, want)
	}
	if got := repo.took(); !slices.Equal(got, []string{"hw/power.kicad_sch"}) {
		t.Errorf("the sync downloaded %v, want the changed sheet alone", got)
	}
	if out, err := runMounts(t, "rm", "bb", "--remote", url); err == nil || !strings.Contains(err.Error(), "designs/bb/artifacts/board, designs/bb/artifacts/schematic are linked to the mount") {
		t.Errorf("mounts rm with artifacts linked = %q, %v; want a refusal naming both", out, err)
	}

	// A restart over the same folders keeps each link where the sync left it.
	url = serveViewer(t, opts)
	out = must(runDesigns(t, url, "artifacts", "bb"))
	if !strings.Contains(out, "from bb/hw/bb.kicad_sch@main at "+linkV2) {
		t.Errorf("after a restart, designs artifacts = %q; want the schematic at v2", out)
	}
	if out, err := runDesigns(t, url, "put", "bb", "board", filepath.Join(dir, "store")); err == nil || !strings.Contains(err.Error(), "UnlinkArtifact") {
		t.Errorf("designs put over a linked board = %q, %v; want a refusal saying to unlink", out, err)
	}
	must(runDesigns(t, url, "unlink", "bb", "board"))
	must(runDesigns(t, url, "unlink", "bb", "schematic"))
	if out, err := runMounts(t, "rm", "bb", "--remote", url); err != nil || out != "removed bb\n" {
		t.Errorf("mounts rm after unlinking = %q, %v; want it removed", out, err)
	}
}
