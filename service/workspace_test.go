package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/artifact"
	"strings"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// memWorkspace is an in-memory Workspace: it proves WorkspaceService runs with no os (C13). It
// keys entries by "mount\x00rel", rejects a rel containing ".." as ErrInvalidPath (mimicking
// containment), and returns a plain error for a missing dir (the service maps that to NotFound).
type memWorkspace struct {
	mounts  []MountInfo
	entries map[string][]DirEntry
}

func (m *memWorkspace) Mounts() []MountInfo { return m.mounts }

func (m *memWorkspace) ListDir(_ context.Context, uri artifact.URI) ([]DirEntry, error) {
	if strings.Contains(uri.Path, "..") {
		return nil, fmt.Errorf("%w: %q", ErrInvalidPath, uri.Path)
	}
	e, ok := m.entries[uri.Mount+"\x00"+uri.Path]
	if !ok {
		return nil, fmt.Errorf("no such dir %q", uri.Path)
	}
	return e, nil
}

func TestWorkspaceServiceListDirOverMemPort(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "m", Root: "/x"}},
		entries: map[string][]DirEntry{
			"m\x00": {
				{Name: "notes.txt"},
				{Name: "board.edn"},
				{Name: "sub", IsDir: true},
				{Name: ".hidden.edn"}, // dotfile, skipped
			},
		},
	})
	req := func(path string) *webapi.ListDirRequest {
		return &webapi.ListDirRequest{Uri: uriStr("m", path)}
	}

	t.Run("dir first, files sorted, dotfile skipped, format labeled", func(t *testing.T) {
		resp, err := svc.ListDir(context.Background(), req(""))
		if err != nil {
			t.Fatal(err)
		}
		got := resp.GetEntries()
		if len(got) != 3 {
			t.Fatalf("want 3 (sub, board.edn, notes.txt), got %d: %+v", len(got), got)
		}
		if !got[0].GetIsDir() || got[0].GetName() != "sub" {
			t.Errorf("dir should sort first: %+v", got[0])
		}
		if got[1].GetName() != "board.edn" || got[1].GetFormat() != "edif" {
			t.Errorf("want board.edn/edif: %+v", got[1])
		}
		if got[2].GetName() != "notes.txt" || got[2].GetFormat() != "" {
			t.Errorf("want notes.txt with empty format: %+v", got[2])
		}
	})

	// Containment is enforced when the URI is PARSED, not when the adapter joins it, so an escaping
	// ref classifies as an invalid argument. The value is sent raw because building it through the
	// URI constructor is precisely what is now impossible.
	t.Run("containment is enforced at the parse", func(t *testing.T) {
		_, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: "mount://m/../x"})
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("want ErrInvalidArgument, got %v", err)
		}
	})

	t.Run("missing dir classifies as not-found", func(t *testing.T) {
		if _, err := svc.ListDir(context.Background(), req("nope")); !errors.Is(err, ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

// A browser can only ever show a folder holding nothing it opens as empty, so Opens leaves it out.
// "Empty" is measured over the whole subtree, not one level: a folder of folders of library files is
// as useless to open as a folder with nothing in it. And it is measured against the CALLER's kinds,
// which is why the same fixture prunes differently for the two trees below.
func TestWorkspaceServiceListDirPrunesEmptyDirs(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "m", Root: "/x"}},
		entries: map[string][]DirEntry{
			"m\x00": {
				{Name: "boards", IsDir: true}, // design lives two levels down
				{Name: "docs", IsDir: true},   // a datasheet, which no design reader opens
				{Name: "empty", IsDir: true},  // nothing at all
				{Name: "libs", IsDir: true},   // files, but none a reader opens
			},
			"m\x00boards":      {{Name: "rev2", IsDir: true}},
			"m\x00boards/rev2": {{Name: "board.edn"}},
			"m\x00docs":        {{Name: "txb0104.pdf"}},
			"m\x00empty":       {},
			"m\x00libs":        {{Name: "parts.lock"}, {Name: "nested", IsDir: true}},
			"m\x00libs/nested": {{Name: "readme.md"}, {Name: ".hidden.edn"}}, // dotfile does not count
		},
	})
	names := func(opens ...webapi.FileKind) []string {
		t.Helper()
		resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), Opens: opens})
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range resp.GetEntries() {
			out = append(out, e.GetName())
		}
		return out
	}

	t.Run("prunes dirs with no design anywhere beneath", func(t *testing.T) {
		if got := names(webapi.FileKind_FILE_KIND_DESIGN); len(got) != 1 || got[0] != "boards" {
			t.Fatalf("entries = %v, want [boards] (empty and libs pruned)", got)
		}
	})

	// "Empty" is per-client, which is what Opens states: the datasheets tree opens the PDF under
	// docs/, so for THAT client docs/ is the folder with something in it and boards/ is the empty
	// one. Same tree, same walk, opposite answer.
	t.Run("prunes by the caller's kinds, not by design format", func(t *testing.T) {
		if got := names(webapi.FileKind_FILE_KIND_DATASHEET); len(got) != 1 || got[0] != "docs" {
			t.Fatalf("entries = %v, want [docs] for a datasheet client", got)
		}
	})

	t.Run("a client that opens both keeps both", func(t *testing.T) {
		got := names(webapi.FileKind_FILE_KIND_DESIGN, webapi.FileKind_FILE_KIND_DATASHEET)
		if len(got) != 2 || got[0] != "boards" || got[1] != "docs" {
			t.Fatalf("entries = %v, want [boards docs]", got)
		}
	})

	t.Run("prunes nothing when the caller declares nothing", func(t *testing.T) {
		if got := names(); len(got) != 4 {
			t.Fatalf("entries = %v, want all 4 dirs when Opens is empty", got)
		}
	})

	// UNSPECIFIED is not a kind a client can open. Honouring it would make every folder of lock
	// files look openable, which is the opposite of what asking to prune means.
	t.Run("an unspecified kind prunes nothing rather than everything", func(t *testing.T) {
		if got := names(webapi.FileKind_FILE_KIND_UNSPECIFIED); len(got) != 4 {
			t.Fatalf("entries = %v, want all 4 dirs", got)
		}
	})
}

// A subtree the walk cannot settle keeps its folder: a folder wrongly shown costs a click, one
// wrongly hidden costs a design. Here the design sits below the depth bound, so the walk runs out
// before finding it and the folder stays.
func TestWorkspaceServiceListDirKeepsDirsBeyondPruneDepth(t *testing.T) {
	entries := map[string][]DirEntry{"m\x00": {{Name: "deep", IsDir: true}}}
	path := "deep"
	for range pruneMaxDepth + 2 {
		next := path + "/d"
		entries["m\x00"+path] = []DirEntry{{Name: "d", IsDir: true}}
		path = next
	}
	entries["m\x00"+path] = []DirEntry{{Name: "board.edn"}}

	svc := NewWorkspaceService(&memWorkspace{mounts: []MountInfo{{Name: "m", Root: "/x"}}, entries: entries})
	resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DESIGN}})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetEntries(); len(got) != 1 || got[0].GetName() != "deep" {
		t.Fatalf("entries = %+v, want deep kept: an unfinished walk shows the folder", got)
	}
}

// A mount is a root the same rule applies to: one serving only datasheets or only library files is
// somewhere the design tree can never show anything, so PruneEmptyMounts leaves it out. The count
// comes back so the sidebar can account for the absence instead of quietly being shorter.
func TestListMountsPrunesEmptyMounts(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "boards", Root: "/boards"}, {Name: "ds", Root: "/ds"}, {Name: "empty", Root: "/empty"}},
		entries: map[string][]DirEntry{
			"boards\x00":     {{Name: "rev2", IsDir: true}},
			"boards\x00rev2": {{Name: "board.edn"}},
			"ds\x00":         {{Name: "txb0104.pdf"}}, // a datasheet is not a design
			"empty\x00":      {},
		},
	})

	t.Run("prunes mounts with no design and counts them", func(t *testing.T) {
		resp, err := svc.ListMounts(context.Background(), &webapi.ListMountsRequest{Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DESIGN}})
		if err != nil {
			t.Fatal(err)
		}
		got := resp.GetMounts()
		if len(got) != 1 || got[0].GetName() != "boards" {
			t.Fatalf("mounts = %+v, want [boards] (ds holds only a PDF, empty holds nothing)", got)
		}
		if resp.GetPrunedMounts() != 2 {
			t.Errorf("prunedMounts = %d, want 2 so the sidebar can say what it hid", resp.GetPrunedMounts())
		}
	})

	// The mirror: the datasheets tree asks the same question about the same mounts and gets the
	// other answer, which is the whole point of the kinds being in the request.
	t.Run("a datasheets client keeps the mount of PDFs and prunes the boards", func(t *testing.T) {
		resp, err := svc.ListMounts(context.Background(), &webapi.ListMountsRequest{
			Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DATASHEET},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := resp.GetMounts()
		if len(got) != 1 || got[0].GetName() != "ds" {
			t.Fatalf("mounts = %+v, want [ds]", got)
		}
	})

	// A client that declares nothing gets everything, which is what any caller written before this
	// field existed does.
	t.Run("prunes nothing when the caller declares nothing", func(t *testing.T) {
		resp, err := svc.ListMounts(context.Background(), &webapi.ListMountsRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.GetMounts()) != 3 || resp.GetPrunedMounts() != 0 {
			t.Fatalf("mounts = %+v, pruned = %d, want all 3 and 0", resp.GetMounts(), resp.GetPrunedMounts())
		}
	})
}

// TestListMounts preserves configuration order (the command line's mount order is the UI's
// sidebar order) and maps both fields to the wire form.
func TestListMounts(t *testing.T) {
	ws := &memWorkspace{mounts: []MountInfo{{Name: "z", Root: "/data/z"}, {Name: "a", Root: "/data/a"}}}
	resp, err := NewWorkspaceService(ws).ListMounts(context.Background(), &webapi.ListMountsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	ms := resp.GetMounts()
	if len(ms) != 2 || ms[0].GetName() != "z" || ms[1].GetName() != "a" {
		t.Fatalf("mounts = %+v, want configuration order z then a (not sorted)", ms)
	}
	if ms[0].GetRoot() != "/data/z" {
		t.Errorf("root = %q, want /data/z", ms[0].GetRoot())
	}
}
