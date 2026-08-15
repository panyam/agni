package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/panyam/agni/internal/artifact"
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

// A design browser can only ever show a folder with no readable design under it as empty, so
// PruneEmptyDirs leaves it out. "Empty" means the whole subtree, not one level: a folder of
// folders of library files is as useless to open as a folder with nothing in it.
func TestWorkspaceServiceListDirPrunesEmptyDirs(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "m", Root: "/x"}},
		entries: map[string][]DirEntry{
			"m\x00": {
				{Name: "boards", IsDir: true}, // design lives two levels down
				{Name: "empty", IsDir: true},  // nothing at all
				{Name: "libs", IsDir: true},   // files, but none a reader opens
			},
			"m\x00boards":      {{Name: "rev2", IsDir: true}},
			"m\x00boards/rev2": {{Name: "board.edn"}},
			"m\x00empty":       {},
			"m\x00libs":        {{Name: "parts.lock"}, {Name: "nested", IsDir: true}},
			"m\x00libs/nested": {{Name: "readme.md"}, {Name: ".hidden.edn"}}, // dotfile does not count
		},
	})
	names := func(prune bool) []string {
		t.Helper()
		resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), PruneEmptyDirs: prune})
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
		if got := names(true); len(got) != 1 || got[0] != "boards" {
			t.Fatalf("entries = %v, want [boards] (empty and libs pruned)", got)
		}
	})

	// The flag is opt-in because "empty" is per-client: the datasheets tree lists PDFs, which no
	// design reader opens, so pruning by design format would hide the folders it wants. Off, the
	// listing is unchanged.
	t.Run("off by default", func(t *testing.T) {
		if got := names(false); len(got) != 3 {
			t.Fatalf("entries = %v, want all 3 dirs when pruning is off", got)
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
	resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), PruneEmptyDirs: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.GetEntries(); len(got) != 1 || got[0].GetName() != "deep" {
		t.Fatalf("entries = %+v, want deep kept: an unfinished walk shows the folder", got)
	}
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
