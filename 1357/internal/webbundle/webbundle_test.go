package webbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

const tag = "v9.8.7"

func viewerFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/ViewerPage.html": {Data: []byte("<html>viewer</html>")},
		"static/app.js":             {Data: []byte("console.log(1)")},
	}
}

func tarball(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

// release serves one tag's checksums and bundle the way GitHub's download URLs do, counting fetches.
func release(t *testing.T, bundle []byte, sum string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/" + tag + "/" + ChecksumsName:
			fmt.Fprintf(w, "0000  agni_%s_linux_amd64.tar.gz\n%s  %s\n", tag, sum, AssetName(tag))
		case "/" + tag + "/" + AssetName(tag):
			w.Write(bundle)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestEmbeddedBundleIsWrittenOnceAndReused(t *testing.T) {
	src := Source{Embedded: viewerFS(), Version: "abc123", CacheRoot: t.TempDir()}
	dir, how, err := src.Dir(context.Background())
	if err != nil || how != "embedded" {
		t.Fatalf("Dir = %q, %q, %v", dir, how, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "static", "app.js"))
	if err != nil || string(b) != "console.log(1)" {
		t.Fatalf("static/app.js = %q, %v", b, err)
	}
	again, _, err := src.Dir(context.Background())
	if err != nil || again != dir {
		t.Errorf("second Dir = %q, %v, want the same %q", again, err, dir)
	}
}

func TestADifferentEmbeddedViewerGetsItsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	first, _, _ := Source{Embedded: viewerFS(), CacheRoot: root}.Dir(context.Background())
	changed := viewerFS()
	changed["static/app.js"] = &fstest.MapFile{Data: []byte("console.log(2)")}
	second, _, _ := Source{Embedded: changed, CacheRoot: root}.Dir(context.Background())
	if first == second {
		t.Errorf("two different viewers share %q", first)
	}
}

func TestAReleaseDownloadsItsBundleOnce(t *testing.T) {
	bundle := tarball(t, map[string]string{"templates/ViewerPage.html": "<html/>", "static/app.js": "x"})
	srv, hits := release(t, bundle, sha(bundle))
	var notes strings.Builder
	src := Source{Version: tag, Releases: srv.URL, CacheRoot: t.TempDir(), Notes: &notes}
	dir, how, err := src.Dir(context.Background())
	if err != nil || how != "downloaded" {
		t.Fatalf("Dir = %q, %q, %v", dir, how, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "static", "app.js")); err != nil {
		t.Error(err)
	}
	if !strings.Contains(notes.String(), AssetName(tag)) {
		t.Errorf("no note named the download: %q", notes.String())
	}
	before := hits.Load()
	if _, _, err := src.Dir(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != before {
		t.Errorf("a second Dir downloaded again (%d requests, then %d)", before, hits.Load())
	}
}

func TestADownloadThatFailsItsChecksumIsRefusedAndNotKept(t *testing.T) {
	bundle := tarball(t, map[string]string{"templates/ViewerPage.html": "<html/>"})
	srv, _ := release(t, bundle, sha([]byte("something else")))
	root := t.TempDir()
	_, _, err := Source{Version: tag, Releases: srv.URL, CacheRoot: root}.Dir(context.Background())
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("a bundle not matching its checksum was accepted: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "web")); len(entries) != 0 {
		t.Errorf("a refused download left %d entries in the cache", len(entries))
	}
}

func TestAnArchiveNamingAPathOutsideItselfIsRefused(t *testing.T) {
	bundle := tarball(t, map[string]string{"templates/ViewerPage.html": "<html/>", "../escape.txt": "x"})
	srv, _ := release(t, bundle, sha(bundle))
	root := t.TempDir()
	_, _, err := Source{Version: tag, Releases: srv.URL, CacheRoot: filepath.Join(root, "cache")}.Dir(context.Background())
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("an archive escaping its directory was accepted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "cache", "web", "escape.txt")); err == nil {
		t.Error("the escaping entry was written")
	}
}

func TestABuildThatIsNotAReleaseHasNoBundle(t *testing.T) {
	srv, hits := release(t, nil, "")
	for _, v := range []string{"(devel)", "unknown", "4f2a9c1e0b7d", "4f2a9c1e0b7d+dirty", "v0.3.13-0.20261030000000-4f2a9c1e0b7d"} {
		_, _, err := Source{Version: v, Releases: srv.URL, CacheRoot: t.TempDir()}.Dir(context.Background())
		if !errors.Is(err, ErrNoBundle) {
			t.Errorf("version %q: err = %v, want ErrNoBundle", v, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("a build with no release made %d requests", hits.Load())
	}
}

func TestAReleaseWithNoBundleSaysWhichURLAnswered(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	_, _, err := Source{Version: tag, Releases: srv.URL, CacheRoot: t.TempDir()}.Dir(context.Background())
	if err == nil || errors.Is(err, ErrNoBundle) || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v, want the 404 named and not ErrNoBundle", err)
	}
}
