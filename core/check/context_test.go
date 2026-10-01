package check

import (
	"context"
	"errors"
	"testing"

	ir "github.com/panyam/agni/gen/go/agni/v1/ir"
)

// TestRunStopsWhenItsContextIsDone covers agni issue 795 in the runner. The first rule cancels the
// run, as a caller going away mid-check would; Run must not evaluate the second rule, and must return
// the cancellation rather than the findings so far, which would read as a run that found nothing more.
func TestRunStopsWhenItsContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ran := 0
	first := &Rule{Name: "first", Severity: "error", Eval: func(context.Context, Model) []Verdict { ran++; cancel(); return nil }}
	second := &Rule{Name: "second", Severity: "error", Eval: func(context.Context, Model) []Verdict { ran++; return nil }}
	m := NewModel(&ir.Design{})
	for name, run := range map[string]func() error{
		"Run":         func() error { _, err := Run(ctx, m, []*Rule{first, second}); return err },
		"RunVerdicts": func() error { _, err := RunVerdicts(ctx, m, []*Rule{withSet(first), withSet(second)}); return err },
	} {
		ran = 0
		ctx, cancel = context.WithCancel(context.Background())
		if err := run(); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err = %v, want context.Canceled", name, err)
		}
		if ran != 1 {
			t.Errorf("%s ran %d rules after the first cancelled, want it to stop at 1", name, ran)
		}
		cancel()
	}
}

func withSet(r *Rule) *Rule { c := *r; c.StatesConsideredSet = true; return &c }
