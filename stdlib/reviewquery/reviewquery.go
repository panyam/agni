// Package reviewquery registers the datalog engine as the compiler for a review manifest's inline
// query bindings. core/review holds the manifest vocabulary and no query language (C29), core/query
// holds the language and knows nothing about checklists, and this package imports both so neither
// has to import the other. Blank-import it wherever a binary composes a checklist surface, the way
// stdlib/relations is blank-imported to install the built-in relations.
//
// A binary that omits it still loads a manifest, but an inline query binding then fails at Load with
// a message naming this package rather than resolving to no rules.
package reviewquery

import (
	"fmt"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/review"
)

// Compiler compiles a manifest's inline query as a datalog program.
type Compiler struct{}

// CompileQuery parses the manifest's match text and binds the result to the identity and presentation
// the manifest already fixed. The error names the query rather than the item, because Load wraps it
// with the item id and doubling that reads as two separate problems.
func (Compiler) CompileQuery(req review.QueryRequest) (*check.Rule, error) {
	prog, err := query.Parse(req.Query)
	if err != nil {
		return nil, fmt.Errorf("query does not parse: %w", err)
	}
	// A fault in an inline query is reported rather than compiled into a rule that passes clean
	// (agni issue 540).
	r, err := query.RuleFromQuery(query.FindingQuery{
		Rule:        req.Rule,
		Query:       prog,
		Kind:        req.Kind,
		SubjectVar:  req.Subject,
		Message:     req.Message,
		ParamSymbol: req.ParamSymbol,
	})
	if err != nil {
		return nil, fmt.Errorf("query does not compile: %w", err)
	}
	return r, nil
}

func init() { review.RegisterQueryCompiler(Compiler{}) }
