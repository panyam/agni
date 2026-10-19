package service

import (
	"context"
	"sort"
	"sync"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/agni/core/query"
	"github.com/panyam/agni/core/timing"
)

// ruleResults holds what each rule returned on one kept model (agni issue 390), so a check that asks
// for a rule already run there answers from it rather than evaluating the rule again. Rule
// evaluation is most of a check's time: on the largest fetched board, 9.4 of 9.9 seconds.
//
// It lives on the model's cache entry, which is what makes it safe. That entry is keyed on the
// design's tiers and the overlay's identity, and checked on every hit against every file its read
// touched, so an edit to the netlist, the board, a sub-sheet or a symbol library, or a different
// convention, reaches a different entry with nothing stored on it. An entry that could not be kept
// (an overlay that does not identify, a corpus fetched over the network) stores nothing.
type ruleResults struct {
	mu    sync.Mutex
	rules map[ruleKey]ruleRun
}

// ruleKey names a rule within the catalog it was composed into. The composed catalog follows from
// the service's base catalog and the overlay, and the overlay is already in the entry's key, so the
// base catalog is the one part a second service sharing the cache could change.
type ruleKey struct {
	base *check.Catalog
	name string
}

// ruleRun is one rule's findings and verdicts, each in the order check.RunAll returns them.
type ruleRun struct {
	findings []check.Finding
	verdicts []check.Verdict
}

// runKept is check.RunAll over rules, answering each rule this model has already run from what that
// run returned and evaluating only the rest. Its answer is the one check.RunAll gives over the same
// rules: findings and verdicts sorted by rule, and within a rule in the order RunAll left them, which
// is the same whichever other rules ran beside it.
//
// A run stopped by its context stores nothing. Under a work budget, a rule that came back
// inconclusive is not stored, since the budget may be what stopped it and a request with a larger one
// is owed the real answer; its results still answer this request.
//
// A run under a subject scope (agni issue 897) answers a kept rule by narrowing its whole run to the
// scope, and evaluates the rest under the scope without storing them, since a scoped answer is not
// the rule's answer for the design.
func (b *builtModel) runKept(ctx context.Context, base *check.Catalog, rules []*check.Rule) ([]check.Finding, []check.Verdict, error) {
	if scope := check.SubjectScope(ctx); scope != nil {
		return b.runScoped(ctx, scope, base, rules)
	}
	if !b.kept {
		timing.Cached(ctx, "results", "uncached")
		return check.RunAll(ctx, b.model, rules)
	}
	have := make(map[string]ruleRun, len(rules))
	var missing []*check.Rule
	b.results.mu.Lock()
	for _, r := range rules {
		if run, ok := b.results.rules[ruleKey{base, r.Name}]; ok {
			have[r.Name] = run
		} else {
			missing = append(missing, r)
		}
	}
	b.results.mu.Unlock()

	if len(missing) == 0 {
		timing.Cached(ctx, "results", "memory")
	} else {
		timing.Cached(ctx, "results", "built")
		fs, vs, err := check.RunAll(ctx, b.model, missing)
		if err != nil {
			return nil, nil, err
		}
		ran := make(map[string]ruleRun, len(missing))
		for _, f := range fs {
			run := ran[f.Rule]
			run.findings = append(run.findings, f)
			ran[f.Rule] = run
		}
		for _, v := range vs {
			run := ran[v.Rule]
			run.verdicts = append(run.verdicts, v)
			ran[v.Rule] = run
		}
		budgeted := query.BudgetOf(ctx) > 0
		b.results.mu.Lock()
		if b.results.rules == nil {
			b.results.rules = map[ruleKey]ruleRun{}
		}
		for _, r := range missing {
			run := ran[r.Name]
			have[r.Name] = run
			if budgeted && run.inconclusive() {
				continue
			}
			b.results.rules[ruleKey{base, r.Name}] = run
		}
		b.results.mu.Unlock()
	}

	return joinRuns(have)
}

// runScoped is runKept under a scope: kept rules narrowed to it, the others evaluated under it, and
// the two joined in check.RunAll's order.
func (b *builtModel) runScoped(ctx context.Context, scope *check.Scope, base *check.Catalog, rules []*check.Rule) ([]check.Finding, []check.Verdict, error) {
	have := map[string]ruleRun{}
	var missing []*check.Rule
	if b.kept {
		b.results.mu.Lock()
		for _, r := range rules {
			if run, ok := b.results.rules[ruleKey{base, r.Name}]; ok {
				fs, vs := scope.Results(run.findings, run.verdicts)
				have[r.Name] = ruleRun{findings: fs, verdicts: vs}
			} else {
				missing = append(missing, r)
			}
		}
		b.results.mu.Unlock()
	} else {
		missing = rules
	}
	switch {
	case !b.kept:
		timing.Cached(ctx, "results", "uncached")
	case len(missing) == 0:
		timing.Cached(ctx, "results", "memory")
	default:
		timing.Cached(ctx, "results", "scoped")
	}
	if len(missing) > 0 {
		fs, vs, err := check.RunAll(ctx, b.model, missing)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range missing {
			have[r.Name] = ruleRun{}
		}
		for _, f := range fs {
			run := have[f.Rule]
			run.findings = append(run.findings, f)
			have[f.Rule] = run
		}
		for _, v := range vs {
			run := have[v.Rule]
			run.verdicts = append(run.verdicts, v)
			have[v.Rule] = run
		}
	}
	return joinRuns(have)
}

// joinRuns lays per-rule results out in check.RunAll's order, by rule name and within a rule as that
// rule's run left them.
func joinRuns(have map[string]ruleRun) ([]check.Finding, []check.Verdict, error) {
	names := make([]string, 0, len(have))
	for n := range have {
		names = append(names, n)
	}
	sort.Strings(names)
	var fs []check.Finding
	var vs []check.Verdict
	for _, n := range names {
		fs = append(fs, have[n].findings...)
		vs = append(vs, have[n].verdicts...)
	}
	return fs, vs, nil
}

func (r ruleRun) inconclusive() bool {
	for _, f := range r.findings {
		if f.Inconclusive {
			return true
		}
	}
	for _, v := range r.verdicts {
		if v.Outcome == check.Inconclusive {
			return true
		}
	}
	return false
}
