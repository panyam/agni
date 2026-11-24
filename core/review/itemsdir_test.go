package review

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

const inlineTwin = `name: Power review
areas:
  - name: power
    items:
      - {id: "P1", title: bulk caps, rule: bulk-cap}
      - {id: "P2", title: decoupling, rule: decoupling-present}
`

func itemsFS(manifest string) fstest.MapFS {
	return fstest.MapFS{
		"inline.yaml":                      {Data: []byte(inlineTwin)},
		"reviews/power.yaml":               {Data: []byte(manifest)},
		"reviews/checklist/01-power/a":     {Data: []byte("not an item")},
		"reviews/checklist/01-power/p2.md": {Data: []byte("---\nid: P2\ntitle: decoupling\nrule: decoupling-present\n---\nnotes\n")},
		"reviews/checklist/01-power/p1.md": {Data: []byte("---\nid: P1\ntitle: bulk caps\nrule: bulk-cap\n---\n")},
		"reviews/checklist/README.md":      {Data: []byte("how we review\n")},
	}
}

// A manifest file naming items_dir reads the folder beside it, and the folder itself reads as a
// checklist titled with its name, each the same as the inline manifest (agni issue 1836).
func TestLoadFSReadsAManifestNamingItemsDirAndABareFolder(t *testing.T) {
	fsys := itemsFS("name: Power review\nitems_dir: checklist\n")
	want, err := LoadFS(fsys, "inline.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadFS(fsys, "reviews/power.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a manifest naming items_dir reads as\n%+v\nwant its inline twin\n%+v", got, want)
	}
	folder, err := LoadFS(fsys, "reviews/checklist")
	if err != nil {
		t.Fatal(err)
	}
	want.Name = "checklist"
	if !reflect.DeepEqual(folder, want) {
		t.Errorf("the bare folder reads as\n%+v\nwant the same items titled checklist\n%+v", folder, want)
	}
}

// A manifest naming items_dir with areas beside it, a folder outside the manifest's own, a missing
// folder, or a broken item file is refused, and a broken file is named (agni issue 1836).
func TestLoadFSRefusesABadItemsDir(t *testing.T) {
	for label, c := range map[string]struct{ manifest, want string }{
		"areas beside it":  {"name: R\nitems_dir: checklist\nareas: []\n", "remove areas"},
		"outside":          {"name: R\nitems_dir: ../elsewhere\n", "under the manifest's own folder"},
		"empty":            {"name: R\nitems_dir: \"\"\n", "under the manifest's own folder"},
		"a missing folder": {"name: R\nitems_dir: missing\n", "items_dir reviews/missing"},
		"no name":          {"items_dir: checklist\n", "missing required field \"name\""},
	} {
		if _, err := LoadFS(itemsFS(c.manifest), "reviews/power.yaml"); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v; want an error containing %q", label, err, c.want)
		}
	}
	fsys := itemsFS("name: R\nitems_dir: checklist\n")
	fsys["reviews/checklist/01-power/p3.md"] = &fstest.MapFile{Data: []byte("---\nid: P3\ntitel: typo\n---\n")}
	if _, err := LoadFS(fsys, "reviews/power.yaml"); err == nil || !strings.Contains(err.Error(), "reviews/checklist/01-power/p3.md: frontmatter") {
		t.Errorf("a broken item file = %v; want the error to name reviews/checklist/01-power/p3.md", err)
	}
}

// An item file may carry the team's own metadata beside its binding. The review ignores it, and the
// checklist reads as its inline twin. A key that looks like a typo of a known field still fails and
// names that field, and items within an area run in the natural order of their ids (agni issue 1888).
func TestItemFilesKeepTeamMetadataAndSortIdsNaturally(t *testing.T) {
	fsys := itemsFS("name: Power review\nitems_dir: checklist\n")
	fsys["reviews/checklist/01-power/p1.md"] = &fstest.MapFile{Data: []byte("---\nid: P1\ntitle: bulk caps\nrule: bulk-cap\nowner: hw-team\nlast_triaged: 2026-11-02\nneeds: [datasheet]\n---\n")}
	want, err := LoadFS(fsys, "inline.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadFS(fsys, "reviews/power.yaml")
	if err != nil {
		t.Fatalf("an item file with owner, last_triaged and needs: %v; want it read, the metadata ignored", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("with team metadata the checklist reads as\n%+v\nwant its inline twin\n%+v", got, want)
	}

	fsys["reviews/checklist/01-power/p1.md"] = &fstest.MapFile{Data: []byte("---\nid: P1\ntitle: bulk caps\nrul: bulk-cap\n---\n")}
	if _, err := LoadFS(fsys, "reviews/power.yaml"); err == nil || !strings.Contains(err.Error(), `rul:, did you mean "rule"`) {
		t.Errorf("rul: in place of rule: = %v; want it refused, naming rule", err)
	}

	numbered := fstest.MapFS{}
	for _, id := range []string{"100", "16", "2"} {
		numbered["checklist/power/"+id+".md"] = &fstest.MapFile{Data: []byte("---\nid: \"" + id + "\"\ntitle: item " + id + "\nrule: bulk-cap\n---\n")}
	}
	man, err := LoadFS(numbered, "checklist")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range man.Areas[0].Items {
		ids = append(ids, it.ID)
	}
	if !reflect.DeepEqual(ids, []string{"2", "16", "100"}) {
		t.Errorf("items 100, 16 and 2 run as %v; want 2, 16, 100", ids)
	}
}
