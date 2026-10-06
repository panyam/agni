package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/service"
)

// propose runs the rules over an in-memory tree.
func propose(t *testing.T, files map[string]string) *webapi.ProposeDesignsResponse {
	t.Helper()
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	return service.ProposeFromFiles("m", paths, func(p string) ([]byte, error) {
		b, ok := files[p]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(b), nil
	})
}

func onlyDesign(t *testing.T, r *webapi.ProposeDesignsResponse) *webapi.ProposedDesign {
	t.Helper()
	if len(r.GetDesigns()) != 1 {
		t.Fatalf("want one design, got %d: %v", len(r.GetDesigns()), r)
	}
	return r.GetDesigns()[0]
}

func unread(r *webapi.ProposeDesignsResponse) map[string]string {
	out := map[string]string{}
	for _, u := range r.GetUnread() {
		out[u.GetPath()] = u.GetReason()
	}
	return out
}

const childRef = `(kicad_sch (sheet (property "Sheetname" "Power") (property "Sheetfile" "sub/power.kicad_sch")))`

func TestProposeKicadProjectClaimsItsSheetsBoardAndLibraries(t *testing.T) {
	r := propose(t, map[string]string{
		"b/board.kicad_pro":     "{}",
		"b/board.kicad_sch":     childRef,
		"b/sub/power.kicad_sch": "(kicad_sch)",
		"b/board.kicad_pcb":     "(kicad_pcb)",
		"b/sym-lib-table":       "",
		"b/lib/parts.kicad_sym": "",
	})
	d := onlyDesign(t, r)
	if d.GetDesign().GetEntryUri() != "mount://m/b/board.kicad_sch" || strings.Join(d.GetDesign().GetCompanionUris(), ",") != "mount://m/b/board.kicad_pcb" {
		t.Errorf("entry %s companions %v", d.GetDesign().GetEntryUri(), d.GetDesign().GetCompanionUris())
	}
	if got := strings.Join(d.GetFiles(), ","); got != "b/board.kicad_pcb,b/board.kicad_pro,b/board.kicad_sch,b/lib/parts.kicad_sym,b/sub/power.kicad_sch,b/sym-lib-table" {
		t.Errorf("files %s", got)
	}
	if len(r.GetUnread()) != 0 {
		t.Errorf("unread %v", r.GetUnread())
	}
}

func TestProposeKicadRootSheetWithoutAProject(t *testing.T) {
	r := propose(t, map[string]string{"root.kicad_sch": childRef, "sub/power.kicad_sch": "(kicad_sch)"})
	d := onlyDesign(t, r)
	if d.GetDesign().GetEntryUri() != "mount://m/root.kicad_sch" {
		t.Errorf("the child sheet was taken for a root: %v", d)
	}
}

func TestProposeEdifPairsANetlistWithItsSchematic(t *testing.T) {
	d := onlyDesign(t, propose(t, map[string]string{"x.edn": "", "x.eds": ""}))
	if d.GetDesign().GetEntryUri() != "mount://m/x.edn" || strings.Join(d.GetDesign().GetCompanionUris(), ",") != "mount://m/x.eds" {
		t.Errorf("%v", d)
	}
}

// Two netlists in one drop are two revisions of one design, never merged into one read.
func TestProposeTwoNetlistsAreRevisionsNotOneDesign(t *testing.T) {
	d := onlyDesign(t, propose(t, map[string]string{"x-rev-a.edn": "", "x-rev-a.eds": "", "x-rev-b.edn": ""}))
	if d.GetDesign().GetEntryUri() != "mount://m/x-rev-a.edn" {
		t.Errorf("entry %s, want the netlist with a schematic beside it", d.GetDesign().GetEntryUri())
	}
	revs := d.GetDesign().GetRevisions()
	if len(revs) != 1 || revs[0].GetEntryUri() != "mount://m/x-rev-b.edn" || len(revs[0].GetCompanionUris()) != 0 {
		t.Errorf("revisions %v", revs)
	}
	if !strings.Contains(d.GetDesignYaml(), "revisions:\n  - entry: x-rev-b.edn") || !strings.Contains(d.GetNote(), "revisions") {
		t.Errorf("descriptor or note does not say so:\n%s\n%s", d.GetDesignYaml(), d.GetNote())
	}
}

func TestProposeLeavesAStrayBoardAndOtherFilesUnreadWithAReason(t *testing.T) {
	r := propose(t, map[string]string{"x.edn": "", "stray.kicad_pcb": "", "README.md": "", "lone.kicad_sym": ""})
	got := unread(r)
	if !strings.Contains(got["stray.kicad_pcb"], "no schematic or netlist named stray") {
		t.Errorf("stray board: %q", got["stray.kicad_pcb"])
	}
	if got["README.md"] != "not a design file" {
		t.Errorf("README: %q", got["README.md"])
	}
	if !strings.Contains(got["lone.kicad_sym"], "KiCad library") {
		t.Errorf("library: %q", got["lone.kicad_sym"])
	}
}

func TestProposeALoneSchematicExportSaysItIsNotANetlist(t *testing.T) {
	d := onlyDesign(t, propose(t, map[string]string{"s.eds": ""}))
	if !strings.Contains(d.GetNote(), "not a netlist") {
		t.Errorf("note %q", d.GetNote())
	}
}

func TestProposeReportsADeclaredFolderRatherThanGuessing(t *testing.T) {
	r := propose(t, map[string]string{"d/design.yaml": "name: d\nentry: q.edn\n", "d/q.edn": "", "d/q-rev.edn": ""})
	d := onlyDesign(t, r)
	if !d.GetDeclared() || d.GetDesignYaml() != "name: d\nentry: q.edn\n" || len(d.GetFiles()) != 3 {
		t.Errorf("%v", d)
	}
}

func TestProposeCountsAProjectsConfigAsSupport(t *testing.T) {
	r := propose(t, map[string]string{"project.yaml": "name: p", "profiles/can.yaml": "", "params/a.textproto": "", "designs/g/g.edn": ""})
	sort.Strings(r.Support)
	if strings.Join(r.GetSupport(), ",") != "params/a.textproto,profiles/can.yaml,project.yaml" || len(r.GetUnread()) != 0 {
		t.Errorf("support %v unread %v", r.GetSupport(), r.GetUnread())
	}
}

func TestProposeNotesASecondDesignInOneFolder(t *testing.T) {
	r := propose(t, map[string]string{"a.edn": "", "b.kicad_sch": "(kicad_sch)"})
	if len(r.GetDesigns()) != 2 || r.GetDesigns()[0].GetNote() != "" || !strings.Contains(r.GetDesigns()[1].GetNote(), "shares this folder") {
		t.Errorf("%v", r.GetDesigns())
	}
}

// A zip reads as the folder it was made from, so its proposal is the folder's under the zip's name.
func TestProposeAZipAsItsFolder(t *testing.T) {
	folder := map[string]string{"board.kicad_pro": "{}", "board.kicad_sch": childRef, "sub/power.kicad_sch": "(kicad_sch)", "board.kicad_pcb": ""}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range folder {
		f, _ := w.Create(name)
		f.Write([]byte(body))
	}
	w.Close()
	host := fshost.New(fshost.Mount{Name: "local", FS: fshost.ExpandZips(fshost.MemFS(map[string][]byte{"drop/board.zip": buf.Bytes()}))})
	svc := service.NewWorkspaceService(host.Workspace()).WithDesignFiles(nil, host.Workspace())
	r, err := svc.ProposeDesigns(context.Background(), &webapi.ProposeDesignsRequest{Uri: "mount://local/drop"})
	if err != nil {
		t.Fatal(err)
	}
	d := onlyDesign(t, r)
	if d.GetDesign().GetEntryUri() != "mount://local/drop/board.zip/board.kicad_sch" || len(d.GetFiles()) != 4 || len(r.GetUnread()) != 0 {
		t.Errorf("%v unread %v", d, r.GetUnread())
	}
}

// withoutDescriptors hides every design.yaml, so a folder that declares itself reads as one dropped
// without a descriptor.
type withoutDescriptors struct{ fs.FS }

func (w withoutDescriptors) Open(name string) (fs.File, error) {
	if filepath.Base(name) == "design.yaml" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return w.FS.Open(name)
}

func (w withoutDescriptors) ReadDir(name string) ([]fs.DirEntry, error) {
	all, err := fs.ReadDir(w.FS, name)
	out := all[:0]
	for _, e := range all {
		if e.Name() != "design.yaml" {
			out = append(out, e)
		}
	}
	return out, err
}

// TestProposeARealKicadBoard runs the rules over a fetched sample board with a hierarchy of sheets in
// a subfolder, so the sheet walk meets a real file rather than a fixture written for it. The corpus
// declares the board since agni-samples v0.1.2, and its descriptors are hidden here because the
// proposal rules are what a visitor's dropped copy of the folder meets.
func TestProposeARealKicadBoard(t *testing.T) {
	dir := filepath.Join("..", "tools", "samples", "boards", "royalblue54L-feather")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the samples corpus is missing (make samples): %v", err)
	}
	host := fshost.New(fshost.Mount{Name: "s", FS: withoutDescriptors{os.DirFS(dir)}})
	svc := service.NewWorkspaceService(host.Workspace()).WithDesignFiles(nil, host.Workspace())
	r, err := svc.ProposeDesigns(context.Background(), &webapi.ProposeDesignsRequest{Uri: "mount://s"})
	if err != nil {
		t.Fatal(err)
	}
	var main *webapi.ProposedDesign
	for _, d := range r.GetDesigns() {
		if d.GetDesign().GetEntryUri() == "mount://s/RoyalBlue54L-Feather.kicad_sch" {
			main = d
		}
	}
	if main == nil {
		t.Fatalf("no design entered at the root sheet: %v", r.GetDesigns())
	}
	sheets := 0
	for _, f := range main.GetFiles() {
		if strings.HasPrefix(f, "sch/") && strings.HasSuffix(f, ".kicad_sch") {
			sheets++
		}
	}
	if sheets == 0 || strings.Join(main.GetDesign().GetCompanionUris(), ",") != "mount://s/RoyalBlue54L-Feather.kicad_pcb" {
		t.Errorf("child sheets %d, companions %v", sheets, main.GetDesign().GetCompanionUris())
	}
	for _, u := range r.GetUnread() {
		if strings.HasSuffix(u.GetPath(), ".kicad_sch") {
			t.Errorf("a sheet was left unread: %v", u)
		}
	}
}
