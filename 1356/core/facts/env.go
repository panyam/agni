package facts

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/jaala/ns"
)

// Env holds what a generator registered in the fact layer reads. That is the design's Model, plus a
// memo, so the generator builds an index once per fact base rather than on every call. A generator
// runs once per binding of the atom that calls it, so an index rebuilt per call turns a walk
// quadratic.
//
// Model is nil for a fact base with no design, such as the whole-corpus datasheet base; a generator
// then yields nothing.
type Env struct {
	Model check.Model

	mu    sync.Mutex
	memo  map[string]any
	spent atomic.Int64
}

// NewEnv returns the environment for one fact base over m.
func NewEnv(m check.Model) *Env { return &Env{Model: m, memo: map[string]any{}} }

// Memo returns the value stored under key, building it the first time. Safe for concurrent use.
func (e *Env) Memo(key string, build func() any) any {
	e.mu.Lock()
	defer e.mu.Unlock()
	if v, ok := e.memo[key]; ok {
		return v
	}
	v := build()
	e.memo[key] = v
	return v
}

// Spend charges n units of a generator's own work beyond the rows it emits, such as the connections
// a walk examined. The charge goes two places. First, it goes to the evaluation's budget, through
// the ns.Allowance in ctx (panyam/jaala#361). So a budgeted query stops partway through an all-pairs
// walk instead of after it. Second, it goes to this fact base's tally. The engine reports a host's
// spending apart from its own work, so query.Work adds the tally to the engine's count. A non-nil
// error means the budget is spent, and the generator returns it unchanged.
func (e *Env) Spend(ctx context.Context, n int64) error {
	e.spent.Add(n)
	if a, ok := ns.AllowanceFrom(ctx); ok {
		return a.Spend(n)
	}
	return nil
}

// Spent is the work every generator over this fact base has charged through Spend.
func (e *Env) Spent() int64 { return e.spent.Load() }

// EnvSource is a Source that holds its fact base's Env. A generator's query runs over a Source, and
// ns.Builtin.Gen hands the generator that Source. The generator reaches the design through EnvSource.
type EnvSource interface {
	FactsEnv() *Env
}

// EnvOf is the Env behind src, or nil when src carries none.
func EnvOf(src ns.Source) *Env {
	if es, ok := src.(EnvSource); ok {
		return es.FactsEnv()
	}
	return nil
}
