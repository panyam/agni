package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/gen/go/agni/v1/webapi/webapiconnect"
	"github.com/panyam/agni/mountset"
)

// Adding the tutorial's rev B copies the symbol folder its design.yaml names, ../gateway/symbols,
// as an artifact of its own, and keeps the project it extends as a pack it reads under (agni issue
// 1756).
func TestAnAddedDesignCopiesItsSymbolsAndReadsUnderItsProjectsPack(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srv := designServer(t, filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), mountset.Folder("tut", "../../examples/tutorial-project"))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://tut/designs", Designs: []*webapi.DesignToAdd{{Folder: "designs/gateway-rev-b", DesignId: "rev-b"}}}))
	if err != nil {
		t.Fatal(err)
	}
	d := added.Msg.GetDesigns()[0]
	if !slices.Equal(d.GetPacks(), []string{"packs/gateway"}) {
		t.Errorf("rev-b's packs = %v; want the project it extends, packs/gateway", d.GetPacks())
	}
	if syms := d.GetConfig().GetSymbolPathUris(); len(syms) != 1 || !strings.HasSuffix(syms[0], "/symbols") {
		t.Errorf("rev-b's symbol folders = %v; want the view's symbols artifact", syms)
	}
	arts, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/rev-b"}))
	if err != nil {
		t.Fatal(err)
	}
	var symbols *webapi.Artifact
	for _, a := range arts.Msg.GetArtifacts() {
		if a.GetType() == webapi.ArtifactType_ARTIFACT_TYPE_SYMBOLS {
			symbols = a
		}
	}
	if symbols == nil || symbols.GetName() != "designs/rev-b/artifacts/symbols" || symbols.GetSource().GetPath() != "designs/gateway/symbols" || len(symbols.GetFiles()) == 0 {
		t.Fatalf("rev-b's artifacts = %v; want a symbols artifact copied from designs/gateway/symbols", arts.Msg.GetArtifacts())
	}

	// The view's design.yaml, which a read opens, names the symbols artifact's folder.
	views, err := filepath.Glob(filepath.Join(dir, "uploads", "designs", "rev-b", "*", "design.yaml"))
	if err != nil || len(views) != 1 {
		t.Fatalf("rev-b's views = %v, %v; want one", views, err)
	}
	if b, err := os.ReadFile(views[0]); err != nil || !strings.Contains(string(b), "symbols:\n    - symbols\n") && !strings.Contains(string(b), "symbols:\n- symbols\n") {
		t.Errorf("rev-b's view design.yaml = %q, %v; want symbols: naming the symbols artifact", b, err)
	}

	// A check by name with no --pack reads under the design's pack: it answers as --pack gateway does,
	// and unlike a check under no pack at all.
	implicit, err := runAgni(t, "check", "designs/rev-b", "--remote", srv.URL, "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := runAgni(t, "check", "designs/rev-b", "--remote", srv.URL, "--format", "json", "--pack", "gateway")
	if err != nil {
		t.Fatal(err)
	}
	if findingCount(t, implicit) != findingCount(t, explicit) {
		t.Errorf("a check of designs/rev-b with no --pack found %d, and with --pack gateway %d; want the same, read under its pack", findingCount(t, implicit), findingCount(t, explicit))
	}
}

func TestNoSymbolCopyLeavesTheFoldersOutAndSaysSo(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	srv := designServer(t, filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), mountset.Folder("tut", "../../examples/tutorial-project"))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://tut/designs", NoSymbolCopy: true, Designs: []*webapi.DesignToAdd{{Folder: "designs/gateway-rev-b", DesignId: "rev-b"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := added.Msg.GetUncopiedSymbols(); !slices.Equal(got, []string{"designs/rev-b: mount://tut/designs/gateway/symbols"}) {
		t.Errorf("uncopied_symbols = %v; want rev-b's one symbol folder", got)
	}
	arts, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/rev-b"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range arts.Msg.GetArtifacts() {
		if a.GetType() == webapi.ArtifactType_ARTIFACT_TYPE_SYMBOLS {
			t.Errorf("no_symbol_copy still copied %s", a.GetName())
		}
	}
}

func findingCount(t *testing.T, out string) int {
	t.Helper()
	i := strings.Index(out, "{")
	var resp struct {
		Findings []json.RawMessage `json:"findings"`
	}
	if i < 0 || json.Unmarshal([]byte(out[i:]), &resp) != nil {
		t.Fatalf("not a check's JSON: %.200s", out)
	}
	return len(resp.Findings)
}
