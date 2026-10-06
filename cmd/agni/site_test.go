package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"google.golang.org/protobuf/encoding/protojson"
)

// fakeWebDir is the viewer's templates beside stand-in built assets, so writing a site needs no
// bundle build.
func fakeWebDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := copyTree(filepath.Join("..", "..", "web", "templates"), filepath.Join(dir, "templates")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"app.js", "app.css", "agni.wasm", "wasm_exec.js", "agni-worker.js"} {
		if err := os.WriteFile(filepath.Join(dir, "static", f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestSiteWritesEveryPageUnderItsBase writes a demo under a prefix and reads it back as a static
// host would: the landing page and every viewer page load their assets under the prefix and say they
// are static, the seed's files sit under raw/, and its listing parses as the message the page reads.
func TestSiteWritesEveryPageUnderItsBase(t *testing.T) {
	out := t.TempDir()
	seed := filepath.Join("..", "..", "examples", "tutorial-project")
	if err := writeSite(out, fakeWebDir(t), "/agni/demo", []string{"gateway=" + seed}, io.Discard); err != nil {
		t.Fatal(err)
	}
	read := func(p string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	landing := read("index.html")
	for _, want := range []string{`href="/agni/demo/designs/gateway/designs/gateway/view/"`, "Sample Board", `href="/agni/demo/designs/local/view/"`} {
		if !strings.Contains(landing, want) {
			t.Errorf("landing page lacks %s", want)
		}
	}
	for _, page := range []string{"designs/gateway/designs/gateway/view/index.html", "designs/local/view/index.html"} {
		shell := read(page)
		for _, want := range []string{`href="/agni/demo/static/app.css"`, `src="/agni/demo/static/app.js"`, `data-base="/agni/demo/"`, `data-host="static"`, `data-engine="wasm"`} {
			if !strings.Contains(shell, want) {
				t.Errorf("%s lacks %s", page, want)
			}
		}
		if strings.Contains(shell, `"/static/`) {
			t.Errorf("%s loads an asset from the site root", page)
		}
	}
	var listing webapi.ListDesignFilesResponse
	if err := protojson.Unmarshal([]byte(read("files/gateway.json")), &listing); err != nil || listing.GetMount() != "gateway" || len(listing.GetFiles()) == 0 {
		t.Fatalf("listing: %v %v", err, listing.GetFiles())
	}
	for _, f := range listing.GetFiles() {
		if _, err := os.Stat(filepath.Join(out, "raw", "gateway", filepath.FromSlash(f.GetPath()))); err != nil {
			t.Errorf("listed %s is not under raw/: %v", f.GetPath(), err)
		}
	}
}

// TestSiteSeedsAFolderThatIsItselfADesign seeds the gateway's own folder, whose design.yaml declares
// the design at the folder's root. The viewer addresses a design by its path within its mount, so the
// folder goes one level down and its design gets a page and a listing that agree.
func TestSiteSeedsAFolderThatIsItselfADesign(t *testing.T) {
	out := t.TempDir()
	seed := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	if err := writeSite(out, fakeWebDir(t), "/", []string{"gw=" + seed}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "designs", "gw", "gateway", "view", "index.html")); err != nil {
		t.Errorf("no viewer page for the design: %v", err)
	}
	landing, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil || !strings.Contains(string(landing), `href="/designs/gw/gateway/view/"`) {
		t.Errorf("landing page does not link the design: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "files", "gw.json"))
	if err != nil {
		t.Fatal(err)
	}
	var listing webapi.ListDesignFilesResponse
	if err := protojson.Unmarshal(b, &listing); err != nil || len(listing.GetFiles()) == 0 {
		t.Fatalf("listing: %v %v", err, listing.GetFiles())
	}
	for _, f := range listing.GetFiles() {
		if !strings.HasPrefix(f.GetPath(), "gateway/") {
			t.Errorf("listed %s outside the design's folder", f.GetPath())
		}
		if _, err := os.Stat(filepath.Join(out, "raw", "gw", filepath.FromSlash(f.GetPath()))); err != nil {
			t.Errorf("listed %s is not under raw/: %v", f.GetPath(), err)
		}
	}
}

func TestSiteRefusesWithoutABuiltEngine(t *testing.T) {
	web := fakeWebDir(t)
	os.Remove(filepath.Join(web, "static", "agni.wasm"))
	err := writeSite(t.TempDir(), web, "/", []string{"g=" + filepath.Join("..", "..", "examples", "tutorial-project")}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "make ui wasm") {
		t.Errorf("err %v, want a refusal naming make ui wasm", err)
	}
}
