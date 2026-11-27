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
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://tut/boards", Designs: []*webapi.DesignToAdd{{Folder: "boards/gateway-rev-b", DesignId: "rev-b"}}}))
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
	if symbols == nil || symbols.GetName() != "designs/rev-b/artifacts/symbols" || symbols.GetSource().GetPath() != "boards/gateway/symbols" || len(symbols.GetFiles()) == 0 {
		t.Fatalf("rev-b's artifacts = %v; want a symbols artifact copied from boards/gateway/symbols", arts.Msg.GetArtifacts())
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
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://tut/boards", NoSymbolCopy: true, Designs: []*webapi.DesignToAdd{{Folder: "boards/gateway-rev-b", DesignId: "rev-b"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := added.Msg.GetUncopiedSymbols(); !slices.Equal(got, []string{"designs/rev-b: mount://tut/boards/gateway/symbols"}) {
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

// Adding Glasgow from a mount copies the params folder its design.yaml names as an artifact of type
// ARTIFACT_TYPE_PARAMS, and the design's config names it, so U15's TPD3S014 spec reaches a check by
// name (agni issue 1922).
func TestAnAddedDesignCopiesItsParams(t *testing.T) {
	ctx := context.Background()
	boards, err := filepath.Abs(filepath.Join("..", "..", "tools", "samples", "boards"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(boards, "glasgow-revC3", "params")); err != nil {
		t.Fatalf("the Glasgow sample is missing (run `make samples-oracle`): %v", err)
	}
	dir := t.TempDir()
	srv := designServerCapped(t, filepath.Join(dir, "store"), filepath.Join(dir, "uploads"), 1<<28, mountset.Folder("boards", boards))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://boards/glasgow-revC3", Designs: []*webapi.DesignToAdd{{Folder: "glasgow-revC3", DesignId: "glasgow"}}}))
	if err != nil {
		t.Fatal(err)
	}
	if uris := added.Msg.GetDesigns()[0].GetConfig().GetParamUris(); len(uris) != 1 || !strings.HasSuffix(uris[0], "/params") {
		t.Errorf("glasgow's params folders = %v; want the view's params artifact", uris)
	}
	arts, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/glasgow"}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(arts.Msg.GetArtifacts(), func(a *webapi.Artifact) bool {
		return a.GetType() == webapi.ArtifactType_ARTIFACT_TYPE_PARAMS && a.GetName() == "designs/glasgow/artifacts/params" && len(a.GetFiles()) > 0
	}) {
		t.Errorf("glasgow's artifacts = %v; want a params artifact holding the TPD3S014 spec", arts.Msg.GetArtifacts())
	}
}
