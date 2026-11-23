package projects

import (
	"context"
	"slices"
	"testing"
	"testing/fstest"
)

// A tier folder counts as listed when any tier names it or a path under it. A mount:// source lists
// nothing beside the descriptor, and a folder no tier is named for is never noted (agni issue 1861).
func TestUnlistedTiersFindsTheFoldersNoTierNames(t *testing.T) {
	dirs := map[string]bool{"profiles": true, "params": true, "lib": true, "queries": true, "boards": true}
	src := Sources{Symbols: []string{"lib/symbols"}, Params: []string{"mount://corpus/params"}, Queries: []string{"./queries"}}
	got := unlistedTiers(src, func(name string) bool { return dirs[name] })
	if want := []string{"profiles", "params"}; !slices.Equal(got, want) {
		t.Errorf("unlisted = %v, want %v", got, want)
	}
}

// The store tells its listener once about a project.yaml beside an unlisted profiles/, and not
// again on a second read of the unchanged descriptor.
func TestFSStoreNotesAnUnlistedTierFolderOnce(t *testing.T) {
	fsys := fstest.MapFS{
		"team/project.yaml":            {Data: []byte("name: team\nparams: [params]\n")},
		"team/profiles/rail.yaml":      {Data: []byte("x: 1\n")},
		"team/params/TPS62130.yaml":    {Data: []byte("x: 1\n")},
		"team/boards/gw/notes.md":      {Data: []byte("notes\n")},
		"other/project.yaml":           {Data: []byte("name: other\n")},
		"other/boards/gw/design.notes": {Data: []byte("notes\n")},
	}
	var heard []Unlisted
	s := NewFSStore(Tree{Mount: "m", FS: fsys}).OnUnlisted(func(u Unlisted) { heard = append(heard, u) })
	for range 2 {
		if _, err := s.Projects(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(heard) != 1 || heard[0].Descriptor.String() != "mount://m/team/project.yaml" || !slices.Equal(heard[0].Tiers, []string{"profiles"}) {
		t.Fatalf("heard %v; want team/project.yaml with profiles alone, once", heard)
	}
	want := `note: profiles/ sits beside /srv/team/project.yaml, and project.yaml doesn't list it, so this run leaves it out; add "profiles: [profiles]" to project.yaml to read it`
	if got := heard[0].Note("/srv/team/project.yaml"); got != want {
		t.Errorf("note =\n%s\nwant\n%s", got, want)
	}
}
