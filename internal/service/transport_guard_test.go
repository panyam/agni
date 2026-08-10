package service

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoTransportImports is CONSTRAINTS C13's verify, executable: the service implementations
// carry plain proto signatures and no transport dependency; Connect (and any later gRPC /
// gateway transport) lives in internal/server as a translation layer. Test files are exempt
// (they may drive adapters), implementation files are not.
func TestNoTransportImports(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(fset, f, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, "connectrpc.com/") || strings.Contains(path, "grpc") {
				t.Errorf("%s imports transport package %q; service impls are transport-neutral (C13), adapters live in internal/server", f, path)
			}
		}
	}
}

// TestNoFilesystemImports is the other half of C13's verify, and C22's: the services take their I/O
// through injected ports, so an impl file reaching for the filesystem directly is the violation both
// constraints exist to prevent. C22 named this guard as its verify before the guard existed; WS9-050
// wrote it, which is also when it became load-bearing — RunReview now takes its checklist as a value
// specifically so it needs no filesystem, and nothing but this test stops a later edit from quietly
// reading a file to "just resolve the path".
//
// "path" (pure string manipulation on mount-relative refs, no syscalls) is fine and workspace.go uses
// it; "path/filepath" is not, because it resolves against the HOST's separator and conventions, which
// is exactly the assumption a ref must not carry.
func TestNoFilesystemImports(t *testing.T) {
	banned := map[string]string{
		"os":            "read files through the injected Loader port instead",
		"os/exec":       "shelling out is an adapter concern (cmd/agni wires native tools)",
		"path/filepath": "refs are not host paths; use \"path\" for mount-relative string work",
		"io/fs":         "read files through the injected Loader port instead",
		"syscall/js":    "a service impl is runtime-agnostic; the WASM edge is an adapter",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(fset, f, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if why, bad := banned[p]; bad {
				t.Errorf("%s imports %q; service impls take I/O via injected ports (C13/C22): %s", f, p, why)
			}
		}
	}
}
