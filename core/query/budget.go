package query

import (
	"context"

	"github.com/panyam/jaala/datalog"
)

// BudgetExceeded is returned when an evaluation's work passes its budget. A host uses
// errors.As to tell it apart from a wrong query. Running out of budget means the question was too
// expensive here, not that it was badly asked.
type BudgetExceeded = datalog.BudgetExceeded

type budgetKey struct{}

// WithBudget returns ctx with a work budget for every query evaluated under it (agni issue 792).
// That covers a served query, and each query that a check's or review's query-backed rules
// evaluate. query.Work counts the work as candidate comparisons, each row emitted by a generator,
// and each connection examined by a circuit walk. A budget of zero or less adds none, so an
// unbudgeted context is still unbudgeted.
//
// A budget applies per evaluation. Each query in a set gets the whole budget, and so does each rule
// in a run. They don't share one.
func WithBudget(ctx context.Context, work int64) context.Context {
	if work <= 0 {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, work)
}

// BudgetOf is the work budget ctx carries, or 0 for none.
func BudgetOf(ctx context.Context) int64 {
	n, _ := ctx.Value(budgetKey{}).(int64)
	return n
}

// NarrowBudget returns ctx with its budget lowered to work when work is smaller, or set to work
// when ctx carries none. It never raises a budget. So a request can ask for less than a deployment
// allows, and never more. Zero or less leaves ctx as it is.
func NarrowBudget(ctx context.Context, work int64) context.Context {
	if work <= 0 {
		return ctx
	}
	if cur := BudgetOf(ctx); cur > 0 && cur <= work {
		return ctx
	}
	return context.WithValue(ctx, budgetKey{}, work)
}

// EvalOptions turns the context's budget, when it carries one, into engine options. agni passes
// them to every evaluation it runs for a caller, so a budget set once on the context reaches each
// one.
func EvalOptions(ctx context.Context) []datalog.Option {
	if n := BudgetOf(ctx); n > 0 {
		return []datalog.Option{datalog.Budget(n)}
	}
	return nil
}

// ExplainReport holds an evaluation's account of itself, filled when the evaluation is asked to
// explain. It records the plan of each rule body, where its work went, and how each relation was
// read (jaala's Explain, panyam/jaala#147).
type ExplainReport = datalog.Report

// Explain asks an evaluation to fill r with its plan. Its cost grows with the rules and literals in
// a query, not with its tuples, so `agni query --explain` and a timed request can afford it.
func Explain(r *ExplainReport) datalog.Option { return datalog.Explain(r) }

// Total asks an evaluation to set *n to its answer's row count before any offset and limit. A
// caller showing the first rows can then say how many it left out (agni issue 896). Counting needs
// every row, so a limit with no order by then solves the whole goal. It still keeps only the rows
// it returns.
func Total(n *int) datalog.Option { return datalog.Total(n) }

// Witness records how one fact of an answer holds. It is either a leaf from the fact base, with its
// citations, or the rule that derived the fact, with that rule's body witnesses in written order
// (jaala's Witness).
type Witness = datalog.Witness

// Witnesses asks an evaluation to record how each answer row was derived, in Table.Witness (agni
// issue 794). The engine keeps a derivation only when asked, so an evaluation that does not ask
// costs no more than before.
func Witnesses() datalog.Option { return datalog.Witnesses() }
