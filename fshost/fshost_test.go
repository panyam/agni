package fshost

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/panyam/agni/artifact"
	"github.com/panyam/agni/service"
)

func testMounts() []Mount {
	return []Mount{
		{Name: "b", FS: MemFS(map[string][]byte{"x/y.txt": []byte("y"), "top.txt": []byte("t")})},
		{Name: "a", FS: MemFS(map[string][]byte{"one.txt": []byte("1")})},
		// A repeated name is ignored, so the first "a" is the one served.
		{Name: "a", FS: MemFS(map[string][]byte{"shadow.txt": []byte("s")})},
	}
}

// TestRootIsAnFS runs the standard library's fs.FS conformance checks over a composed root, which
// fs.WalkDir, fs.Sub and the readers all rely on.
func TestRootIsAnFS(t *testing.T) {
	if err := fstest.TestFS(Root(testMounts()), "a/one.txt", "b/top.txt", "b/x/y.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := Root(testMounts()).Open("a/shadow.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a repeated mount name was served: %v", err)
	}
}

func TestHostResolvesAURIOnlyInsideItsMount(t *testing.T) {
	h := New(testMounts()...)
	cases := []struct {
		uri  artifact.URI
		want string
		err  error
	}{
		{artifact.URI{Mount: "b", Path: "x/y.txt"}, "b/x/y.txt", nil},
		{artifact.URI{Mount: "b", Path: ""}, "b", nil},
		{artifact.URI{Mount: "nope", Path: "x"}, "", service.ErrNotFound},
		{artifact.URI{Mount: "b", Path: "../a/one.txt"}, "", service.ErrInvalidPath},
	}
	for _, c := range cases {
		got, err := h.name(c.uri)
		if got != c.want || !errors.Is(err, c.err) || (c.err == nil) != (err == nil) {
			t.Errorf("name(%+v) = %q, %v; want %q, %v", c.uri, got, err, c.want, c.err)
		}
	}
	entries, err := h.Workspace().ListDir(context.Background(), artifact.URI{Mount: "b", Path: ""})
	if err != nil || len(entries) != 2 {
		t.Errorf("ListDir(b) = %v, %v; want top.txt and x", entries, err)
	}
}

// TestNoHostFilesystem holds the package to what it is for: every read goes through an fs.FS, so the
// same code serves an in-memory tree in a browser. An os or path/filepath import is the bug shape,
// since it works on a server and fails in every host with no filesystem.
func TestNoHostFilesystem(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		pf, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for _, imp := range pf.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if p == "os" || p == "path/filepath" || p == "syscall/js" {
				t.Errorf("%s imports %s", f, p)
			}
		}
	}
	if checked == 0 {
		t.Fatal("checked no source files, so this guard would pass over nothing")
	}
}
