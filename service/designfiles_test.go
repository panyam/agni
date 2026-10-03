package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/panyam/agni/fshost"
	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/projects"
	"github.com/panyam/agni/service"
)

// designFilesSvc serves the tutorial project as mount "tut" through the fs.FS backend, with the
// project store a server composes, so the descriptor and the config directories it names are real.
func designFilesSvc(t *testing.T) *service.WorkspaceService {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "examples", "tutorial-project"))
	if err != nil {
		t.Fatal(err)
	}
	m := fshost.Mount{Name: "tut", FS: os.DirFS(root)}
	host := fshost.New(m)
	resolver := &service.ProjectResolver{
		Store:  projects.NewFSStore(projects.Tree{Mount: m.Name, FS: m.FS}),
		Config: &fshost.ConfigResolver{Mounts: []fshost.Mount{m}},
	}
	return service.NewWorkspaceService(host.Workspace()).WithDesignFiles(resolver, host.Workspace())
}

func listDesignFiles(t *testing.T, svc *service.WorkspaceService, uri string) *webapi.ListDesignFilesResponse {
	t.Helper()
	resp, err := svc.ListDesignFiles(context.Background(), &webapi.ListDesignFilesRequest{Uri: uri})
	if err != nil {
		t.Fatalf("ListDesignFiles(%s): %v", uri, err)
	}
	return resp
}

func filePaths(resp *webapi.ListDesignFilesResponse) []string {
	var out []string
	for _, f := range resp.GetFiles() {
		out = append(out, f.GetPath())
	}
	return out
}

// TestListDesignFilesIsWhatTheAnalysisReads holds the set to the tiers the engine reads. A file
// missing here is a tier the browser's engine analyses without, and it reads clean rather than
// failing, so each tier is named rather than counted.
func TestListDesignFilesIsWhatTheAnalysisReads(t *testing.T) {
	svc := designFilesSvc(t)
	resp := listDesignFiles(t, svc, "mount://tut/designs/gateway")
	got := map[string]bool{}
	var total int64
	for _, f := range resp.GetFiles() {
		got[f.GetPath()] = true
		total += f.GetSize()
		if !strings.HasPrefix(f.GetSha256(), "sha256:") || f.GetSize() == 0 {
			t.Errorf("%s: size %d, hash %q", f.GetPath(), f.GetSize(), f.GetSha256())
		}
	}
	for _, want := range []string{
		"designs/gateway/gateway.edn",               // the entry
		"designs/gateway/gateway.kicad_sch",         // the schematic companion
		"designs/gateway/gateway.kicad_pcb",         // the board companion
		"designs/gateway/gateway-rev-b.edn",         // a revision, for a diff
		"designs/gateway/design.yaml",               // the design's descriptor
		"designs/gateway/symbols/gateway.kicad_sym", // the design's own symbol library
		"project.yaml",                              // the project's descriptor
		"profiles/can.yaml",                         // a project profile
		"params/acme-buck-3v3.textproto",            // a project datasheet spec
		"lib/house.dl",                              // a project library module
	} {
		if !got[want] {
			t.Errorf("missing %s; got %v", want, filePaths(resp))
		}
	}
	for _, unwanted := range []string{"README.md", "Makefile", "tools/gen_kicad_views.py", ".gitignore"} {
		if got[unwanted] {
			t.Errorf("%s is not read by the analysis and should not be listed", unwanted)
		}
	}
	if resp.GetMount() != "tut" || resp.GetTotalSize() != total {
		t.Errorf("mount %q total %d, want tut and %d", resp.GetMount(), resp.GetTotalSize(), total)
	}

	// Naming the entry or a companion names the same design (C32), so it lists the same files.
	for _, other := range []string{"mount://tut/designs/gateway/gateway.edn", "mount://tut/designs/gateway/gateway.kicad_sch"} {
		if a, b := strings.Join(filePaths(resp), ","), strings.Join(filePaths(listDesignFiles(t, svc, other)), ","); a != b {
			t.Errorf("%s lists a different set from the design folder:\n%s\n%s", other, b, a)
		}
	}
}

func TestListDesignFilesRefusesWhatItCannotList(t *testing.T) {
	svc := designFilesSvc(t)
	for _, uri := range []string{"mount://nope/x.edn", "mount://tut/designs/gateway/missing.edn"} {
		if _, err := svc.ListDesignFiles(context.Background(), &webapi.ListDesignFilesRequest{Uri: uri}); !errors.Is(err, service.ErrNotFound) {
			t.Errorf("%s: err %v, want not found", uri, err)
		}
	}
	bare := service.NewWorkspaceService(fshost.New().Workspace())
	if _, err := bare.ListDesignFiles(context.Background(), &webapi.ListDesignFilesRequest{Uri: "mount://tut/x"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Errorf("a host without WithDesignFiles: err %v, want invalid argument", err)
	}
}
