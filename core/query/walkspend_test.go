package query

import (
	"context"
	"errors"
	"testing"

	"github.com/panyam/agni/core/check"
)

// The circuit walks charge the connections they examine to the evaluation's budget
// (facts.Env.Spend, panyam/jaala#361). These tests read the all-pairs walk over the straight chain
// N0 -R1- N1 -R2- N2 -R3- N3, the shape whose cost runs ahead of the rows it emits.

const allPairs = `net.reaches(?a, ?b) => ?a, ?b`

// walkCost evaluates allPairs on a fresh base and returns the engine's own work and the total Work
// reports.
func walkCost(t *testing.T) (engine, total int64) {
	t.Helper()
	b := NewBase(check.NewModel(reachChainDesign()))
	rows, err := Default.Eval(context.Background(), mustParse(t, allPairs), b)
	if err != nil || len(rows) == 0 {
		t.Fatalf("unbudgeted walk: %d rows, %v", len(rows), err)
	}
	return b.Work(), Work(b)
}

// TestWorkCountsWhatAWalkSpent: the reported work is the engine's plus the walk's own steps, and a
// query calling no walk reports the engine's alone.
func TestWorkCountsWhatAWalkSpent(t *testing.T) {
	engine, total := walkCost(t)
	if total <= engine {
		t.Errorf("Work = %d with the engine's own %d, want the walk's steps added", total, engine)
	}
	b := NewBase(check.NewModel(reachChainDesign()))
	if _, err := Default.Eval(context.Background(), mustParse(t, `component.net(?r, ?n) => ?r`), b); err != nil {
		t.Fatal(err)
	}
	if Work(b) != b.Work() || b.Work() == 0 {
		t.Errorf("a query with no walk: Work = %d, engine = %d, want them equal and nonzero", Work(b), b.Work())
	}
}

// TestABudgetStopsAWalkOnWhatItSpent: a budget the engine's work alone fits in stops the walk once
// its steps are charged too. control: a budget of the whole reported work answers.
func TestABudgetStopsAWalkOnWhatItSpent(t *testing.T) {
	engine, total := walkCost(t)
	eval := func(budget int64) error {
		ctx := WithBudget(context.Background(), budget)
		_, err := Default.Eval(ctx, mustParse(t, allPairs), NewBase(check.NewModel(reachChainDesign())), EvalOptions(ctx)...)
		return err
	}
	var be *BudgetExceeded
	if err := eval(engine); !errors.As(err, &be) {
		t.Errorf("under a budget of the engine's own work (%d): err = %v, want the budget exceeded", engine, err)
	}
	if err := eval(total); err != nil {
		t.Errorf("control: under a budget of the reported work (%d): %v", total, err)
	}
}
