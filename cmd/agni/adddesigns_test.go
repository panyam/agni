package main

import (
	"context"
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
	"github.com/panyam/goapplib/storage/mounts"
)

// twoBoards is a folder holding two boards and no design.yaml: the Jetson cut's KiCad schematic and
// board, and the tutorial gateway's netlist and board.
func twoBoards(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for dir, files := range map[string]map[string]string{
		"boards/cut": {
			"../../examples/cross-probe-project/boards/jetson-cut/jetson-cut.kicad_sch":   "jetson-cut.kicad_sch",
			"../../examples/cross-probe-project/boards/jetson-cut/power.kicad_sch":        "power.kicad_sch",
			"../../examples/cross-probe-project/boards/jetson-cut/usb_debug_pd.kicad_sch": "usb_debug_pd.kicad_sch",
			"../../examples/cross-probe-project/boards/jetson-cut/jetson-cut.kicad_pcb":   "jetson-cut.kicad_pcb",
			"../../examples/cross-probe-project/boards/jetson-cut/jetson-cut.kicad_pro":   "jetson-cut.kicad_pro",
		},
		"boards/gw": {
			gatewayRevA: "gateway.edn",
		},
	} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o750); err != nil {
			t.Fatal(err)
		}
		for from, to := range files {
			b, err := os.ReadFile(from)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, dir, to), b, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// Adding a folder that holds two boards keeps one design per board, each checking as its folder
// does. Nothing is kept until the boards are named, each artifact records where it came from, and
// the source filter finds a board by its folder (agni issue 1736).
func TestAddingAFolderKeepsOneDesignPerBoard(t *testing.T) {
	ctx := context.Background()
	lab := twoBoards(t)
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	count := func() int {
		t.Helper()
		list, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		return len(list.Msg.GetDesigns())
	}

	out, err := runDesigns(t, srv.URL, "add", "lab/boards")
	if err != nil || strings.Count(out, "proposed\t") != 2 || count() != 0 {
		t.Fatalf("designs add with no --yes = %q, %v, %d designs; want two proposals listed and none kept", out, err, count())
	}
	if _, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://lab/boards", Designs: []*webapi.DesignToAdd{{Folder: "boards/cut"}}, ValidateOnly: true})); err != nil || count() != 0 {
		t.Fatalf("AddDesigns validate_only = %v, %d designs; want none kept", err, count())
	}
	if out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--only", "boards/gw", "--series", "lab"); err != nil || count() != 1 {
		t.Fatalf("designs add --only boards/gw = %q, %v, %d designs; want the gateway kept alone", out, err, count())
	}
	if out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--yes"); err == nil {
		t.Fatalf("designs add --yes over a kept gateway = %q; want ALREADY_EXISTS", out)
	}
	if out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--only", "cut"); err != nil || count() != 2 {
		t.Fatalf("designs add --only cut = %q, %v, %d designs; want both kept, each named for its folder", out, err, count())
	}

	for id, folder := range map[string]string{"gw": "boards/gw", "cut": "boards/cut"} {
		arts, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/" + id}))
		if err != nil || len(arts.Msg.GetArtifacts()) == 0 {
			t.Fatalf("artifacts of %s = %v, %v; want some", id, arts, err)
		}
		for _, a := range arts.Msg.GetArtifacts() {
			if src := a.GetSource(); src.GetMount() != "lab" || src.GetPath() != folder {
				t.Errorf("%s's source = %v; want lab at %s", a.GetName(), src, folder)
			}
		}
		stored := findingKeys(t, "check", "designs/"+id, "--remote", srv.URL)
		local := findingKeys(t, "check", filepath.Join(lab, filepath.FromSlash(folder)))
		if len(local) == 0 || !slices.Equal(stored, local) {
			t.Errorf("check designs/%s = %d findings, check on its folder = %d; want the same set\nonly stored: %v\nonly on the folder: %v", id, len(stored), len(local), minus(stored, local), minus(local, stored))
		}
	}
	found, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{Filter: `source = "mount://lab/boards/cut"`}))
	if err != nil || len(found.Msg.GetDesigns()) != 1 || found.Msg.GetDesigns()[0].GetName() != "designs/cut" {
		t.Errorf("designs from mount://lab/boards/cut = %v, %v; want cut alone", found, err)
	}
	if all, err := designs.ListDesigns(ctx, connect.NewRequest(&webapi.ListDesignsRequest{Filter: `source = "mount://lab"`})); err != nil || len(all.Msg.GetDesigns()) != 2 {
		t.Errorf("designs from mount://lab = %v, %v; want both", all, err)
	}
}

// A server with mounts and no --store lists no designs, however many design.yaml files its folders
// hold, and refuses to add one (agni issue 1736).
func TestAServerWithNoStoreListsNoDesigns(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.Abs(filepath.Join("..", "..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	ms := []mounts.Mount{mountset.Folder("tut", root)}
	api := servedAPIFor(t, ms, ms)
	list, err := api.Design.ListDesigns(ctx, &webapi.ListDesignsRequest{})
	if err != nil || len(list.GetDesigns()) != 0 {
		var names []string
		for _, d := range list.GetDesigns() {
			names = append(names, d.GetName())
		}
		t.Errorf("ListDesigns with mounts and no store = %v, %v; want none", names, err)
	}
	if _, err := api.Design.GetDesign(ctx, &webapi.GetDesignRequest{Name: "designs/gateway"}); err == nil {
		t.Error("GetDesign of a folder's design.yaml name answered; want NOT_FOUND")
	}
	if _, err := api.Design.AddDesigns(ctx, &webapi.AddDesignsRequest{Uri: "mount://tut/boards/gateway", Designs: []*webapi.DesignToAdd{{Folder: "boards/gateway"}}}); err == nil {
		t.Error("AddDesigns with no store succeeded; want FAILED_PRECONDITION")
	}
}

// Two releases of one board share an entry name in two folders, and --yes keeps both, each named for
// its folder (agni issue 1736).
func TestTwoReleasesOfOneBoardGetTwoIDs(t *testing.T) {
	lab := twoBoards(t)
	b, err := os.ReadFile(gatewayRevA)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(lab, "boards", "gw-b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lab, "boards", "gw-b", "gateway.edn"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	out, err := runDesigns(t, srv.URL, "add", "lab/boards", "--yes", "--series", "lab")
	if err != nil {
		t.Fatalf("designs add --yes = %q, %v; want all three kept", out, err)
	}
	if n := strings.Count(out, "latest"); n != 1 {
		t.Errorf("designs add --yes printed %q; want one design of the series marked latest, not %d", out, n)
	}
	for _, id := range []string{"designs/cut", "designs/gw", "designs/gw-b"} {
		if !strings.Contains(out, id+"\t") {
			t.Errorf("designs add --yes printed %q; want %s", out, id)
		}
	}
}

// A folder holding two boards proposes both, and adding the folder alone would be a guess. The
// request names the board by its entry, and a request naming only the folder is refused with both
// entries listed (agni issue 1736).
func TestTwoBoardsInOneFolderAreAddedByEntry(t *testing.T) {
	ctx := context.Background()
	lab := t.TempDir()
	for from, to := range map[string]string{gatewayRevA: "gateway.edn", gatewayRevB: "spare.edn"} {
		b, err := os.ReadFile(from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(lab, to), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	designs := webapiconnect.NewDesignServiceClient(http.DefaultClient, srv.URL)
	proposed, err := webapiconnect.NewWorkspaceServiceClient(http.DefaultClient, srv.URL).ProposeDesigns(ctx, connect.NewRequest(&webapi.ProposeDesignsRequest{Uri: "mount://lab"}))
	if err != nil || len(proposed.Msg.GetDesigns()) != 2 {
		t.Fatalf("ProposeDesigns = %v, %v; want two proposals", proposed, err)
	}
	var spare string
	for _, p := range proposed.Msg.GetDesigns() {
		if strings.HasSuffix(p.GetDesign().GetEntryUri(), "/spare.edn") {
			spare = p.GetDesign().GetEntryUri()
		}
	}
	if spare == "" {
		t.Fatalf("ProposeDesigns = %v; want one entry at spare.edn", proposed.Msg.GetDesigns())
	}

	_, err = designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://lab", Designs: []*webapi.DesignToAdd{{Folder: ""}}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "spare.edn") || !strings.Contains(err.Error(), "gateway.edn") {
		t.Fatalf("AddDesigns naming only a shared folder = %v; want INVALID_ARGUMENT naming both entries", err)
	}
	added, err := designs.AddDesigns(ctx, connect.NewRequest(&webapi.AddDesignsRequest{Uri: "mount://lab", Designs: []*webapi.DesignToAdd{{Folder: "", Entry: spare}}}))
	if err != nil || len(added.Msg.GetDesigns()) != 1 || added.Msg.GetDesigns()[0].GetName() != "designs/spare" {
		t.Fatalf("AddDesigns naming spare.edn = %v, %v; want designs/spare", added, err)
	}
	arts, err := designs.ListArtifacts(ctx, connect.NewRequest(&webapi.ListArtifactsRequest{Parent: "designs/spare"}))
	if err != nil || len(arts.Msg.GetArtifacts()) != 1 {
		t.Fatalf("ListArtifacts(designs/spare) = %v, %v; want one artifact", arts, err)
	}
	if files := arts.Msg.GetArtifacts()[0].GetFiles(); len(files) != 1 || !strings.HasSuffix(files[0].GetPath(), "spare.edn") {
		t.Fatalf("designs/spare's files = %v; want spare.edn alone", files)
	}
}

// Boards of one name in two folders add with ids that differ, and --only can give one an id of its
// own by its entry (agni issue 1756).
func TestBoardsOfOneNameInTwoFoldersAddWithoutAClash(t *testing.T) {
	lab := t.TempDir()
	for _, dir := range []string{"rev-a", "rev-b"} {
		for from, to := range map[string]string{gatewayRevA: "gateway.edn", gatewayRevB: "spare.edn"} {
			b, err := os.ReadFile(from)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(lab, dir), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(lab, dir, to), b, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	srv := designServer(t, t.TempDir(), t.TempDir(), mountset.Folder("lab", lab))
	out, err := runDesigns(t, srv.URL, "add", "lab", "--only", "rev-a-gateway", "--only", "rev-b/gateway.edn=gw-b")
	if err != nil {
		t.Fatalf("designs add --only = %q, %v", out, err)
	}
	for _, id := range []string{"designs/rev-a-gateway\t", "designs/gw-b\t"} {
		if !strings.Contains(out, id) {
			t.Errorf("designs add printed %q; want %s", out, id)
		}
	}
	if !strings.Contains(out, "proposed\trev-b\trev-b-spare\t") {
		t.Errorf("designs add printed %q; want rev-b's spare left as a proposal with its own id", out)
	}
}
