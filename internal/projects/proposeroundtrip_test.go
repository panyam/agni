package projects

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

// TestProposedDescriptorsReadBackAsTheProposedDesign writes each proposal's design.yaml into its
// folder and resolves the design through the store a server uses. A descriptor the page shows a
// visitor that then reads as some other design would teach them the wrong thing (agni issue 854).
func TestProposedDescriptorsReadBackAsTheProposedDesign(t *testing.T) {
	tree := map[string]string{
		"edif/x-rev-a.edn":      "",
		"edif/x-rev-a.eds":      "",
		"edif/x-rev-b.edn":      "",
		"kicad/board.kicad_pro": "{}",
		"kicad/board.kicad_sch": `(kicad_sch (sheet (property "Sheetfile" "sub/p.kicad_sch")))`,
		"kicad/sub/p.kicad_sch": "(kicad_sch)",
		"kicad/board.kicad_pcb": "",
		"odd name/n: 1.edn":     "",
	}
	var paths []string
	for p := range tree {
		paths = append(paths, p)
	}
	resp := service.ProposeFromFiles("m", paths, func(p string) ([]byte, error) { return []byte(tree[p]), nil })
	if len(resp.GetDesigns()) != 3 {
		t.Fatalf("want 3 proposals, got %v", resp.GetDesigns())
	}
	fsys := fstest.MapFS{}
	for p, b := range tree {
		fsys[p] = &fstest.MapFile{Data: []byte(b)}
	}
	for _, d := range resp.GetDesigns() {
		fsys[strings.TrimPrefix(d.GetFolder()+"/"+DesignDescriptor, "/")] = &fstest.MapFile{Data: []byte(d.GetDesignYaml())}
	}
	store := NewFSStore(Tree{Mount: "m", FS: fsys})
	for _, want := range resp.GetDesigns() {
		u, err := artifact.Parse(want.GetDesign().GetEntryUri())
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := store.ResolveDesign(context.Background(), u)
		if err != nil || got == nil {
			t.Fatalf("%s: the written descriptor does not resolve: %v\n%s", want.GetFolder(), err, want.GetDesignYaml())
		}
		if got.GetEntryUri() != want.GetDesign().GetEntryUri() ||
			strings.Join(got.GetCompanionUris(), ",") != strings.Join(want.GetDesign().GetCompanionUris(), ",") ||
			len(got.GetRevisions()) != len(want.GetDesign().GetRevisions()) {
			t.Errorf("%s: proposed %v, read back %v\n%s", want.GetFolder(), want.GetDesign(), got, want.GetDesignYaml())
		}
	}
}

// TestProposingTheTutorialGatewayRecoversItsDeclaredDesign hides the gateway's design.yaml and asks
// for a proposal over the rest of its folder. The entry and companions must be the ones the
// tutorial declares, since a visitor dropping that folder should get the design the docs describe.
func TestProposingTheTutorialGatewayRecoversItsDeclaredDesign(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "tutorial-project", "designs", "gateway")
	fsys := os.DirFS(dir)
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && p != DesignDescriptor {
			paths = append(paths, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	resp := service.ProposeFromFiles("m", paths, func(p string) ([]byte, error) { return fs.ReadFile(fsys, p) })
	if len(resp.GetDesigns()) != 1 {
		t.Fatalf("want one design, got %v", resp.GetDesigns())
	}
	got := resp.GetDesigns()[0].GetDesign()

	store := NewFSStore(Tree{Mount: "m", FS: fsys})
	declared, _, err := store.ResolveDesign(context.Background(), artifact.URI{Mount: "m", Path: "gateway.edn"})
	if err != nil || declared == nil {
		t.Fatalf("the declared design does not resolve: %v", err)
	}
	if got.GetEntryUri() != declared.GetEntryUri() || strings.Join(got.GetCompanionUris(), ",") != strings.Join(declared.GetCompanionUris(), ",") {
		t.Errorf("proposed entry %s companions %v; the tutorial declares %s and %v",
			got.GetEntryUri(), got.GetCompanionUris(), declared.GetEntryUri(), declared.GetCompanionUris())
	}
	if len(resp.GetUnread()) != 0 {
		t.Errorf("unread %v", resp.GetUnread())
	}
}
