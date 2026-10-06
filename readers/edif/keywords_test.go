package edif

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// lookedUp returns every keyword literal the reader's non-test source looks a list up by: the
// second argument of collect and findFirst, the argument of Child and Children, a string compared
// with Head(), and a case label of a switch on Head().
func lookedUp(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	lit := func(e ast.Expr) {
		if b, ok := e.(*ast.BasicLit); ok && b.Kind == token.STRING {
			if s, err := strconv.Unquote(b.Value); err == nil {
				seen[s] = true
			}
		}
	}
	isHead := func(e ast.Expr) bool {
		c, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		s, ok := c.Fun.(*ast.SelectorExpr)
		return ok && s.Sel.Name == "Head"
	}
	for _, en := range entries {
		name := en.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "writer.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				switch fn := x.Fun.(type) {
				case *ast.Ident:
					if (fn.Name == "collect" || fn.Name == "findFirst") && len(x.Args) >= 2 {
						lit(x.Args[1])
					}
				case *ast.SelectorExpr:
					if (fn.Sel.Name == "Child" || fn.Sel.Name == "Children") && len(x.Args) == 1 {
						lit(x.Args[0])
					}
				}
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					if isHead(x.X) {
						lit(x.Y)
					}
					if isHead(x.Y) {
						lit(x.X)
					}
				}
			case *ast.SwitchStmt:
				if x.Tag != nil && isHead(x.Tag) {
					for _, s := range x.Body.List {
						for _, e := range s.(*ast.CaseClause).List {
							lit(e)
						}
					}
				}
			}
			return true
		})
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestEveryLookedUpKeywordIsCanonicalized holds the keyword table to the reader. A keyword the
// reader looks a list up by but the table lacks would match only in the reader's own spelling, which
// is how an Altium export read as an empty design (agni issue 941).
func TestEveryLookedUpKeywordIsCanonicalized(t *testing.T) {
	used := lookedUp(t)
	// Positive control: the sweep has to find the reader's lookups, or a pattern that matches
	// nothing would pass over an empty set.
	if len(used) < 50 {
		t.Fatalf("the sweep found only %d keyword lookups, so it is not reading the reader: %v", len(used), used)
	}
	for _, k := range used {
		if canonical[strings.ToLower(k)] != k {
			t.Errorf("the reader looks lists up by %q, but the keyword table spells it %q", k, canonical[strings.ToLower(k)])
		}
	}
}
