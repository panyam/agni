package facts

import (
	"sync"

	"github.com/panyam/agni/core/check"
	"github.com/panyam/jaala/ns"
)

// Env is what a generator registered in the fact layer reads: the design's Model, and a memo for an
// index the generator builds once per fact base rather than on every call. A generator runs once per
// binding of the atom that calls it, so an index rebuilt per call turns a walk quadratic.
//
// Model is nil for a fact base with no design, such as the whole-corpus datasheet base; a generator
// then yields nothing.
type Env struct {
	Model check.Model

	mu   sync.Mutex
	memo map[string]any
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

// EnvSource is a Source that carries its fact base's Env. A generator is handed the Source its query
// runs over (ns.Builtin.Gen), and reaches the design through this.
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
