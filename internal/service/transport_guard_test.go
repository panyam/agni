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
