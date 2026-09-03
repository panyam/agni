package review

import (
	"fmt"
	"strings"
	"testing"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
)

// This package holds no query language (C29), so its own tests have to supply one. The adapter below
// is stdlib/reviewquery's, written out here because that package imports THIS one and an in-package
// test cannot import it back. Importing core/query from a test is the same posture stdlib/relations
// takes: the production dependency is zero (go list -deps proves it), and the test still exercises
// the real engine rather than a stub that could agree with a broken contract.
type datalogCompiler struct{}

func (datalogCompiler) CompileQuery(req QueryRequest) (*check.Rule, error) {
	prog, err := query.Parse(req.Query)
	if err != nil {
		return nil, fmt.Errorf("query does not parse: %w", err)
	}
	return query.RuleFromQuery(query.FindingQuery{
		Rule:        req.Rule,
		Query:       prog,
		Kind:        req.Kind,
		SubjectVar:  req.Subject,
		Message:     req.Message,
		ParamSymbol: req.ParamSymbol,
	}), nil
}

func init() { RegisterQueryCompiler(datalogCompiler{}) }

// TestNoQueryCompilerIsAnError pins the half of the silent-pass trap this seam introduces. A binary
// that composes no query engine still loads a manifest, and an inline query it cannot compile must
// not resolve to zero rules: runItem reads that as an item nothing checked, so a house rule would
// report as unverified when it was never parsed. The error names the omission instead.
func TestNoQueryCompilerIsAnError(t *testing.T) {
	compilerMu.Lock()
	orig := compiler
	compiler = nil
	compilerMu.Unlock()
	t.Cleanup(func() {
		compilerMu.Lock()
		compiler = orig
		compilerMu.Unlock()
	})

	_, err := compileQuery(Item{ID: "i", Binding: Binding{Query: &QueryBinding{
		Match: `component.mpn(?r,"X") => ?r`, Subject: "r", Message: "m",
	}}})
	if err == nil {
		t.Fatal("compiling an inline query with no engine registered returned no error")
	}
	if got := err.Error(); !strings.Contains(got, "no query compiler is registered") {
		t.Errorf("error %q does not name the missing engine", got)
	}
}

// TestRegisterQueryCompilerRejectsBadRegistration pins that a nil compiler and a second engine both
// fail at load. Two engines sharing one manifest's match field would make a query's language depend
// on import order, which is not a thing anyone could debug.
func TestRegisterQueryCompilerRejectsBadRegistration(t *testing.T) {
	for name, fn := range map[string]func(){
		"nil compiler":  func() { RegisterQueryCompiler(nil) },
		"second engine": func() { RegisterQueryCompiler(datalogCompiler{}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("RegisterQueryCompiler(%s) did not panic; want a load-time rejection", name)
				}
			}()
			fn()
		})
	}
}
