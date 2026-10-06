// Package timing records where one request's time went (agni issue 914): its stages, which cache
// layer answered each read, how long each rule took, and the plan of each query evaluated.
//
// A Recorder rides the request's context. Every recording call here is a no-op when the context
// carries none, so a request nobody is timing pays one context lookup per call and nothing else.
// The package imports only the standard library, so the rule loop in core/check can record into it.
package timing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Recorder collects one request's timings. Its methods are safe for concurrent use, since a request
// may read its tiers in parallel.
type Recorder struct {
	// Explain asks query evaluations to record their plans (jaala's Explain), which costs time in
	// proportion to the rules and literals a query runs.
	Explain bool

	mu      sync.Mutex
	start   time.Time
	stages  []Stage
	lookups []Lookup
	rules   []RuleTime
	plans   []Plan
}

// Stage is one named span of a request. Stages nest, so a parent's duration includes its children's.
type Stage struct {
	Name string
	// Start is the span's offset from the start of the request.
	Start time.Duration
	Dur   time.Duration
}

// Lookup is which tier answered one cached read: "memory" (kept in this process), "store" (the
// persistent tier, such as the browser's), "built" (read and built now) or "uncached" (no cache).
type Lookup struct {
	Layer  string
	Source string
}

// RuleTime is how long one rule's evaluation took.
type RuleTime struct {
	Name string
	Dur  time.Duration
}

// Plan is one query evaluation's plan, as jaala's Explain renders it.
type Plan struct {
	Query string
	Text  string
}

// New returns a Recorder whose clock starts now.
func New() *Recorder { return &Recorder{start: time.Now()} }

type key struct{}

// With returns a context that records into r.
func With(ctx context.Context, r *Recorder) context.Context { return context.WithValue(ctx, key{}, r) }

// From returns the context's Recorder, nil when the request is not being timed.
func From(ctx context.Context) *Recorder {
	r, _ := ctx.Value(key{}).(*Recorder)
	return r
}

// Begin starts a stage and returns the function that ends it, for `defer timing.Begin(ctx, "x")()`.
func Begin(ctx context.Context, name string) func() {
	r := From(ctx)
	if r == nil {
		return func() {}
	}
	t := time.Now()
	return func() {
		r.mu.Lock()
		r.stages = append(r.stages, Stage{Name: name, Start: t.Sub(r.start), Dur: time.Since(t)})
		r.mu.Unlock()
	}
}

// Cached records which tier answered a read of layer.
func Cached(ctx context.Context, layer, source string) {
	if r := From(ctx); r != nil {
		r.mu.Lock()
		r.lookups = append(r.lookups, Lookup{Layer: layer, Source: source})
		r.mu.Unlock()
	}
}

// Ruled records that rule took d to evaluate.
func Ruled(ctx context.Context, rule string, d time.Duration) {
	if r := From(ctx); r != nil {
		r.mu.Lock()
		r.rules = append(r.rules, RuleTime{Name: rule, Dur: d})
		r.mu.Unlock()
	}
}

// Explaining reports whether the request asked for query plans.
func Explaining(ctx context.Context) bool {
	r := From(ctx)
	return r != nil && r.Explain
}

// Planned records the plan one query evaluation ran.
func Planned(ctx context.Context, query, plan string) {
	if r := From(ctx); r != nil {
		r.mu.Lock()
		r.plans = append(r.plans, Plan{Query: query, Text: plan})
		r.mu.Unlock()
	}
}

// Snapshot is a Recorder's contents at one moment, in the order a reader wants them.
type Snapshot struct {
	Total   time.Duration
	Stages  []Stage    // by start
	Lookups []Lookup   // in the order the reads happened
	Rules   []RuleTime // slowest first
	Plans   []Plan     // in the order the queries ran
}

// Snapshot returns what r has recorded, with the request's total taken now.
func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := Snapshot{
		Total:   time.Since(r.start),
		Stages:  append([]Stage(nil), r.stages...),
		Lookups: append([]Lookup(nil), r.lookups...),
		Rules:   append([]RuleTime(nil), r.rules...),
		Plans:   append([]Plan(nil), r.plans...),
	}
	sort.SliceStable(s.Stages, func(i, j int) bool { return s.Stages[i].Start < s.Stages[j].Start })
	sort.SliceStable(s.Rules, func(i, j int) bool { return s.Rules[i].Dur > s.Rules[j].Dur })
	return s
}

// Slowest returns the stage taking the most time, so a log line can name what dominated. A stage
// that only wraps others still counts, so the answer is the widest span, and a reader goes from it to
// the stages inside it.
func (s Snapshot) Slowest() (Stage, bool) {
	var best Stage
	for _, st := range s.Stages {
		if st.Dur > best.Dur {
			best = st
		}
	}
	return best, best.Name != ""
}

// Text renders the snapshot for a terminal: the total, each stage, each cache lookup, the slowest
// rules (at most topRules, all of them when topRules is 0), and each plan.
func (s Snapshot) Text(topRules int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "total %s\n", round(s.Total))
	for _, st := range s.Stages {
		fmt.Fprintf(&b, "  %-24s %10s  at %s\n", st.Name, round(st.Dur), round(st.Start))
	}
	if len(s.Lookups) > 0 {
		parts := make([]string, len(s.Lookups))
		for i, l := range s.Lookups {
			parts[i] = l.Layer + "=" + l.Source
		}
		fmt.Fprintf(&b, "  cache: %s\n", strings.Join(parts, " "))
	}
	rules := s.Rules
	if topRules > 0 && len(rules) > topRules {
		rules = rules[:topRules]
	}
	if len(rules) > 0 {
		fmt.Fprintf(&b, "  slowest rules (%d of %d):\n", len(rules), len(s.Rules))
		for _, r := range rules {
			fmt.Fprintf(&b, "    %-40s %10s\n", r.Name, round(r.Dur))
		}
	}
	for _, p := range s.Plans {
		fmt.Fprintf(&b, "  plan for %s\n", p.Query)
		for _, line := range strings.Split(strings.TrimRight(p.Text, "\n"), "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
	}
	return b.String()
}

// round drops a duration's digits past what a reader compares by.
func round(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(10 * time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(10 * time.Microsecond)
	default:
		return d.Round(time.Microsecond)
	}
}
