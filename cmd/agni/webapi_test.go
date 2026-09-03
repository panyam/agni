package main

import (
	"context"
	"errors"
	"github.com/panyam/agni/artifact"
	"os"
	"path/filepath"
	"testing"

	"github.com/panyam/agni/gen/go/agni/v1/webapi"
	"github.com/panyam/agni/internal/mounts"
	"github.com/panyam/agni/service"
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

	// Over a real filesystem, not just the in-memory port: "hollow" holds only folders and a file
	// no reader opens, so a design browser asking for pruning never sees it.
	t.Run("opens drops a subtree with nothing the caller can open", func(t *testing.T) {
		mustMkdir(t, filepath.Join(root, "hollow", "libs"))
		mustWrite(t, filepath.Join(root, "hollow", "libs", "parts.lock"))
		resp, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: uriStr("m", ""), Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DESIGN}})
		if err != nil {
			t.Fatal(err)
		}
		var dirs []string
		for _, e := range resp.GetEntries() {
			if e.GetIsDir() {
				dirs = append(dirs, e.GetName())
			}
		}
		if len(dirs) != 1 || dirs[0] != "sub" {
			t.Fatalf("dirs = %v, want [sub] (hollow pruned, sub kept for its inner.xml)", dirs)
		}
	})

	// The kind label over a real filesystem: a PDF is a datasheet, a netlist is a design, a lock file
	// is neither. The browser trees filter on this rather than re-deriving it from the extension.
	t.Run("labels each file with the client that opens it", func(t *testing.T) {
		mustWrite(t, filepath.Join(root, "sub", "part.pdf"))
		resp, err := list("m", "sub")
		if err != nil {
			t.Fatal(err)
		}
		byName := map[string]webapi.FileKind{}
		for _, e := range resp.GetEntries() {
			byName[e.GetName()] = e.GetKind()
		}
		if byName["part.pdf"] != webapi.FileKind_FILE_KIND_DATASHEET {
			t.Errorf("part.pdf kind = %v, want DATASHEET", byName["part.pdf"])
		}
		if byName["inner.xml"] != webapi.FileKind_FILE_KIND_DESIGN {
			t.Errorf("inner.xml kind = %v, want DESIGN", byName["inner.xml"])
		}
	})

	t.Run("unknown mount classifies as not-found", func(t *testing.T) {
		if _, err := list("nope", ""); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	// A traversal is refused when the URI is PARSED, not when the adapter joins it, so it classifies
	// as an invalid argument. The malformed value is sent as a raw string because building it through
	// the URI constructor is exactly what is now impossible.
	t.Run("traversal is refused at the parse", func(t *testing.T) {
		_, err := svc.ListDir(context.Background(), &webapi.ListDirRequest{Uri: "mount://m/../.."})
		if !errors.Is(err, service.ErrInvalidArgument) {
			t.Fatalf("want ErrInvalidArgument, got %v", err)
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

// The pruning rule over a real filesystem and real mount roots: "ds" holds a datasheet, which no
// design reader opens, so the design tree is served the boards mount alone. A mount root that does
// not resolve at all is kept rather than counted as empty, since a missing mount is an operator's
// mistake to see, not something to hide.
func TestWorkspaceServiceListMountsPrunesEmptyMounts(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "boards", "rev2"))
	mustWrite(t, filepath.Join(root, "boards", "rev2", "board.edn"))
	mustMkdir(t, filepath.Join(root, "ds"))
	mustWrite(t, filepath.Join(root, "ds", "txb0104.pdf"))

	svc := service.NewWorkspaceService(&osWorkspace{mounts: []mounts.Mount{
		{Name: "boards", Root: filepath.Join(root, "boards")},
		{Name: "ds", Root: filepath.Join(root, "ds")},
		{Name: "gone", Root: filepath.Join(root, "no-such-dir")},
	}})
	resp, err := svc.ListMounts(context.Background(), &webapi.ListMountsRequest{Opens: []webapi.FileKind{webapi.FileKind_FILE_KIND_DESIGN}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range resp.GetMounts() {
		names = append(names, m.GetName())
	}
	if len(names) != 2 || names[0] != "boards" || names[1] != "gone" {
		t.Fatalf("mounts = %v, want [boards gone]: ds serves only a datasheet, and an unreadable root is not proof of emptiness", names)
	}
	if resp.GetPrunedMounts() != 1 {
		t.Errorf("prunedMounts = %d, want 1", resp.GetPrunedMounts())
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
