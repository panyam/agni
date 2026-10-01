package review

import (
	"fmt"
	"sync"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/facts"
)

// A manifest may bind an item to an INLINE query, a house rule authored in the checklist rather than
// in the shipped catalog. This package knows the manifest's own vocabulary (required fields, what a
// kind may be, what the rule is called) and delegates parsing the query to a registered compiler, so
// no core package knows a query language (C29). The two meet at *check.Rule, the same currency
// check.RegisterSource trades in, so a second engine contributes house rules by registering here.

// QueryRequest is everything a compiler needs that is NOT its own language: the identity and
// presentation the manifest fixed, plus the query text in whatever language the compiler speaks.
// Kind and Severity arrive defaulted and validated, so a compiler never re-derives them.
type QueryRequest struct {
	// Rule carries the identity, severity, summary and tags the manifest determined. A compiler fills
	// its Eval and returns it; it must not rename it, because the id is what a review item binds to.
	Rule check.Rule
	// Query is the manifest's `match` text, uninterpreted. Its language is whatever the registered
	// compiler speaks; this package never parses it.
	Query string
	// Subject names the variable (or column) whose binding is the finding's subject.
	Subject string
	// Kind is the subject's entity kind: component, net, or pin. Never empty.
	Kind string
	// Message is the finding template, with placeholders the compiler's own language resolves.
	Message string
	// ParamSymbol optionally names the datasheet symbol the query checks, so a finding gains a
	// structured citation. Empty when the item makes no datasheet claim.
	ParamSymbol string
	// Vocabulary is the relation vocabulary the query's names resolve against: the shipped one plus
	// the run's library (agni issue 779). Nil is the process default. It is the fact layer's registry,
	// not a query engine's, so this package still names no query language (C29).
	Vocabulary *facts.Registry
}

// QueryCompiler turns a manifest's inline query into a rule. An engine registers one implementation;
// the manifest layer holds no opinion about the language beyond requiring that it compile.
type QueryCompiler interface {
	CompileQuery(QueryRequest) (*check.Rule, error)
}

var (
	compilerMu sync.RWMutex
	compiler   QueryCompiler
)

// RegisterQueryCompiler installs the engine that compiles inline manifest queries. Call it once at
// init from a package that imports both this one and a query engine, the way stdlib/relations bridges
// the built-in relations into the fact layer. It panics on a nil compiler or a second registration,
// because two engines silently sharing one manifest's `match` field would make a query's language
// depend on import order.
func RegisterQueryCompiler(c QueryCompiler) {
	compilerMu.Lock()
	defer compilerMu.Unlock()
	if c == nil {
		panic("review: RegisterQueryCompiler with nil compiler")
	}
	if compiler != nil {
		panic("review: RegisterQueryCompiler called twice; one engine compiles a manifest's inline queries")
	}
	compiler = c
}

// QueryCompilerInstalled reports whether an engine has registered an inline-query compiler, for a
// composition check at startup. A binary with no compiler still loads every manifest that binds no
// inline query, so the absence is not an error here, but it stays invisible until some manifest uses
// one.
func QueryCompilerInstalled() bool {
	compilerMu.RLock()
	defer compilerMu.RUnlock()
	return compiler != nil
}

// queryCompiler returns the registered compiler, or an error naming the omission.
//
// A missing compiler is an ERROR rather than a skipped item. Otherwise the inline query resolves to no
// rules and runItem reports a house rule as unverified when it was never even parsed. Load surfaces
// this at parse time, so the failure lands where the manifest is read rather than mid-run.
func queryCompiler() (QueryCompiler, error) {
	compilerMu.RLock()
	defer compilerMu.RUnlock()
	if compiler == nil {
		return nil, fmt.Errorf("no query compiler is registered, so an inline query binding cannot be compiled (import a query engine bridge, e.g. stdlib/reviewquery)")
	}
	return compiler, nil
}
