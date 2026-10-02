package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestNoCommandCallsAHandRolledEncoderInAnotherPackage closes the gap TestEveryJSONFormatEmitsAProto
// states about itself: it reads this package's source, so a json encoder one package away is
// invisible to it. `query` once reached one through report.TableJSON, and `review` through
// review.RenderJSON until agni issue 734. This follows every call cmd/agni makes into the module's
// other packages and fails on one that lands in an exported function building json with
// encoding/json, which is a second shape of an answer the wire already has a message for (C31).
func TestNoCommandCallsAHandRolledEncoderInAnotherPackage(t *testing.T) {
	fset := token.NewFileSet()
	cmdFiles := parseDir(t, fset, ".")
	// The module packages this command imports, by import path, and the local name each file uses.
	imported := map[string]bool{}
	for _, f := range cmdFiles {
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(p, modulePath+"/") && !strings.Contains(p, "/gen/") {
				imported[p] = true
			}
		}
	}
	encoders := map[string]bool{} // "importpath.Func"
	for p := range imported {
		dir := filepath.Join("..", "..", strings.TrimPrefix(p, modulePath+"/"))
		for _, name := range handRolledEncoders(parseDir(t, fset, dir)) {
			encoders[p+"."+name] = true
		}
	}
	var offenders []string
	for _, f := range cmdFiles {
		alias := map[string]string{} // local name -> import path
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			name := filepath.Base(p)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			alias[name] = p
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && encoders[alias[id.Name]+"."+sel.Sel.Name] {
				offenders = append(offenders, fset.Position(call.Pos()).String()+" "+id.Name+"."+sel.Sel.Name)
			}
			return true
		})
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("these commands reach a hand-rolled json encoder in another package: %v\n\n"+
			"C31: --format json is protojson of the command's wire message. Emit the message the "+
			"rpc returns instead, and delete the encoder once nothing calls it.", offenders)
	}
}

// TestTheReachCheckFindsAHandRolledEncoder is the positive control: the detector finds an exported
// function building json with encoding/json, and ignores protojson and unexported helpers, so the
// test above cannot pass by matching nothing.
func TestTheReachCheckFindsAHandRolledEncoder(t *testing.T) {
	const src = `package p
import (
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
)
func RenderJSON(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
func Wire(m any) ([]byte, error) { return protojson.Marshal(nil) }
func helper(v any) ([]byte, error) { return json.Marshal(v) }
`
	f, err := parser.ParseFile(token.NewFileSet(), "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := handRolledEncoders([]*ast.File{f}); len(got) != 1 || got[0] != "RenderJSON" {
		t.Errorf("found %v, want only RenderJSON", got)
	}
}

const modulePath = "github.com/panyam/agni"

// handRolledEncoders names the exported functions whose bodies call encoding/json's Marshal,
// MarshalIndent or NewEncoder.
func handRolledEncoders(files []*ast.File) []string {
	var out []string
	for _, f := range files {
		jsonName := ""
		for _, imp := range f.Imports {
			if p, _ := strconv.Unquote(imp.Path.Value); p == "encoding/json" {
				jsonName = "json"
				if imp.Name != nil {
					jsonName = imp.Name.Name
				}
			}
		}
		if jsonName == "" {
			continue
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil || !fn.Name.IsExported() || fn.Recv != nil {
				continue
			}
			found := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if sel, ok := n.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == jsonName {
						switch sel.Sel.Name {
						case "Marshal", "MarshalIndent", "NewEncoder":
							found = true
						}
					}
				}
				return !found
			})
			if found {
				out = append(out, fn.Name.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func parseDir(t *testing.T, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	var out []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		src, err := os.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, n, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}
