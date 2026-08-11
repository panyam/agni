package main

import (
	"context"
	"errors"
	"github.com/panyam/agni/internal/artifact"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/internal/service"
)

func TestWorkspaceServiceListDir(t *testing.T) {
	root := t.TempDir()
	// Layout: a subdir, two supported files, one unsupported, one dotfile.
	mustMkdir(t, filepath.Join(root, "sub"))
	mustWrite(t, filepath.Join(root, "board.edn"))
	mustWrite(t, filepath.Join(root, "amp.sch")) // xschem/gEDA schematic (regression: was hidden)
	mustWrite(t, filepath.Join(root, "layout.kicad_pcb"))
	mustWrite(t, filepath.Join(root, "notes.txt"))   // unsupported
	mustWrite(t, filepath.Join(root, ".hidden.edn")) // dotfile
	mustWrite(t, filepath.Join(root, "sub", "inner.xml"))

	svc := service.NewWorkspaceService(&osWorkspace{mounts: []mounts.Mount{{Name: "m", Root: root}}})
	list := func(mount, path string) (*webapi.ListDirResponse, error) {
		resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr(mount, path)})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	t.Run("root lists dirs first then all files, unsupported tagged with empty format", func(t *testing.T) {
		msg, err := list("m", "")
		if err != nil {
			t.Fatal(err)
		}
		got := msg.GetEntries()
		if len(got) != 5 {
			t.Fatalf("want 5 entries (sub, amp.sch, board.edn, layout.kicad_pcb, notes.txt), got %d: %+v", len(got), got)
		}
		if !got[0].GetIsDir() || got[0].GetName() != "sub" {
			t.Errorf("first entry should be dir sub, got %+v", got[0])
		}
		if got[1].GetName() != "amp.sch" || got[1].GetFormat() != "xschem" {
			t.Errorf("want amp.sch/xschem (regression: .sch must not be filtered out), got %+v", got[1])
		}
		if got[2].GetName() != "board.edn" || got[2].GetFormat() != "edif" {
			t.Errorf("want board.edn/edif, got %+v", got[2])
		}
		if got[3].GetName() != "layout.kicad_pcb" || got[3].GetFormat() != "kicad" {
			t.Errorf("want layout.kicad_pcb/kicad, got %+v", got[3])
		}
		// An unrecognized file is listed (folder never looks empty) but carries no format, which
		// the UI renders disabled.
		if got[4].GetName() != "notes.txt" || got[4].GetFormat() != "" {
			t.Errorf("want notes.txt with empty format (unsupported), got %+v", got[4])
		}
	})

	t.Run("subdir via path, mount-relative", func(t *testing.T) {
		msg, err := list("m", "sub")
		if err != nil {
			t.Fatal(err)
		}
		got := msg.GetEntries()
		if len(got) != 1 || got[0].GetName() != "inner.xml" || got[0].GetUri() != "mount://m/sub/inner.xml" {
			t.Fatalf("want [sub/inner.xml], got %+v", got)
		}
	})

	t.Run("unknown mount classifies as not-found", func(t *testing.T) {
		if _, err := list("nope", ""); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("traversal keeps its invalid-path classification", func(t *testing.T) {
		if _, err := list("m", "../.."); !errors.Is(err, service.ErrInvalidPath) {
			t.Fatalf("want ErrInvalidPath, got %v", err)
		}
	})

	t.Run("missing dir classifies as not-found", func(t *testing.T) {
		if _, err := list("m", "does-not-exist"); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p string) {
	t.Helper()
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceServiceListMounts(t *testing.T) {
	svc := service.NewWorkspaceService(&osWorkspace{mounts: []mounts.Mount{{Name: "a", Root: "/x"}, {Name: "b", Root: "/y"}}})
	resp, err := svc.ListMounts(context.Background(), &webapi.ListMountsRequest{})
	if err != nil {
		t.Fatalf("ListMounts errored: %v", err)
	}
	got := resp.GetMounts()
	if len(got) != 2 || got[0].GetName() != "a" || got[0].GetRoot() != "/x" || got[1].GetName() != "b" {
		t.Fatalf("unexpected mounts: %+v", got)
	}
}

// uriStr builds an artifact URI string for a request literal in a test. A fixture URI that will not
// parse is a broken test rather than a condition under test, so it panics instead of returning an
// error nobody would check.
func uriStr(mount, p string) string {
	u, err := artifact.New(mount, p)
	if err != nil {
		panic(err)
	}
	return u.String()
}
