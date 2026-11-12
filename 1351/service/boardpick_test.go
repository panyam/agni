package service

import (
	"context"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
)

// A picker that asks for board marks gets the URI to attach on a board file and on an ODB++ job's
// folder, the job's matrix/matrix entry, and on nothing else. A listing that does not ask gets none
// (agni issue 1367).
func TestListDirMarksBoardsWhenAsked(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "m", Root: "/x"}},
		entries: map[string][]DirEntry{
			"m\x00":           {{Name: "job", IsDir: true}, {Name: "plain", IsDir: true}, {Name: "x.kicad_pcb"}, {Name: "x.edn"}},
			"m\x00job/matrix": {{Name: "matrix"}},
			"m\x00job":        {{Name: "matrix", IsDir: true}},
			"m\x00plain":      {{Name: "notes.txt"}},
		},
	})
	marks := func(ask bool) map[string]string {
		resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), MarkBoards: ask})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, e := range resp.GetEntries() {
			out[e.GetName()] = e.GetBoardUri()
		}
		return out
	}
	want := map[string]string{"job": "mount://m/job/matrix/matrix", "plain": "", "x.kicad_pcb": "mount://m/x.kicad_pcb", "x.edn": ""}
	got := marks(true)
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: board_uri %q, want %q", name, got[name], w)
		}
	}
	for name, b := range marks(false) {
		if b != "" {
			t.Errorf("%s is marked though the listing did not ask", name)
		}
	}
}

// A listing filtered to designs keeps an ODB++ job's folder. The job is entered by its matrix/matrix
// file, whose name gives no kind, so the subtree walk alone pruned every job (agni issue 1367).
func TestListDirKeepsAnODBJobWhenFilteringToDesigns(t *testing.T) {
	svc := NewWorkspaceService(&memWorkspace{
		mounts: []MountInfo{{Name: "m", Root: "/x"}},
		entries: map[string][]DirEntry{
			"m\x00":           {{Name: "job", IsDir: true}, {Name: "plain", IsDir: true}},
			"m\x00job":        {{Name: "matrix", IsDir: true}, {Name: "steps", IsDir: true}},
			"m\x00job/matrix": {{Name: "matrix"}},
			"m\x00job/steps":  {},
			"m\x00plain":      {{Name: "notes.txt"}},
		},
	})
	resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DESIGN}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range resp.GetEntries() {
		names = append(names, e.GetName())
	}
	if len(names) != 1 || names[0] != "job" {
		t.Errorf("entries %v, want [job], the ODB++ job kept and the folder of notes pruned", names)
	}
}
