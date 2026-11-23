package projects

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/panyam/agni/service"
)

// The same review, inline and as a folder of item files, so a test can compare the two.
const inlineReview = `checklists:
  review:
    name: Schematic review
    areas:
      - name: system-architecture
        items:
          - {id: "001", title: block diagram, note: drawn by hand}
          - {id: "002", title: interfaces documented, rule: decoupling-present}
      - name: Power
        items:
          - {id: "015", title: bulk caps, rule: bulk-cap}
          - {id: "016", title: rail naming, profile: usb2}
`

var reviewItems = map[string]string{
	"checklist/01-system-architecture/002-interfaces.md": "---\nid: \"002\"\ntitle: interfaces documented\nrule: decoupling-present\n---\nWho owns each interface.\n",
	"checklist/01-system-architecture/001-block.md":      "---\nid: \"001\"\ntitle: block diagram\nnote: drawn by hand\n---\n",
	"checklist/02-power/016-rails.md":                    "---\nid: \"016\"\ntitle: rail naming\nprofile: usb2\narea: Power\n---\n",
	"checklist/02-power/015-bulk.md":                     "---\nid: \"015\"\ntitle: bulk caps\nrule: bulk-cap\narea: Power\n---\n",
	"checklist/README.md":                                "How we review schematics.\n",
}

const folderReview = "checklists:\n  review:\n    name: Schematic review\n    items_dir: checklist/\n"

// A checklist written as a folder of item files reads as the same checklist written inline, through
// a project in a mount and through a pack's files (agni issue 1812). Its item files go with the pack.
func TestAFolderChecklistReadsAsItsInlineTwin(t *testing.T) {
	inline := NewFSStore(Tree{Mount: "m", FS: mapFS(map[string]string{"project.yaml": "name: p\n" + inlineReview})})
	want, err := inline.Project(context.Background(), "projects/p")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"project.yaml": "name: p\n" + folderReview}
	for k, v := range reviewItems {
		files[k] = v
	}
	folder := NewFSStore(Tree{Mount: "m", FS: mapFS(files)})
	got, err := folder.Project(context.Background(), "projects/p")
	if err != nil {
		t.Fatal(err)
	}
	wantMan, gotMan := want.GetConfig().GetChecklists()[0], got.GetConfig().GetChecklists()[0]
	if !proto.Equal(wantMan, gotMan) {
		t.Errorf("the folder checklist reads as\n%v\nwant its inline twin\n%v", gotMan, wantMan)
	}

	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	_, pack, kept, err := PackFromFiles(paths, func(p string) ([]byte, error) {
		b, ok := files[p]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(b), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(pack.GetConfig().GetChecklists()[0], wantMan) {
		t.Errorf("the pack's checklist reads as\n%v\nwant its inline twin\n%v", pack.GetConfig().GetChecklists()[0], wantMan)
	}
	wantKept := []string{"checklist/01-system-architecture/001-block.md", "checklist/01-system-architecture/002-interfaces.md", "checklist/02-power/015-bulk.md", "checklist/02-power/016-rails.md"}
	if !slices.Equal(kept, wantKept) {
		t.Errorf("the pack keeps %v; want the four item files, so a server reading the pushed files reads the same checklist", kept)
	}
}

// A broken item file fails the read and names the file, and a checklist naming items_dir with areas
// beside it, or a folder outside the project, is refused (agni issue 1812).
func TestAFolderChecklistNamesItsBrokenFile(t *testing.T) {
	for label, c := range map[string]struct {
		descriptor string
		items      map[string]string
		want       string
	}{
		"no frontmatter":   {folderReview, map[string]string{"checklist/a/1.md": "id: x\n"}, "checklist/a/1.md: has no frontmatter"},
		"unclosed":         {folderReview, map[string]string{"checklist/a/1.md": "---\nid: x\n"}, "checklist/a/1.md: its frontmatter has no closing"},
		"unknown field":    {folderReview, map[string]string{"checklist/a/1.md": "---\nid: x\ntitel: y\n---\n"}, "checklist/a/1.md: frontmatter"},
		"two bindings":     {folderReview, map[string]string{"checklist/a/1.md": "---\nid: x\nrule: a\nprofile: b\n---\n"}, "checklist/a/1.md: review manifest item \"x\": declares more than one binding"},
		"no area":          {folderReview, map[string]string{"checklist/1.md": "---\nid: x\n---\n"}, "checklist/1.md: an item at the top of items_dir needs area:"},
		"a shared id":      {folderReview, map[string]string{"checklist/a/1.md": "---\nid: x\n---\n", "checklist/b/2.md": "---\nid: x\n---\n"}, "checklist/a/1.md and checklist/b/2.md both hold item \"x\""},
		"no item files":    {folderReview, map[string]string{"checklist/notes.txt": "x"}, "holds no item files"},
		"areas beside it":  {"checklists:\n  review:\n    name: R\n    items_dir: checklist\n    areas: []\n", nil, "names items_dir, so its items come from that folder; remove areas"},
		"outside":          {"checklists:\n  review:\n    name: R\n    items_dir: ../shared\n", nil, "must stay inside the design folder"},
		"a missing folder": {folderReview, nil, "items_dir checklist"},
	} {
		files := map[string]string{"project.yaml": "name: p\n" + c.descriptor}
		for k, v := range c.items {
			files[k] = v
		}
		_, err := NewFSStore(Tree{Mount: "m", FS: mapFS(files)}).Project(context.Background(), "projects/p")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v; want an error containing %q", label, err, c.want)
		}
		var paths []string
		for p := range files {
			paths = append(paths, p)
		}
		_, _, _, perr := PackFromFiles(paths, func(p string) ([]byte, error) { return []byte(files[p]), nil })
		if !errors.Is(perr, service.ErrInvalidArgument) {
			t.Errorf("%s: a pack = %v; want ErrInvalidArgument", label, perr)
		}
	}
}

// Editing, adding or removing an item file changes the checklist the store serves, with no restart
// (agni issue 1812).
func TestEditingAnItemFileRereadsTheChecklist(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "project.yaml", "name: p\n"+folderReview)
	writeFile(t, root, "checklist/power/1.md", "---\nid: \"1\"\ntitle: bulk caps\n---\n")
	s := NewFSStore(Tree{Mount: "m", FS: os.DirFS(root)})
	titles := func() []string {
		t.Helper()
		p, err := s.Project(context.Background(), "projects/p")
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, a := range p.GetConfig().GetChecklists()[0].GetManifest().GetAreas() {
			for _, it := range a.GetItems() {
				out = append(out, a.GetName()+"/"+it.GetTitle())
			}
		}
		return out
	}
	if got := titles(); !slices.Equal(got, []string{"power/bulk caps"}) {
		t.Fatalf("checklist = %v; want the one item", got)
	}
	writeFile(t, root, "checklist/power/1.md", "---\nid: \"1\"\ntitle: bulk capacitance\n---\n")
	if got := titles(); !slices.Equal(got, []string{"power/bulk capacitance"}) {
		t.Errorf("after an edit, checklist = %v; want the new title", got)
	}
	writeFile(t, root, "checklist/power/2.md", "---\nid: \"2\"\ntitle: decoupling\n---\n")
	if got := titles(); !slices.Equal(got, []string{"power/bulk capacitance", "power/decoupling"}) {
		t.Errorf("after adding an item, checklist = %v; want both", got)
	}
}

// The jetson-project example's power checklist, kept as a folder, reads as the Power area of its
// inline review checklist (agni issue 1812).
func TestTheJetsonExamplesFolderChecklistMatchesItsInlineReview(t *testing.T) {
	s := NewFSStore(Tree{Mount: "m", FS: os.DirFS("../../examples/jetson-project")})
	p, err := s.Project(context.Background(), "projects/jetson")
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, c := range p.GetConfig().GetChecklists() {
		byName[c.GetName()] = i
	}
	ri, ok1 := byName["review"]
	pi, ok2 := byName["power"]
	if !ok1 || !ok2 {
		t.Fatalf("the example's checklists are %v; want review and power", byName)
	}
	review := p.GetConfig().GetChecklists()[ri].GetManifest().GetAreas()[0]
	power := p.GetConfig().GetChecklists()[pi].GetManifest().GetAreas()
	if len(power) != 1 || power[0].GetName() != "power" || len(power[0].GetItems()) != 2 {
		t.Fatalf("the power checklist reads as %v; want one area, power, with two items", power)
	}
	for i, it := range power[0].GetItems() {
		if !proto.Equal(it, review.GetItems()[i]) {
			t.Errorf("power item %d = %v; want review's %v", i, it, review.GetItems()[i])
		}
	}
}
