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
	for _, f := range []string{"app.js", "load.js", "app.css", "agni.wasm", "wasm_exec.js", "agni-worker.js"} {
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
	// Each seed and the drop page open through the Load page (agni issue 1072).
	for _, want := range []string{`href="/agni/demo/designs/gateway/designs/gateway/load/"`, "Sample Board", `href="/agni/demo/designs/local/load/"`} {
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
	for _, page := range []string{"designs/gateway/designs/gateway/load/index.html", "designs/local/load/index.html"} {
		shell := read(page)
		for _, want := range []string{`src="/agni/demo/static/load.js"`, `data-base="/agni/demo/"`, `data-host="static"`, `data-engine="wasm"`, `data-build="`} {
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
	wantBoards(t, &listing, "designs/gateway/gateway.kicad_pcb")
}

// wantBoards fails unless the listing flags exactly want as board files, as ListDesignFiles would
// for the seeded designs (agni issue 992).
func wantBoards(t *testing.T, listing *webapi.ListDesignFilesResponse, want ...string) {
	t.Helper()
	var got []string
	for _, f := range listing.GetFiles() {
		if f.GetBoard() {
			got = append(got, f.GetPath())
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the listing flags %v as boards, want %v", got, want)
	}
}

// gatewayInNoProject copies the tutorial gateway's folder to a temp dir as a design in no project, its
// descriptor without the `extends` naming the project the folder alone does not hold.
func gatewayInNoProject(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	dst := filepath.Join(t.TempDir(), "gateway")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	desc := filepath.Join(dst, "design.yaml")
	b, err := os.ReadFile(desc)
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "extends:") {
			kept = append(kept, line)
		}
	}
	if err := os.WriteFile(desc, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestSiteSeedsAFolderThatIsItselfADesign seeds the gateway's own folder, whose design.yaml declares
// the design at the folder's root. The viewer addresses a design by its path within its mount, so the
// folder goes one level down and its design gets a page and a listing that agree.
func TestSiteSeedsAFolderThatIsItselfADesign(t *testing.T) {
	out := t.TempDir()
	seed := gatewayInNoProject(t)
	if err := writeSite(out, fakeWebDir(t), "/", []string{"gw=" + seed}, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, page := range []string{"view", "load"} {
		if _, err := os.Stat(filepath.Join(out, "designs", "gw", "gateway", page, "index.html")); err != nil {
			t.Errorf("no %s page for the design: %v", page, err)
		}
	}
	landing, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil || !strings.Contains(string(landing), `href="/designs/gw/gateway/load/"`) {
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
	wantBoards(t, &listing, "gateway/gateway.kicad_pcb")
}

// TestSiteWritesTheListingsTheFileTreesRead writes two seeds and reads back what the page answers
// ListMounts and ListDir from on a static host (agni issue 978): every seed's mount, naming no host
// path, and each mount's entries as the server's ListDir names them, folders included, down to the
// gateway's other revision two folders below the mount root.
func TestSiteWritesTheListingsTheFileTreesRead(t *testing.T) {
	out := t.TempDir()
	tutorial := filepath.Join("..", "..", "examples", "tutorial-project")
	seeds := []string{"gateway=" + tutorial, "gw=" + gatewayInNoProject(t)}
	if err := writeSite(out, fakeWebDir(t), "/", seeds, io.Discard); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "mounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var mounts webapi.ListMountsResponse
	if err := protojson.Unmarshal(b, &mounts); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range mounts.GetMounts() {
		got = append(got, m.GetName()+" "+m.GetUri())
		if m.GetRoot() != m.GetName() {
			t.Errorf("mount %s names its root %q, want its own name rather than a host path", m.GetName(), m.GetRoot())
		}
	}
	if want := []string{"gateway mount://gateway", "gw mount://gw"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("mounts %v, want %v", got, want)
	}
	entries := func(mount string) map[string]*webapi.DirEntry {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(out, "dirs", mount+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var dir webapi.ListDirResponse
		if err := protojson.Unmarshal(b, &dir); err != nil {
			t.Fatal(err)
		}
		byURI := map[string]*webapi.DirEntry{}
		for _, e := range dir.GetEntries() {
			byURI[e.GetUri()] = e
		}
		return byURI
	}
	gateway := entries("gateway")
	for _, uri := range []string{"mount://gateway/designs", "mount://gateway/designs/gateway"} {
		if e := gateway[uri]; e == nil || !e.GetIsDir() {
			t.Errorf("gateway lists %s as %v, want a folder", uri, e)
		}
	}
	rev := gateway["mount://gateway/designs/gateway/gateway-rev-c.edn"]
	if rev == nil || rev.GetName() != "gateway-rev-c.edn" || rev.GetFormat() != "edif" || rev.GetKind() != webapi.FileKind_FILE_KIND_DESIGN {
		t.Errorf("gateway lists rev C as %v, want an edif design", rev)
	}
	if e := entries("gw")["mount://gw/gateway/gateway.edn"]; e == nil || e.GetKind() != webapi.FileKind_FILE_KIND_DESIGN {
		t.Errorf("gw lists gateway.edn as %v, want a design", e)
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
