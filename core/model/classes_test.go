package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"

	"github.com/panyam/agni/core/model"
)

// TestComponentClassesListsEveryConstant holds ComponentClasses to the const block it enumerates, by
// reading types.go: every constant typed ComponentClass, bar ClassUnknown, must be listed, and nothing
// else may be. A class added to the vocabulary and not to the list is a class no project can extend.
func TestComponentClassesListsEveryConstant(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "types.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var declared []string
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, s := range gd.Specs {
			vs := s.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "ComponentClass" {
				continue
			}
			for _, v := range vs.Values {
				if lit, ok := v.(*ast.BasicLit); ok && lit.Value != `"unknown"` {
					declared = append(declared, lit.Value[1:len(lit.Value)-1])
				}
			}
		}
	}
	// Positive control: a parse that found nothing would compare equal to nothing.
	if len(declared) < 20 {
		t.Fatalf("found only %d ComponentClass constants in types.go; the AST walk is not matching the const block", len(declared))
	}
	var listed []string
	for _, cl := range model.ComponentClasses() {
		listed = append(listed, string(cl))
	}
	sort.Strings(declared)
	sort.Strings(listed)
	if len(declared) != len(listed) {
		t.Fatalf("ComponentClasses lists %v, types.go declares %v", listed, declared)
	}
	for i := range declared {
		if declared[i] != listed[i] {
			t.Fatalf("ComponentClasses lists %v, types.go declares %v", listed, declared)
		}
	}
}
