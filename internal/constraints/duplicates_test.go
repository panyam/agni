package constraints

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// C33: a function body is not copied into a second package.
//
// Every duplicate agni 698 found had the same history: one package needed what another had, the
// function was copied because importing it was inconvenient, and the two agreed until someone edited
// one. The net-class cascade was written twice and agreed by luck; model.RenderRoute and
// isRegulatorInternal each did the same before they were caught. This test fires at the COPY, which is
// the one moment the duplicate is exact. It cannot see two copies that have since diverged, which is
// what a review is for.
//
// Six lines is where a body stops being an idiom anyone would retype and starts being logic somebody
// wrote once. The allowlist records a duplicate that is COINCIDENCE, with the reason, and an entry
// matching nothing fails, so the list cannot outlive what it excuses.
var allowedDuplicateBodies = map[string]string{
	"readers/edif/writer.go:sortedKeys + stdlib/rules/intent/strapgroups.go:sortedKeys": "the " +
		"sort-a-map's-keys idiom; two copies cannot disagree about anything",
}

const minDuplicateLines = 6

func TestC33NoFunctionBodyIsCopiedAcrossPackages(t *testing.T) {
	root := repoRoot(t)
	files := map[string][]byte{}
	for _, f := range engineSources(t, root) {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		files[mustRel(t, root, f)] = b
	}
	groups, seen := duplicateBodies(t, files)
	if seen < 2000 {
		t.Fatalf("parsed only %d function bodies, so finding no duplicates proves nothing", seen)
	}
	found := map[string]bool{}
	for _, g := range groups {
		found[g] = true
		if _, ok := allowedDuplicateBodies[g]; !ok {
			t.Errorf("the same function body appears in more than one package: %s. Promote one copy to "+
				"a package both can import (named for what it does, never a utils), or, if the two are "+
				"coincidence, add the group to allowedDuplicateBodies with the reason (C33)", g)
		}
	}
	for g := range allowedDuplicateBodies {
		if !found[g] {
			t.Errorf("allowedDuplicateBodies excuses %s, which is no longer duplicated; remove the entry", g)
		}
	}
}

// The positive control. A matcher that never fires would leave the sweep above green on any tree.
func TestC33DetectsACopiedBody(t *testing.T) {
	body := "func %s(xs []int) int {\n\tn := 0\n\tfor _, x := range xs {\n\t\tn += x\n\t}\n\treturn n\n}\n"
	files := map[string][]byte{
		"a/one.go":   []byte("package a\n\n// total sums.\n" + strings.Replace(body, "%s", "total", 1)),
		"b/two.go":   []byte("package b\n\n" + strings.Replace(body, "%s", "sum", 1)),
		"a/three.go": []byte("package a\n\n" + strings.Replace(body, "%s", "again", 1)),
	}
	groups, _ := duplicateBodies(t, files)
	if len(groups) != 1 || groups[0] != "a/one.go:total + a/three.go:again + b/two.go:sum" {
		t.Errorf("want one group spanning packages a and b, got %q", groups)
	}
	files["b/two.go"] = []byte("package b\n\n" + strings.Replace(strings.Replace(body, "%s", "sum", 1), "n += x", "n -= x", 1))
	if groups, _ := duplicateBodies(t, files); len(groups) != 0 {
		t.Errorf("a copy confined to one package is that package's business, got %q", groups)
	}
}

// duplicateBodies groups functions by their body with comments dropped, and returns each group that
// spans more than one package directory as "file:func + file:func", sorted, plus the count of bodies
// it parsed.
func duplicateBodies(t *testing.T, files map[string][]byte) ([]string, int) {
	t.Helper()
	type fn struct{ file, name string }
	byBody := map[string][]fn{}
	seen := 0
	for rel, src := range files {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			seen++
			if fset.Position(fd.End()).Line-fset.Position(fd.Pos()).Line < minDuplicateLines {
				continue
			}
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, token.NewFileSet(), fd.Body); err != nil {
				t.Fatal(err)
			}
			byBody[buf.String()] = append(byBody[buf.String()], fn{filepath.ToSlash(rel), fd.Name.Name})
		}
	}
	var out []string
	for _, fns := range byBody {
		pkgs := map[string]bool{}
		var names []string
		for _, f := range fns {
			pkgs[filepath.Dir(f.file)] = true
			names = append(names, f.file+":"+f.name)
		}
		if len(pkgs) > 1 {
			sort.Strings(names)
			out = append(out, strings.Join(names, " + "))
		}
	}
	sort.Strings(out)
	return out, seen
}
