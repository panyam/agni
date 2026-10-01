package query

import (
	"context"
	"testing"
)

// TestNarrowBudgetNeverRaises is the rule that lets a request ask for less work than a deployment
// allows and never more (agni issue 792).
func TestNarrowBudgetNeverRaises(t *testing.T) {
	for _, c := range []struct {
		name        string
		set, narrow int64
		want        int64
	}{
		{"lower", 100, 10, 10},
		{"higher is ignored", 100, 1000, 100},
		{"none set takes the request's", 0, 50, 50},
		{"zero leaves it", 100, 0, 100},
		{"nothing at all", 0, 0, 0},
	} {
		got := BudgetOf(NarrowBudget(WithBudget(context.Background(), c.set), c.narrow))
		if got != c.want {
			t.Errorf("%s: set %d, narrowed by %d, got %d, want %d", c.name, c.set, c.narrow, got, c.want)
		}
	}
	if len(EvalOptions(context.Background())) != 0 {
		t.Error("an unbudgeted context asks the engine for a budget")
	}
}
